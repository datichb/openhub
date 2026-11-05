package tracker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// ─────────────────────────────────────────────────────────────────────────────
// Result types
// ─────────────────────────────────────────────────────────────────────────────

// SyncResult is the outcome of a full tracker sync run.
type SyncResult struct {
	SyncedAt time.Time
	Projects []ProjectSyncResult
	// Aggregated counters.
	ClaimsCreated  int
	ClaimsUpdated  int
	ClaimsReleased int // done claims cleaned up by retention policy
	ClaimsExpired  int // expired claims released (team setting release_expired_on_sync)
	LabelsPushed   int
	Warnings       []SyncWarning
	Errors         []SyncError
}

// ProjectSyncResult holds the outcome for a single hub project.
type ProjectSyncResult struct {
	ProjectID      string
	TrackerProject string
	IssuesFetched  int
	ClaimsCreated  int
	ClaimsUpdated  int
	LabelsPushed   int
	Duration       time.Duration
	Warnings       []string
}

// SyncWarning is a non-fatal condition worth surfacing (e.g. reassignment).
type SyncWarning struct {
	ProjectID string
	TicketID  string
	Message   string
}

// String is the warning with its place (project/ticket) when it has one.
func (w SyncWarning) String() string {
	var place []string
	for _, p := range []string{w.ProjectID, w.TicketID} {
		if p != "" {
			place = append(place, p)
		}
	}
	if len(place) == 0 {
		return w.Message
	}
	return strings.Join(place, "/") + ": " + w.Message
}

// SyncError is an error scoped to one project or ticket that did not abort the run.
type SyncError struct {
	ProjectID string
	TicketID  string
	Err       error
}

func (e SyncError) Error() string {
	if e.TicketID != "" {
		return fmt.Sprintf("%s/%s: %v", e.ProjectID, e.TicketID, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.ProjectID, e.Err)
}

// ─────────────────────────────────────────────────────────────────────────────
// Engine
// ─────────────────────────────────────────────────────────────────────────────

// Store is what the engine needs from the team-state repository: the
// interfaces of the sub-domains it reads and writes, nothing more
// (*teamstate.Repo implements them all).
type Store interface {
	teamstate.RepoSync
	teamstate.BoardStatusSetter
	teamstate.ConfigReader
	teamstate.MemberReader
	teamstate.ClaimReader
	teamstate.ClaimLabeler
	teamstate.ClaimTrackerSync
	teamstate.ClaimCleaner
	teamstate.ClaimExpirer
	teamstate.ClaimRemovalJudge
	teamstate.SyncLockReader
}

// Engine reconciles team-state claims with an external tracker.
type Engine struct {
	tracker  Tracker
	repo     Store
	cfg      teamstate.TrackerConfig
	boardCfg teamstate.BoardConfig
	state    *SyncState
	stateDir string
	// identity is who runs the sync (see SetIdentity); unknown holds, for the
	// run in progress, the tracker names assigned to issues that match no
	// member (lower case → name as the tracker writes it).
	identity Identity
	unknown  map[string]string
	// index says which hub project holds the claim of each issue of the
	// tracker, for the run in progress.
	index *issueIndex
	// released are the claims the run removes on purpose (the done claims past
	// their retention, the expired ones): the commit of the run names them, so
	// that no member takes them for claims lost by accident.
	released []teamstate.Claim
}

// Identity says who runs the sync, to warn when their tracker name is missing.
type Identity struct {
	// TeamID is the team of the sync (for the command given in the warning).
	TeamID string
	// MemberID is the member of hub.toml for this team.
	MemberID string
	// CrossServer: the tracker is not on the server of the team-state
	// repository, so the GitLab name of the member is not their name there.
	CrossServer bool
	// TokenFingerprint identifies the token of the tracker without being it
	// (see TokenFingerprint): the refusals counted by the circuit breaker are
	// those of that token.
	TokenFingerprint string
}

// SetIdentity tells the engine who runs the sync. When issues of the tracker
// are assigned to names that match no member and that member has no
// tracker_username (or none at all, on the same server), the result of the run
// carries a warning that says how to set it.
func (e *Engine) SetIdentity(id Identity) { e.identity = id }

// NewEngine creates a sync engine.
// stateDir is the local directory for sync-state.json (~/.oh/).
func NewEngine(t Tracker, repo Store, cfg teamstate.TrackerConfig, stateDir string) *Engine {
	return &Engine{
		tracker:  t,
		repo:     repo,
		cfg:      cfg,
		stateDir: stateDir,
	}
}

// RunTimeout bounds a run: the wait for the lock of the team-state clone (a
// background writer) and the sync itself. A caller that wants the run to be
// cut earlier gives a context that is.
const RunTimeout = teamstate.BackgroundLockWait + 60*time.Second

// DefaultSyncInterval is sync_interval_minutes when the team sets none.
const DefaultSyncInterval = 5 * time.Minute

var (
	// ErrAutoSyncDisabled: an automatic run was asked while the team has not
	// turned auto_sync on.
	ErrAutoSyncDisabled = errors.New("tracker: automatic sync is not enabled (auto_sync = false)")
	// ErrAutoSyncMuted: the server refused the token three times in a row; the
	// automatic sync waits for another token (a manual sync still tries).
	ErrAutoSyncMuted = errors.New("tracker: automatic sync stopped after repeated refusals of the token")
)

// LeaseHeldError: the automatic sync of another run holds the lease of the
// team (sync.lock) and it has not expired: that run synchronizes this interval.
type LeaseHeldError struct {
	// Member is the member whose sync holds the lease.
	Member string
	// Until is the end of the lease.
	Until time.Time
}

func (e *LeaseHeldError) Error() string {
	return fmt.Sprintf("tracker: the sync of %s holds the lease until %s", e.Member, e.Until.Local().Format("15:04:05"))
}

// SyncInterval is the interval of the automatic sync (sync_interval_minutes, 5
// minutes when unset) and the length of the lease the sync takes.
func (e *Engine) SyncInterval() time.Duration {
	if m := e.cfg.SyncIntervalMinutes; m > 0 {
		return time.Duration(m) * time.Minute
	}
	return DefaultSyncInterval
}

// AutoSyncEnabled reports whether the team asked for the automatic sync
// (auto_sync).
func (e *Engine) AutoSyncEnabled() bool { return e.cfg.AutoSync }

// Run executes a full sync cycle on request (oh team sync-tracker, the r key of
// the board) and returns the aggregated result. It does not wait for the
// lease of the automatic sync and, when the team has auto_sync on, takes it
// (the others leave the next interval to this sync).
//
// Strategy:
//  1. Pull the team-state repo to get the latest claims.
//  2. For each configured project, reconcile claims against the tracker.
//  3. Apply all local file changes (UpdateClaimStatus, AddClaimLabel, CreateClaim).
//  4. One single CommitAndPush for the entire batch.
//
// The single commit approach minimises round-trips and reduces the concurrency
// window for conflicts — if two members sync simultaneously, at most one of them
// will encounter a non-fast-forward push and retry with rebase.
func (e *Engine) Run(ctx context.Context) (*SyncResult, error) { return e.run(ctx, false) }

// RunAuto is the run of the automatic sync (daemon, opening of the board). It
// fails with ErrAutoSyncDisabled when the team did not turn auto_sync on, with
// ErrAutoSyncMuted when the server refused the token three times in a row, and
// with a *LeaseHeldError when the sync of another run (a colleague, another
// machine) holds the lease of this interval: one member synchronizes per
// interval. The lease is a sync.lock of the team-state (member and expiry),
// written by the sync itself in its commit, so that two members who start
// together are told apart by the push: the second one finds the lease of the
// first when it runs its transaction again.
func (e *Engine) RunAuto(ctx context.Context) (*SyncResult, error) { return e.run(ctx, true) }

func (e *Engine) run(ctx context.Context, auto bool) (*SyncResult, error) {
	slog.Debug("tracker.sync.start", "projects", len(e.cfg.Projects), "autoplan", e.cfg.AutoPlanAssigned, "auto", auto) //nolint:staticcheck // backward compat: deprecated field
	start := time.Now()

	if auto {
		if !e.cfg.AutoSync {
			return nil, ErrAutoSyncDisabled
		}
		if !e.ShouldAutoSync() {
			return nil, ErrAutoSyncMuted
		}
		// Off the network: a lease the clone already shows is a lease.
		if lock, err := e.repo.ReadSyncLock(); err == nil && lock.Held(start) {
			return nil, &LeaseHeldError{Member: lock.Member, Until: lock.ExpiresAt.Time()}
		}
	}

	// Global timeout to bound the total sync wall time (plus the wait for
	// the lock of the team-state clone: a background writer).
	ctx, cancel := context.WithTimeout(teamstate.WithLockWait(ctx, teamstate.BackgroundLockWait), RunTimeout)
	defer cancel()

	// Load (or initialise) the local sync state.
	state, err := LoadSyncState(e.stateDir)
	if err != nil {
		return nil, fmt.Errorf("sync engine: loading state: %w", err)
	}
	e.state = state

	// Load board config for dynamic status resolution.
	if teamCfg, cfgErr := e.repo.LoadConfig(ctx); cfgErr == nil && teamCfg != nil {
		e.boardCfg = teamCfg.Board
		// Propagate custom statuses to the repo so UpdateClaimStatus
		// and UpdateClaimStatusFromTracker accept them.
		e.repo.SetBoardStatuses(e.boardCfg.AllStatuses())
	}

	// One transaction for the whole run: pull, every claim change of
	// the reconciliation and the clean-up of done claims, one commit, one
	// push. A push overtaken by another member runs the reconciliation
	// again on top of it.
	var result *SyncResult
	var fatal, abort error
	var held *LeaseHeldError
	txErr := e.repo.Transact(ctx, func(ctx context.Context, tx *teamstate.Tx) (teamstate.TxResult, error) {
		now := time.Now().UTC()
		held = nil
		if auto {
			// The latest state of the team-state: a colleague may have taken
			// the lease since the clone was read.
			if lock, err := tx.ReadSyncLock(); err == nil && lock.Held(now) {
				held = &LeaseHeldError{Member: lock.Member, Until: lock.ExpiresAt.Time()}
				return teamstate.TxResult{}, held
			}
		}
		result = &SyncResult{SyncedAt: now}
		fatal, abort = e.reconcileAll(ctx, result)
		if abort != nil {
			return teamstate.TxResult{}, abort
		}
		e.released = nil
		e.cleanupDone(ctx, result)
		e.releaseExpired(ctx, result)
		if e.cfg.AutoSync && fatal == nil {
			member := firstNonEmptyStr(e.identity.MemberID, "unknown")
			if err := tx.WriteSyncLock(teamstate.NewSyncLock(member, now, e.SyncInterval())); err != nil {
				slog.Warn("tracker.sync.lease_failed", "error", err)
			}
		}
		msg := fmt.Sprintf("sync: %d created, %d updated, %d labels pushed, %d released",
			result.ClaimsCreated, result.ClaimsUpdated, result.LabelsPushed, result.ClaimsReleased)
		if result.ClaimsExpired > 0 {
			msg += fmt.Sprintf(", %d expired", result.ClaimsExpired)
		}
		return teamstate.TxResult{Message: teamstate.WithReleasedTrailers(msg, e.released)}, nil
	})
	if abort != nil {
		return nil, abort
	}
	if held != nil && errors.Is(txErr, held) {
		return nil, held
	}
	if txErr != nil {
		// Nothing reached the team-state: neither a result nor the sync
		// state (the timestamps of a run that pushed nothing) is kept.
		slog.Warn("tracker.sync.push_failed", "error", txErr)
		return nil, fmt.Errorf("sync engine: %w", txErr)
	}

	// The refusals of the token, counted once per run (the transaction may
	// have run its build more than once): a run that got through clears them.
	switch {
	case fatal == nil:
		e.state.ClearTokenFailures(e.breakerKey())
	case errors.Is(fatal, ErrTokenInvalid):
		e.state.NoteTokenFailure(e.breakerKey(), e.identity.TokenFingerprint, time.Now().UTC())
	}

	// Persist the updated sync timestamps.
	if err := e.state.Save(e.stateDir); err != nil {
		slog.Warn("tracker.sync.state_save_failed", "error", err)
	}
	slog.Debug("tracker.sync.complete", "duration", time.Since(start), "created", result.ClaimsCreated, "updated", result.ClaimsUpdated)
	if fatal != nil {
		return result, fatal
	}
	return result, nil
}

// reconcileAll reconciles every project inside the transaction of Run. A
// fatal error (invalid token) stops the loop and is reported after the
// commit of what was done; abort aborts the transaction.
func (e *Engine) reconcileAll(ctx context.Context, result *SyncResult) (fatal, abort error) {
	// Load members once (needed for auto-plan and assignee lookup).
	members, err := e.repo.ListMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync engine: listing members: %w", err)
	}
	memberByGitLab := make(map[string]teamstate.Member, len(members))
	for _, m := range members {
		// TrackerUsername takes priority over GitLabUsername for tracker sync.
		// This allows teams to use a different GitLab/Jira instance for the tracker
		// than the one used for the team-state repo.
		username := m.TrackerUsername
		if username == "" {
			username = m.GitLabUsername
		}
		if username != "" {
			memberByGitLab[strings.ToLower(username)] = m
		}
	}
	slog.Debug("tracker.sync.members", "count", len(members))
	for _, issue := range e.cfg.MappingIssues(e.boardCfg) {
		result.Warnings = append(result.Warnings, SyncWarning{Message: issue.Text()})
	}
	e.unknown = map[string]string{}
	defer func() {
		if w := e.unknownAssigneesWarning(members); w != nil {
			result.Warnings = append(result.Warnings, *w)
		}
	}()

	trackerType := Type(e.cfg.Type)

	projects := e.cfg.Projects //nolint:staticcheck // backward compat: deprecated field
	hubProjects := sortedKeys(projects)
	if err := e.indexClaims(ctx, hubProjects); err != nil {
		return nil, err
	}

	for _, hubProjectID := range hubProjects {
		trackerProjectID := projects[hubProjectID]
		slog.Debug("tracker.sync.project", "hub_project", hubProjectID, "tracker_project", trackerProjectID)
		start := time.Now()
		pr, projectErr := e.reconcileProject(ctx, hubProjectID, trackerProjectID, trackerType, memberByGitLab)
		pr.Duration = time.Since(start)
		result.Projects = append(result.Projects, pr)
		for _, w := range pr.Warnings {
			result.Warnings = append(result.Warnings, SyncWarning{ProjectID: hubProjectID, Message: w})
		}
		result.ClaimsCreated += pr.ClaimsCreated
		result.ClaimsUpdated += pr.ClaimsUpdated
		result.LabelsPushed += pr.LabelsPushed

		if projectErr != nil {
			// Fatal errors (token invalid, rate limit) abort the entire run.
			if errors.Is(projectErr, ErrTokenInvalid) {
				return projectErr, nil
			}
			if IsRateLimited(projectErr) {
				result.Errors = append(result.Errors, SyncError{ProjectID: hubProjectID, Err: projectErr})
				break // stop processing further projects
			}
			// Non-fatal project error — skip and continue.
			result.Errors = append(result.Errors, SyncError{ProjectID: hubProjectID, Err: projectErr})
		}

		e.state.SetLastSync(trackerType, trackerProjectID, result.SyncedAt)
	}

	return nil, nil
}

// patternOf is the ticket pattern of a hub project (empty: none).
func (e *Engine) patternOf(hubProjectID string) string {
	return e.cfg.TicketPatterns[hubProjectID] //nolint:staticcheck // backward compat: deprecated field
}

// indexClaims records, for the run, the issue of every existing claim of the
// hub projects (in the order given: the first project to hold an issue keeps
// it), so that a claim is created once per issue of the tracker.
func (e *Engine) indexClaims(ctx context.Context, hubProjects []string) error {
	e.index = newIssueIndex()
	for _, hub := range hubProjects {
		claims, err := e.repo.ListClaims(ctx, hub)
		if err != nil {
			return fmt.Errorf("listing claims of %s: %w", hub, err)
		}
		trackerProject := e.cfg.Projects[hub] //nolint:staticcheck // backward compat: deprecated field
		for i := range claims {
			if iid, ok := claimIID(claims[i].ExternalIID, claims[i].TicketID, e.patternOf(hub)); ok {
				e.index.note(trackerProject, iid, hub)
			}
		}
	}
	return nil
}

// issueClaimed reports whether the issue has a claim already: under the ID
// the sync would give it, or under any other (the same number, in this hub
// project or another one linked to the same tracker project).
func (e *Engine) issueClaimed(claimedTickets map[string]bool, trackerProjectID string, issue *IssueState, ticketID string) bool {
	return claimedTickets[ticketID] || (issue.IID > 0 && e.index.taken(trackerProjectID, issue.IID))
}

// removedOnPurpose reports whether the claim of the issue was released on
// purpose, under the ID the sync gives it or under its bare number (the claims
// made before the ID followed the pattern).
func (e *Engine) removedOnPurpose(ctx context.Context, hubProjectID, ticketID string, iid int) bool {
	if e.repo.ClaimRemovedOnPurpose(ctx, hubProjectID, ticketID) {
		return true
	}
	bare := strconv.Itoa(iid)
	return iid > 0 && bare != ticketID && e.repo.ClaimRemovedOnPurpose(ctx, hubProjectID, bare)
}

// noteClaimed records the claim the run creates for the issue.
func (e *Engine) noteClaimed(hubProjectID, trackerProjectID string, claimedTickets map[string]bool, ticketID string, iid int) {
	claimedTickets[ticketID] = true
	if iid > 0 {
		e.index.note(trackerProjectID, iid, hubProjectID)
	}
}

// noteUnknownAssignees remembers the assignees of an issue that match no
// member.
func (e *Engine) noteUnknownAssignees(assignees []string, memberByName map[string]teamstate.Member) {
	if e.unknown == nil {
		return
	}
	for _, name := range assignees {
		key := strings.ToLower(strings.TrimSpace(name))
		if _, ok := memberByName[key]; key != "" && !ok {
			if _, seen := e.unknown[key]; !seen {
				e.unknown[key] = strings.TrimSpace(name)
			}
		}
	}
}

// maxUnknownNamesShown bounds the names listed in the warning.
const maxUnknownNamesShown = 5

// unknownAssigneesWarning is the warning of the run when issues are assigned to
// names that belong to no member while the member who runs the sync has no
// tracker_username: their issues may be among them. nil when there is nothing
// to say or the engine does not know who runs it.
func (e *Engine) unknownAssigneesWarning(members []teamstate.Member) *SyncWarning {
	if len(e.unknown) == 0 || e.identity.MemberID == "" {
		return nil
	}
	var me *teamstate.Member
	for i := range members {
		if members[i].ID == e.identity.MemberID {
			me = &members[i]
		}
	}
	if me == nil || me.TrackerUsername != "" || (!e.identity.CrossServer && me.GitLabUsername != "") {
		return nil
	}
	names := make([]string, 0, len(e.unknown))
	for _, n := range e.unknown {
		names = append(names, n)
	}
	sort.Strings(names)
	shown := strings.Join(names[:min(len(names), maxUnknownNamesShown)], ", ")
	if len(names) > maxUnknownNamesShown {
		shown += ", …"
	}
	return &SyncWarning{Message: i18n.Tf("tracker.sync.unknown_assignees",
		len(names), shown, me.ID, firstNonEmptyStr(e.identity.TeamID, "<team>"))}
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// cleanupDone releases the done claims past their retention (board terminal
// statuses), inside the transaction of Run.
func (e *Engine) cleanupDone(ctx context.Context, result *SyncResult) {
	teamCfg, cfgErr := e.repo.LoadConfig(ctx)
	if cfgErr != nil || teamCfg == nil {
		return
	}
	retDays := teamCfg.Claim.DoneRetentionDays
	if retDays <= 0 {
		retDays = 7 // default
	}
	released, cleanErr := e.repo.CleanupDoneClaims(ctx, retDays, e.boardCfg.TerminalStatuses()...)
	if cleanErr != nil {
		slog.Warn("tracker.sync.cleanup_failed", "error", cleanErr)
	} else if len(released) > 0 {
		slog.Info("tracker.sync.cleanup", "released", len(released), "retention_days", retDays)
		result.ClaimsReleased = len(released)
		e.forgetKnown(released)
	}
}

// releaseExpired releases the claims of the synced projects whose lease is
// past (or that are inactive, for a claim without lease), inside the
// transaction of Run. Only when the team asks for it ([claim]
// release_expired_on_sync, off by default): an expired claim is otherwise
// reported by oh doctor and may be taken over.
func (e *Engine) releaseExpired(ctx context.Context, result *SyncResult) {
	teamCfg, err := e.repo.LoadConfig(ctx)
	if err != nil || teamCfg == nil || !teamCfg.Claim.ReleaseExpiredOnSync {
		return
	}
	projects := make([]string, 0, len(e.cfg.Projects)) //nolint:staticcheck // backward compat: deprecated field
	for p := range e.cfg.Projects {                    //nolint:staticcheck // backward compat: deprecated field
		projects = append(projects, p)
	}
	released, err := e.repo.ReleaseExpiredClaims(ctx, result.SyncedAt, projects...)
	if err != nil {
		slog.Warn("tracker.sync.expire_failed", "error", err)
		return
	}
	if len(released) == 0 {
		return
	}
	slog.Info("tracker.sync.expired", "released", len(released))
	result.ClaimsExpired = len(released)
	for _, c := range released {
		result.Warnings = append(result.Warnings, SyncWarning{ProjectID: c.Project, TicketID: c.TicketID,
			Message: i18n.Tf("tracker.sync.expired_released", c.ClaimedBy)})
	}
	e.forgetKnown(released)
}

// forgetKnown takes the claims the run released itself out of the known
// tickets: they are gone on purpose, not lost.
func (e *Engine) forgetKnown(released []teamstate.Claim) {
	e.released = append(e.released, released...)
	for _, c := range released {
		e.state.ForgetKnownTicket(c.Project, c.TicketID)
	}
}

// breakerKey identifies the token failures of this engine in the sync state:
// the team and the tracker it syncs.
func (e *Engine) breakerKey() string {
	return firstNonEmptyStr(e.identity.TeamID, "-") + "/" + e.cfg.Type
}

// ShouldAutoSync reports whether an automatic sync may try: false once the
// server refused the same token three times in a row (the count is kept in the
// sync state, so that it holds from one run, and one process, to the next). A
// manual sync always tries; another token, or a run that gets through, lifts
// the stop.
func (e *Engine) ShouldAutoSync() bool {
	state := e.state
	if state == nil {
		loaded, err := LoadSyncState(e.stateDir)
		if err != nil {
			return true
		}
		state = loaded
	}
	return !state.TokenMuted(e.breakerKey(), e.identity.TokenFingerprint)
}

// FetchTicketDetail fetches the full title and description of a ticket from the
// external tracker. Used for on-demand display in the TUI detail view.
// Returns the title and the full (non-truncated) description.
func (e *Engine) FetchTicketDetail(ctx context.Context, hubProjectID, ticketID string) (title, description string, err error) {
	trackerProjectID, ok := e.cfg.Projects[hubProjectID] //nolint:staticcheck // backward compat: deprecated field
	if !ok {
		// Try single-project config fallback
		if len(e.cfg.Projects) == 1 { //nolint:staticcheck // backward compat: deprecated field
			for _, v := range e.cfg.Projects { //nolint:staticcheck // backward compat: deprecated field
				trackerProjectID = v
			}
		} else {
			return "", "", fmt.Errorf("no tracker project configured for %q", hubProjectID)
		}
	}

	pattern := e.cfg.TicketPatterns[hubProjectID] //nolint:staticcheck // backward compat: deprecated field

	// Try to get the claim to read the stored ExternalIID.
	var storedIID int
	if c, err := e.repo.GetClaim(ctx, hubProjectID, ticketID); err == nil {
		storedIID = c.ExternalIID
	}

	iid, ok := resolveIID(ticketID, storedIID, pattern)
	if !ok {
		return "", "", fmt.Errorf("cannot resolve tracker IID for %q", ticketID)
	}

	issue, err := e.tracker.FetchIssue(ctx, trackerProjectID, iid)
	if err != nil {
		return "", "", err
	}

	return issue.Title, issue.Description, nil
}

// AssignIssueToMember assigns a tracker issue to a team member.
// Resolves the member's tracker username from the team-state member registry.
// Only operates when PushLabels is enabled (same gate as label push).
// This is a best-effort operation — errors are logged but not fatal.
func (e *Engine) AssignIssueToMember(ctx context.Context, hubProjectID string, externalIID int, memberID string) error {
	if !e.cfg.PushLabels {
		return nil // push disabled
	}
	if externalIID == 0 {
		return fmt.Errorf("no external IID for member assignment")
	}

	trackerProjectID, ok := e.cfg.Projects[hubProjectID] //nolint:staticcheck // backward compat: deprecated field
	if !ok {
		if len(e.cfg.Projects) == 1 { //nolint:staticcheck // backward compat: deprecated field
			for _, v := range e.cfg.Projects { //nolint:staticcheck // backward compat: deprecated field
				trackerProjectID = v
			}
		} else {
			return fmt.Errorf("no tracker project configured for %q", hubProjectID)
		}
	}

	// Resolve member → tracker username (TrackerUsername overrides GitLabUsername)
	member, err := e.repo.GetMember(ctx, memberID)
	if err != nil {
		return fmt.Errorf("resolving member %q: %w", memberID, err)
	}
	trackerUsername := member.TrackerUsername
	if trackerUsername == "" {
		trackerUsername = member.GitLabUsername
	}
	if trackerUsername == "" {
		return fmt.Errorf("member %q has no tracker username configured", memberID)
	}

	return e.tracker.AssignIssue(ctx, trackerProjectID, externalIID, trackerUsername)
}

// ─────────────────────────────────────────────────────────────────────────────
// Per-project reconciliation
// ─────────────────────────────────────────────────────────────────────────────

func (e *Engine) reconcileProject(
	ctx context.Context,
	hubProjectID, trackerProjectID string,
	trackerType Type,
	memberByGitLab map[string]teamstate.Member,
) (ProjectSyncResult, error) {
	pr := ProjectSyncResult{
		ProjectID:      hubProjectID,
		TrackerProject: trackerProjectID,
	}

	// Get the regex pattern for this project (if configured).
	pattern := e.cfg.TicketPatterns[hubProjectID] //nolint:staticcheck // backward compat: deprecated field

	// Load all claims for this project.
	claims, err := e.repo.ListClaims(ctx, hubProjectID)
	if err != nil {
		return pr, fmt.Errorf("listing claims: %w", err)
	}
	slog.Debug("tracker.reconcile.start", "project", hubProjectID, "claims", len(claims))

	// Build a set of already-claimed ticket IDs for idempotence.
	claimedTickets := make(map[string]bool, len(claims))
	for _, c := range claims {
		claimedTickets[c.TicketID] = true
	}

	// ── Pull direction: tracker → claims ─────────────────────────────────────

	for i := range claims {
		c := &claims[i]
		iid, ok := resolveIID(c.TicketID, c.ExternalIID, pattern)
		if !ok {
			slog.Debug("tracker.reconcile.skip_no_iid", "ticket", c.TicketID)
			continue // no tracker link for this ticket
		}

		issue, err := e.tracker.FetchIssue(ctx, trackerProjectID, iid)
		if err != nil {
			if errors.Is(err, ErrIssueNotFound) {
				slog.Debug("tracker.reconcile.issue_gone", "ticket", c.TicketID, "iid", iid)
				continue // ticket deleted on tracker — ignore
			}
			return pr, err // propagate fatal errors
		}
		pr.IssuesFetched++

		// Store the resolved IID if not already set (avoids re-parsing next time).
		if c.ExternalIID == 0 {
			_ = e.repo.SetClaimExternalIID(ctx, hubProjectID, c.TicketID, iid)
		}

		// Metadata sync: title, description, tracker status.
		truncDesc := TruncateDescription(issue.Description)
		if err := e.repo.UpdateClaimMetadata(ctx, hubProjectID, c.TicketID, issue.Title, truncDesc, issue.StatusName); err != nil {
			slog.Warn("tracker.reconcile.metadata_failed", "ticket", c.TicketID, "error", err)
			pr.Warnings = append(pr.Warnings, fmt.Sprintf("ticket %s: metadata update failed: %v", c.TicketID, err))
		}

		// Status sync: the tracker moves the status of the claim only when the
		// issue changed since the previous pass (see statusFromTracker).
		mappedStatus := MapTrackerStatusWithBoard(issue, e.cfg.StatusMapping, e.cfg.LabelStatusMapping, e.boardCfg)
		hubStatus := c.Status
		verdict := e.statusFromTracker(c, issue)
		if verdict == issueChanged && mappedStatus != c.Status {
			slog.Debug("tracker.reconcile.transition", "ticket", c.TicketID, "from", c.Status, "to", mappedStatus, "tracker_status", issue.StatusName)
			if err := e.repo.UpdateClaimStatusFromTracker(ctx, hubProjectID, c.TicketID, mappedStatus); err == nil {
				pr.ClaimsUpdated++
				hubStatus = mappedStatus
			} else {
				slog.Warn("tracker.reconcile.status_failed", "ticket", c.TicketID, "error", err)
				pr.Warnings = append(pr.Warnings, fmt.Sprintf("ticket %s: status update failed: %v", c.TicketID, err))
			}
		}

		// Label sync: mirror tracker labels onto the claim. A label leaves the
		// claim only when it was on the issue at the previous pass and is no
		// longer: the others (session-ended, review, ready-for-review...) were
		// put there by the hub.
		trackerLabelSet := labelSet(issue.Labels)
		for _, l := range issue.Labels {
			if !hasLabel(c.Labels, l) {
				if err := e.repo.AddClaimLabel(ctx, hubProjectID, c.TicketID, l); err != nil {
					slog.Warn("tracker.reconcile.label_failed", "ticket", c.TicketID, "label", l, "error", err)
					pr.Warnings = append(pr.Warnings, fmt.Sprintf("ticket %s: add label %q failed: %v", c.TicketID, l, err))
				}
			}
		}
		for _, l := range c.TrackerLabels {
			if isTrackerMirroredLabel(l) && !trackerLabelSet[l] && hasLabel(c.Labels, l) {
				if err := e.repo.RemoveClaimLabel(ctx, hubProjectID, c.TicketID, l); err != nil {
					slog.Warn("tracker.reconcile.label_failed", "ticket", c.TicketID, "label", l, "error", err)
					pr.Warnings = append(pr.Warnings, fmt.Sprintf("ticket %s: remove label %q failed: %v", c.TicketID, l, err))
				}
			}
		}

		// Assignee change warning.
		if len(issue.Assignees) > 0 {
			assignee := strings.ToLower(issue.Assignees[0])
			if m, ok := memberByGitLab[assignee]; ok && m.ID != c.ClaimedBy {
				pr.Warnings = append(pr.Warnings, i18n.Tf("v54y.tracker.sync.assignee_changed", c.TicketID, issue.Assignees[0], m.ID, c.ClaimedBy))
			}
		}
		e.noteUnknownAssignees(issue.Assignees, memberByGitLab)

		// ── Push direction: claims → tracker ─────────────────────────────────
		seenLabels := issue.Labels
		if e.cfg.PushLabels {
			labelsToAdd := hubLabelsToSync(c.Labels, issue.Labels)
			if len(labelsToAdd) > 0 {
				if err := e.tracker.AddLabels(ctx, trackerProjectID, iid, labelsToAdd); err == nil {
					pr.LabelsPushed += len(labelsToAdd)
					seenLabels = append(slices.Clone(seenLabels), labelsToAdd...)
				}
			}
			if verdict == issueUnchanged {
				var added, removed []string
				seenLabels, added, removed = e.pushStatusLabel(ctx, trackerProjectID, iid, issue, hubStatus, mappedStatus, seenLabels, &pr)
				// The claim carries what the issue now carries: the pass
				// recorded the labels after the push, so the next one would not
				// see the labels taken off.
				for _, l := range added {
					if err := e.repo.AddClaimLabel(ctx, hubProjectID, c.TicketID, l); err != nil {
						slog.Warn("tracker.reconcile.label_failed", "ticket", c.TicketID, "label", l, "error", err)
					}
				}
				for _, l := range removed {
					if err := e.repo.RemoveClaimLabel(ctx, hubProjectID, c.TicketID, l); err != nil {
						slog.Warn("tracker.reconcile.label_failed", "ticket", c.TicketID, "label", l, "error", err)
					}
				}
			}
		}

		// What this pass saw of the issue, for the next one.
		if err := e.repo.SetClaimTrackerMirror(ctx, hubProjectID, c.TicketID, issueState(issue), seenLabels); err != nil {
			slog.Warn("tracker.reconcile.mirror_failed", "ticket", c.TicketID, "error", err)
		}
	}

	// ── Auto-plan: assigned issues without a claim ────────────────────────────
	if e.cfg.AutoPlanAssigned {
		firstSync := len(claims) == 0
		if err := e.autoplan(ctx, hubProjectID, trackerProjectID, trackerType, memberByGitLab, claimedTickets, firstSync, &pr); err != nil {
			// Non-fatal — log but continue.
			slog.Warn("tracker.autoplan.failed", "project", hubProjectID, "error", err)
		}
	}

	// ── Pool: unassigned tracker issues as claimable tickets ─────────────────
	if e.cfg.AutoPlanUnassigned {
		firstSync := len(claims) == 0
		if err := e.autopoolUnassigned(ctx, hubProjectID, trackerProjectID, trackerType, memberByGitLab, claimedTickets, firstSync, &pr); err != nil {
			// Non-fatal — log but continue.
			slog.Warn("tracker.autopool.failed", "project", hubProjectID, "error", err)
		}
	}

	// ── Orphan recovery: detect and restore deleted/corrupted claims ─────────
	if err := e.recoverOrphans(ctx, hubProjectID, trackerProjectID, claimedTickets, &pr); err != nil {
		slog.Warn("tracker.orphan.recovery_failed", "project", hubProjectID, "error", err)
	}

	// ── Snapshot known tickets for future orphan detection ───────────────────
	updatedClaims, _ := e.repo.ListClaims(ctx, hubProjectID)
	known := make([]KnownTicket, 0, len(updatedClaims))
	for _, c := range updatedClaims {
		if c.ExternalIID > 0 {
			known = append(known, KnownTicket{
				TicketID:    c.TicketID,
				ExternalIID: c.ExternalIID,
				ClaimedBy:   c.ClaimedBy,
			})
		}
	}
	e.state.SetKnownTickets(hubProjectID, known)

	return pr, nil
}

// recoverOrphans detects claims that existed in a previous sync but are now
// missing locally (deleted, corrupted, or lost to git conflicts; not the
// ones released on purpose). For each
// orphan, it fetches the issue from the tracker and recreates the claim.
// Only open issues are recovered — closed issues are considered intentionally gone.
func (e *Engine) recoverOrphans(
	ctx context.Context,
	hubProjectID, trackerProjectID string,
	claimedTickets map[string]bool,
	pr *ProjectSyncResult,
) error {
	known := e.state.GetKnownTickets(hubProjectID)
	if len(known) == 0 {
		return nil // first sync or no known tickets yet — nothing to compare
	}

	// Find orphans: tickets in KnownTickets but absent from current claims.
	// A claim released on purpose (oh team release, end of a session, the
	// board, the retention of the done claims) is not an orphan: it left on
	// purpose and the next snapshot no longer lists it.
	var orphans []KnownTicket
	for _, kt := range known {
		if claimedTickets[kt.TicketID] || (kt.ExternalIID > 0 && e.index.taken(trackerProjectID, kt.ExternalIID)) {
			continue
		}
		if e.removedOnPurpose(ctx, hubProjectID, kt.TicketID, kt.ExternalIID) {
			slog.Debug("tracker.orphan.released", "project", hubProjectID, "ticket", kt.TicketID)
			continue
		}
		orphans = append(orphans, kt)
	}
	if len(orphans) == 0 {
		return nil
	}

	slog.Warn("tracker.orphan.detected", "project", hubProjectID, "count", len(orphans))

	// Fetch each orphan individually and recreate the claim
	var relPaths []string
	for _, orphan := range orphans {
		// The member who held it may have left the team since: the claim does
		// not come back under the name of someone who is not there.
		if orphan.ClaimedBy != "" && !e.repo.HasMember(ctx, orphan.ClaimedBy) {
			slog.Debug("tracker.orphan.member_gone", "ticket", orphan.TicketID, "member", orphan.ClaimedBy)
			continue
		}
		issue, err := e.tracker.FetchIssue(ctx, trackerProjectID, orphan.ExternalIID)
		if err != nil {
			if errors.Is(err, ErrIssueNotFound) {
				slog.Debug("tracker.orphan.gone", "ticket", orphan.TicketID, "iid", orphan.ExternalIID)
				continue // issue deleted on tracker — not a real orphan
			}
			slog.Warn("tracker.orphan.fetch_failed", "ticket", orphan.TicketID, "error", err)
			continue
		}
		if issue.IsClosed() {
			slog.Debug("tracker.orphan.closed", "ticket", orphan.TicketID)
			continue // closed on tracker — no need to restore
		}

		// Recreate the claim with current tracker data
		initialStatus := MapTrackerStatusWithBoard(issue, e.cfg.StatusMapping, e.cfg.LabelStatusMapping, e.boardCfg)
		truncDesc := TruncateDescription(issue.Description)
		claim := teamstate.Claim{
			TicketID:      orphan.TicketID,
			Project:       hubProjectID,
			ClaimedBy:     orphan.ClaimedBy,
			Status:        initialStatus,
			Title:         issue.Title,
			Description:   truncDesc,
			TrackerStatus: issue.StatusName,
			TrackerState:  issueState(issue),
			TrackerLabels: issue.Labels,
			Labels:        issue.Labels,
			ExternalIID:   orphan.ExternalIID,
		}
		relPath, err := e.repo.CreateClaimLocal(claim)
		if err != nil {
			if err == teamstate.ErrClaimExists {
				continue // race: claim was recreated between ListClaims and now
			}
			slog.Warn("tracker.orphan.create_failed", "ticket", orphan.TicketID, "error", err)
			continue
		}
		relPaths = append(relPaths, relPath)
		e.noteClaimed(hubProjectID, trackerProjectID, claimedTickets, orphan.TicketID, orphan.ExternalIID)
		pr.ClaimsCreated++
		slog.Info("tracker.orphan.recovered", "ticket", orphan.TicketID, "status", initialStatus)
	}

	if len(relPaths) == 0 {
		return nil
	}

	msg := fmt.Sprintf("orphan-recovery: %d tickets for %s", len(relPaths), hubProjectID)
	return e.repo.CommitAndPush(ctx, msg, relPaths...)
}

// autoPlanWindow bounds the assigned issues fetched per member: far more than
// any realistic board, so that an old issue is not hidden by newer ones.
const autoPlanWindow = 100

// autoplan creates claims for tracker issues assigned to team members but not
// yet claimed. Claims are created locally in batch and committed in a single
// git operation.
//
// The sync reflects the tracker: every open issue assigned to a member comes
// onto the board, in the column its status maps to, without any cap per
// member. What bounds a member is the number of tickets they work on at once
// (max_in_progress_per_member), checked where a ticket is taken or started,
// never here.
func (e *Engine) autoplan(
	ctx context.Context,
	hubProjectID, trackerProjectID string,
	trackerType Type,
	memberByGitLab map[string]teamstate.Member,
	claimedTickets map[string]bool,
	firstSync bool,
	pr *ProjectSyncResult,
) error {
	// The first complete scan of a project does not filter on the update
	// time: an issue assigned long ago and not touched since has never been
	// seen by a run that did.
	var lastSync time.Time
	fullScan := firstSync || !e.state.AutoPlanScan(trackerType, trackerProjectID)
	if !fullScan {
		lastSync = e.state.LastSync(trackerType, trackerProjectID)
	}
	slog.Debug("tracker.autoplan.start", "project", hubProjectID, "members", len(memberByGitLab), "first_sync", firstSync)

	usernames := make([]string, 0, len(memberByGitLab))
	for username := range memberByGitLab {
		usernames = append(usernames, username)
	}
	sort.Strings(usernames)

	// Collect all claims to create, then batch commit.
	var pending []teamstate.Claim
	scanned := true
	for _, username := range usernames {
		member := memberByGitLab[username]
		issues, err := e.tracker.ListAssignedIssues(ctx, trackerProjectID, ListOpts{
			AssigneeUsername: username,
			UpdatedAfter:     lastSync,
			MaxResults:       autoPlanWindow,
		})
		if err != nil {
			if errors.Is(err, ErrTokenInvalid) {
				return err
			}
			slog.Warn("tracker.autoplan.member_error", "username", username, "error", err)
			scanned = false // the scan starts again
			continue
		}
		slog.Debug("tracker.autoplan.member", "username", username, "issues", len(issues))

		for _, issue := range issues {
			ticketID := ticketIDFromIssue(issue, e.patternOf(hubProjectID))
			if e.issueClaimed(claimedTickets, trackerProjectID, &issue, ticketID) || issue.IsClosed() {
				continue
			}
			// A claim released on purpose is not made again by the complete
			// scan; it comes back when the issue is updated on the tracker (the
			// scans that follow only see what changed since the last sync).
			if fullScan && e.removedOnPurpose(ctx, hubProjectID, ticketID, issue.IID) {
				continue
			}
			// Use the configurable status mapping instead of always "planned".
			initialStatus := MapTrackerStatusWithBoard(&issue, e.cfg.StatusMapping, e.cfg.LabelStatusMapping, e.boardCfg)
			pending = append(pending, teamstate.Claim{
				TicketID:      ticketID,
				Project:       hubProjectID,
				ClaimedBy:     member.ID,
				Status:        initialStatus,
				Title:         issue.Title,
				Description:   TruncateDescription(issue.Description),
				TrackerStatus: issue.StatusName,
				TrackerState:  issueState(&issue),
				TrackerLabels: issue.Labels,
				ExternalIID:   issue.IID,
			})
			e.noteClaimed(hubProjectID, trackerProjectID, claimedTickets, ticketID, issue.IID)
		}
	}
	if scanned {
		e.state.MarkAutoPlanScanned(trackerType, trackerProjectID, time.Now().UTC())
	}

	if len(pending) == 0 {
		return nil
	}

	// Batch create: all the files, one commit+push (the transaction of the run
	// pulled when it started).
	var relPaths []string
	for i := range pending {
		relPath, err := e.repo.CreateClaimLocal(pending[i])
		if err != nil {
			if err == teamstate.ErrClaimExists {
				continue
			}
			slog.Warn("tracker.autoplan.create_failed", "ticket", pending[i].TicketID, "error", err)
			continue
		}
		relPaths = append(relPaths, relPath)
		slog.Debug("tracker.autoplan.created", "ticket", pending[i].TicketID, "member", pending[i].ClaimedBy)
		pr.ClaimsCreated++
	}

	if len(relPaths) == 0 {
		return nil
	}

	msg := fmt.Sprintf("autoplan: %d tickets for %s", len(relPaths), hubProjectID)
	return e.repo.CommitAndPush(ctx, msg, relPaths...)
}

// poolClaim is the claim of an issue of the pool: the claim of the member it is
// assigned to when it is assigned to a known one, otherwise a pool claim
// (ClaimedBy="").
func (e *Engine) poolClaim(issue IssueState, ticketID, hubProjectID string, memberByGitLab map[string]teamstate.Member) teamstate.Claim {
	claimedBy := ""
	e.noteUnknownAssignees(issue.Assignees, memberByGitLab)
	if len(issue.Assignees) > 0 {
		if m, ok := memberByGitLab[strings.ToLower(issue.Assignees[0])]; ok {
			claimedBy = m.ID
		}
	}
	return teamstate.Claim{
		TicketID:      ticketID,
		Project:       hubProjectID,
		ClaimedBy:     claimedBy,
		Status:        MapTrackerStatusWithBoard(&issue, e.cfg.StatusMapping, e.cfg.LabelStatusMapping, e.boardCfg),
		Title:         issue.Title,
		Description:   TruncateDescription(issue.Description),
		TrackerStatus: issue.StatusName,
		TrackerState:  issueState(&issue),
		TrackerLabels: issue.Labels,
		ExternalIID:   issue.IID,
		Labels:        issue.Labels,
	}
}

// maxPoolWindow bounds the issues fetched for the pool in one run.
const maxPoolWindow = 2000

// defaultMaxUnassigned is max_unassigned_issues when the team sets none.
const defaultMaxUnassigned = 20

// autopoolUnassigned creates claims for tracker issues so they appear on the
// team board as claimable tickets.
//
// When UnassignedLabels is configured, it fetches the open issues carrying ANY
// of those labels (an OR: one request per label, an issue found by several
// labels counted once), regardless of assignee, via ListIssuesByLabels. Issues
// assigned to a known team member get ClaimedBy=member.ID; others become pool
// claims (ClaimedBy="").
//
// When UnassignedLabels is empty, it falls back to the original behaviour:
// fetching only issues with no assignee via ListUnassignedIssues (pool claims only).
//
// The complete scan of a project's pool (and the one that follows a change of
// unassigned_labels) does not filter on the update time: an issue labelled
// long ago and not touched since has never been seen by a run that did. The
// scan takes as many runs as the pool needs: a run creates at most
// MaxUnassignedIssues claims, and the next one starts over from the whole pool,
// skipping the issues that have a claim by then (the claims committed are the
// progress, so a run that failed to read or to commit leaves nothing to undo
// and is simply made again). The scan is recorded as complete only by a run
// that found nothing left to create; the runs that follow only fetch what was
// updated since the last sync.
//
// The fetch is wide enough (MaxUnassignedIssues plus the issues that already
// have a claim) for the claimed ones not to hide the others. Claims are created
// locally in batch and committed in a single git operation.
func (e *Engine) autopoolUnassigned(
	ctx context.Context,
	hubProjectID, trackerProjectID string,
	trackerType Type,
	memberByGitLab map[string]teamstate.Member,
	claimedTickets map[string]bool,
	firstSync bool,
	pr *ProjectSyncResult,
) error {
	limit := e.cfg.MaxUnassignedIssues
	if limit <= 0 {
		limit = defaultMaxUnassigned
	}
	fullScan := firstSync || !e.state.AutoPoolScan(trackerType, trackerProjectID, e.cfg.UnassignedLabels)
	var lastSync time.Time
	if !fullScan {
		lastSync = e.state.LastSync(trackerType, trackerProjectID)
	}
	slog.Debug("tracker.autopool.start", "project", hubProjectID, "first_sync", firstSync, "full_scan", fullScan, "has_labels", len(e.cfg.UnassignedLabels) > 0)

	type pendingClaim struct {
		claim teamstate.Claim
	}
	var (
		pending   []pendingClaim
		more      bool // an issue is left that this run does not create (the cap)
		truncated bool
		uncertain bool
	)

	// The window of the fetch is the limit plus the claims, so that the claimed
	// issues do not hide the others. Claims released on purpose are skipped by
	// a scan and are not claims: when they crowd the window, it is widened by
	// their number and the fetch is made again.
	window := min(limit+len(claimedTickets)+e.index.count(trackerProjectID), maxPoolWindow)
	for {
		// Fetch issues: use ListIssuesByLabels (all assignees) when labels are
		// configured, otherwise fall back to ListUnassignedIssues (no assignee only).
		var issues []IssueState
		var err error
		if len(e.cfg.UnassignedLabels) > 0 {
			issues, err = e.tracker.ListIssuesByLabels(ctx, trackerProjectID, ListByLabelsOpts{
				Labels:       e.cfg.UnassignedLabels,
				UpdatedAfter: lastSync,
				MaxResults:   window,
			})
		} else {
			issues, err = e.tracker.ListUnassignedIssues(ctx, trackerProjectID, ListUnassignedOpts{
				Labels:       e.cfg.UnassignedLabels,
				UpdatedAfter: lastSync,
				MaxResults:   window,
			})
		}
		if err != nil {
			if errors.Is(err, ErrTokenInvalid) {
				return err
			}
			slog.Warn("tracker.autopool.fetch_error", "error", err)
			return nil // skip on transient errors: the scan starts again
		}
		slog.Debug("tracker.autopool.fetched", "count", len(issues))

		pending, more = nil, false
		released := 0
		for _, issue := range issues {
			ticketID := ticketIDFromIssue(issue, e.patternOf(hubProjectID))
			if e.issueClaimed(claimedTickets, trackerProjectID, &issue, ticketID) {
				continue
			}
			// A claim released on purpose is not made again by the passes of a
			// scan; it comes back if the issue is updated on the tracker.
			if fullScan && e.removedOnPurpose(ctx, hubProjectID, ticketID, issue.IID) {
				released++
				continue
			}
			if len(pending) >= limit {
				more = true
				break
			}
			pending = append(pending, pendingClaim{claim: e.poolClaim(issue, ticketID, hubProjectID, memberByGitLab)})
		}

		// The pool is covered when nothing is left behind: not by the cap, and
		// not by the window of the fetch. A fetch that filled its window with
		// the run at its cap may have more behind: another run settles it, with
		// a wider window.
		truncated = len(issues) >= window
		uncertain = truncated && window < maxPoolWindow && len(pending) >= limit
		if truncated && !more && !uncertain && released > 0 && window < maxPoolWindow {
			window = min(window+released, maxPoolWindow)
			continue
		}
		break
	}
	for i := range pending {
		e.noteClaimed(hubProjectID, trackerProjectID, claimedTickets, pending[i].claim.TicketID, pending[i].claim.ExternalIID)
	}
	covered := !more && !uncertain
	if fullScan && truncated && !uncertain && !more {
		slog.Warn("tracker.autopool.window_full", "project", hubProjectID, "window", window)
	}
	// Recorded only once what the run created is committed.
	finish := func(created int) {
		if !fullScan {
			return
		}
		now := time.Now().UTC()
		if covered {
			e.state.MarkAutoPoolScanned(trackerType, trackerProjectID, e.cfg.UnassignedLabels, now)
			return
		}
		if created > 0 {
			e.state.NoteAutoPoolPass(trackerType, trackerProjectID, e.cfg.UnassignedLabels, now)
		}
	}

	if len(pending) == 0 {
		finish(0)
		return nil
	}

	// Batch create: all the files, one commit+push (the transaction of the run
	// pulled when it started).
	var relPaths []string
	createFailed := false
	for i := range pending {
		relPath, err := e.repo.CreateClaimLocal(pending[i].claim)
		if err != nil {
			if err == teamstate.ErrClaimExists {
				continue
			}
			slog.Warn("tracker.autopool.create_failed", "ticket", pending[i].claim.TicketID, "error", err)
			createFailed = true // the scan starts again
			continue
		}
		relPaths = append(relPaths, relPath)
		slog.Debug("tracker.autopool.created", "ticket", pending[i].claim.TicketID, "claimed_by", pending[i].claim.ClaimedBy)
		pr.ClaimsCreated++
	}

	if len(relPaths) == 0 {
		if !createFailed {
			finish(0)
		}
		return nil
	}

	msg := fmt.Sprintf("autopool: %d tickets for %s", len(relPaths), hubProjectID)
	if err := e.repo.CommitAndPush(ctx, msg, relPaths...); err != nil {
		return err
	}
	if createFailed {
		covered = false // a claim that could not be written is made again
	}
	finish(len(relPaths))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// matchLabelMapping looks for the status the labels of the issue map to.
// Returns the status and true if one does, or ("", false) if none does.
//
// When several labels of the issue are mapped, the winner does not depend on
// the order of the labels of the issue (the tracker's) nor on the order of the
// map (random):
//  1. a status of a blocked column (role "blocked", or "blocked" on the default
//     board) always wins: an impediment is never hidden by another label;
//  2. otherwise the most advanced column wins, the one furthest in the order of
//     the board (on the default board: done, validation, review, in progress,
//     then todo), so that an issue does not fall back because of an old label;
//  3. a tie (two labels for the same status, or statuses that cannot be
//     ordered) goes to the first label in alphabetical order, case ignored.
//
// A value that stands for no status of the board is left out, so that it never
// hides another label. A value like "todo" that stands for the initial column
// is read as the status of that column (see BoardConfig.ResolveStatus).
func matchLabelMapping(issue *IssueState, labelStatusMapping map[string]string, boardCfg teamstate.BoardConfig) (string, bool) {
	if len(labelStatusMapping) == 0 || len(issue.Labels) == 0 {
		return "", false
	}
	// Keys that differ by case only: the first in alphabetical order, so that
	// the answer is the same at every run.
	keys := make([]string, 0, len(labelStatusMapping))
	for k := range labelStatusMapping {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lowerMap := make(map[string]string, len(keys))
	for _, k := range keys {
		if _, dup := lowerMap[strings.ToLower(k)]; !dup {
			lowerMap[strings.ToLower(k)] = labelStatusMapping[k]
		}
	}

	var (
		best        string
		bestBlocked bool
		bestRank    int
		bestName    string
		found       bool
	)
	for _, label := range issue.Labels {
		v, ok := lowerMap[strings.ToLower(label)]
		if !ok {
			continue
		}
		status, _, valid := boardCfg.ResolveStatus(v)
		if !valid {
			continue
		}
		blocked, rank, name := boardCfg.IsBlockedStatus(status), boardCfg.StatusRank(status), strings.ToLower(label)
		better := !found ||
			(blocked != bestBlocked && blocked) ||
			(blocked == bestBlocked && (rank > bestRank || (rank == bestRank && name < bestName)))
		if better {
			best, bestBlocked, bestRank, bestName, found = status, blocked, rank, name, true
		}
	}
	return best, found
}

// matchStatusNameMapping checks if the issue's status name matches the statusMapping.
// Returns the mapped status and true if found and standing for a status of the
// board, or ("", false) otherwise.
func matchStatusNameMapping(issue *IssueState, statusMapping map[string]string, boardCfg teamstate.BoardConfig) (string, bool) {
	if len(statusMapping) == 0 || issue.StatusName == "" {
		return "", false
	}
	nameLower := strings.ToLower(issue.StatusName)
	keys := make([]string, 0, len(statusMapping))
	for k := range statusMapping {
		if strings.ToLower(k) == nameLower {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if status, _, ok := boardCfg.ResolveStatus(statusMapping[k]); ok {
			return status, true
		}
	}
	return "", false
}

// MapTrackerStatus maps a tracker issue's status to a claim status on the
// default board; see MapTrackerStatusWithBoard.
func MapTrackerStatus(issue *IssueState, statusMapping, labelStatusMapping map[string]string) string {
	return MapTrackerStatusWithBoard(issue, statusMapping, labelStatusMapping, teamstate.BoardConfig{})
}

// MapTrackerStatusWithBoard maps a tracker issue's status to a claim status
// using the configured mappings and the BoardConfig, so that the custom
// column IDs are used instead of the hardcoded defaults when the team has
// configured columns.
//
// Resolution order:
//  1. LabelStatusMapping: when several labels of the issue are mapped, the one
//     whose status comes first among the columns of the board wins, then the
//     first in alphabetical order (see matchLabelMapping)
//  2. StatusMapping[issue.StatusName] (case-insensitive)
//  3. Category fallback: Jira statusCategory / GitLab state
//     - "done" / "closed" → the terminal status
//     - "new" → the initial status
//     - "indeterminate" / "opened" → the status of work in progress
//
// A mapped value that is not a status of the board ("todo" for the initial
// column) is read as the status it stands for; one that stands for nothing is
// ignored (the engine warns of it, see reconcileAll).
func MapTrackerStatusWithBoard(issue *IssueState, statusMapping, labelStatusMapping map[string]string, boardCfg teamstate.BoardConfig) string {
	if v, ok := matchLabelMapping(issue, labelStatusMapping, boardCfg); ok {
		return v
	}
	if v, ok := matchStatusNameMapping(issue, statusMapping, boardCfg); ok {
		return v
	}
	// Category fallback using board config
	if boardCfg.HasCustomColumns() {
		switch strings.ToLower(issue.StatusCategory) {
		case "done", "closed":
			ts := boardCfg.TerminalStatuses()
			if len(ts) > 0 {
				return ts[0]
			}
		case "new":
			return boardCfg.InitialStatus()
		default:
			return boardCfg.DefaultWorkStatus()
		}
	}
	switch strings.ToLower(issue.StatusCategory) {
	case "done", "closed":
		return teamstate.ClaimStatusDone
	case "new":
		return teamstate.ClaimStatusPlanned
	default:
		return teamstate.ClaimStatusInProgress
	}
}

// resolveIID returns the external IID for a ticket.
// Priority: stored ExternalIID > pattern extraction from TicketID.
func resolveIID(ticketID string, storedIID int, pattern string) (int, bool) {
	if storedIID > 0 {
		return storedIID, true
	}
	if pattern == "" {
		return 0, false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return 0, false
	}
	matches := re.FindStringSubmatch(ticketID)
	if len(matches) < 2 {
		return 0, false
	}
	iid, err := strconv.Atoi(matches[1])
	if err != nil || iid <= 0 {
		return 0, false
	}
	return iid, true
}

// issueState is the state of the issue as the claim records it: "open" or
// "closed".
func issueState(issue *IssueState) string {
	if issue.IsClosed() {
		return "closed"
	}
	return "open"
}

// issueVerdict says what the sync learns from an issue about the status of its
// claim.
type issueVerdict int

const (
	// issueUnchanged: the state and the labels attached to a column are those
	// of the previous pass. The status of the claim is the hub's.
	issueUnchanged issueVerdict = iota
	// issueChanged: the issue moved (or closed, or reopened) since the previous
	// pass; the mapping of the tracker gives the status.
	issueChanged
	// issueUnseen: no previous pass to compare with, and no event. The pass
	// only records what it sees.
	issueUnseen
)

// statusFromTracker compares the issue with what the previous pass recorded
// on the claim (TrackerState, TrackerStatus, TrackerLabels). The tracker gives
// a status to the claim only when the issue changed: its state, its status
// name or the labels attached to a column of the board (label_status_mapping).
// An issue that stays open without a label of status must not take back, at
// every pass, a card moved on the board, an MR, a verdict or the end of a
// session.
//
// A claim no pass of 5.3.3 or later has seen has nothing to be compared with:
// the pass records the issue and leaves the status, except for the two events
// the claim itself tells: an issue closed while the claim is not in a terminal
// column, and an issue the claim had recorded as closed (GitLab) and that is
// open again while the claim is terminal.
func (e *Engine) statusFromTracker(c *teamstate.Claim, issue *IssueState) issueVerdict {
	terminal := slices.Contains(e.boardCfg.TerminalStatuses(), c.Status)
	if c.TrackerState == "" {
		switch {
		case issue.IsClosed() && !terminal:
			return issueChanged
		case !issue.IsClosed() && terminal && strings.EqualFold(c.TrackerStatus, "closed"):
			return issueChanged
		}
		return issueUnseen
	}
	if c.TrackerState != issueState(issue) ||
		(c.TrackerStatus != "" && issue.StatusName != "" && c.TrackerStatus != issue.StatusName) ||
		e.mappedLabelsKey(c.TrackerLabels) != e.mappedLabelsKey(issue.Labels) {
		return issueChanged
	}
	return issueUnchanged
}

// mappedLabelsKey identifies, among labels, the ones label_status_mapping
// attaches to a status of the board (case ignored): the same key, the same
// labels attached to a column.
func (e *Engine) mappedLabelsKey(labels []string) string {
	var mapped []string
	for _, l := range labels {
		if _, ok := e.statusOfLabel(l); ok {
			mapped = append(mapped, strings.ToLower(l))
		}
	}
	sort.Strings(mapped)
	return strings.Join(slices.Compact(mapped), "\x00")
}

// statusOfLabel is the status of the board a label is attached to by
// label_status_mapping (case ignored), when it is attached to one.
func (e *Engine) statusOfLabel(label string) (string, bool) {
	keys := make([]string, 0, len(e.cfg.LabelStatusMapping))
	for k := range e.cfg.LabelStatusMapping {
		if strings.EqualFold(k, label) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		if status, _, ok := e.boardCfg.ResolveStatus(e.cfg.LabelStatusMapping[k]); ok {
			return e.canonStatus(status), true
		}
	}
	return "", false
}

// canonStatus is the status as the board names it: the legacy "planned" is
// the initial column.
func (e *Engine) canonStatus(status string) string {
	if status == teamstate.ClaimStatusPlanned {
		return e.boardCfg.InitialStatus()
	}
	return status
}

// pushStatusLabel pushes the status of the hub to the tracker as the label
// attached to its column (push_labels), when the issue says another one: the
// label is added and the labels attached to other columns are removed, so that
// the mapping gives the status of the hub back. Without a label attached to
// the column, nothing is pushed. It returns the labels the issue now has, the
// ones it added and the ones it removed.
func (e *Engine) pushStatusLabel(
	ctx context.Context,
	trackerProjectID string, iid int,
	issue *IssueState, hubStatus, mappedStatus string, labels []string,
	pr *ProjectSyncResult,
) (now, added, removed []string) {
	hub := e.canonStatus(hubStatus)
	if hub == e.canonStatus(mappedStatus) {
		return labels, nil, nil
	}
	var target string
	keys := make([]string, 0, len(e.cfg.LabelStatusMapping))
	for k := range e.cfg.LabelStatusMapping {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		li, lj := strings.ToLower(keys[i]), strings.ToLower(keys[j])
		return li < lj || (li == lj && keys[i] < keys[j])
	})
	for _, k := range keys {
		if st, ok := e.statusOfLabel(k); ok && st == hub {
			if target == "" {
				target = k
			}
			if slices.ContainsFunc(labels, func(l string) bool { return strings.EqualFold(l, k) }) {
				target = k
				break
			}
		}
	}
	if target == "" {
		slog.Debug("tracker.push.no_status_label", "issue", issue.IID, "status", hub)
		return labels, nil, nil
	}
	var toAdd, toRemove []string
	if !slices.ContainsFunc(labels, func(l string) bool { return strings.EqualFold(l, target) }) {
		toAdd = append(toAdd, target)
	}
	for _, l := range labels {
		if st, ok := e.statusOfLabel(l); ok && st != hub {
			toRemove = append(toRemove, l)
		}
	}
	if len(toAdd) == 0 && len(toRemove) == 0 {
		return labels, nil, nil
	}
	warn := func(verb string, err error) {
		slog.Warn("tracker.push.status_label_failed", "issue", iid, "verb", verb, "error", err)
		if !errors.Is(err, ErrWriteDisabled) {
			pr.Warnings = append(pr.Warnings, fmt.Sprintf("issue %d: %s status label failed: %v", iid, verb, err))
		}
	}
	out := slices.Clone(labels)
	if len(toAdd) > 0 {
		if err := e.tracker.AddLabels(ctx, trackerProjectID, iid, toAdd); err != nil {
			warn("add", err)
			return labels, nil, nil
		}
		pr.LabelsPushed += len(toAdd)
		out = append(out, toAdd...)
		added = toAdd
	}
	if len(toRemove) > 0 {
		if err := e.tracker.RemoveLabels(ctx, trackerProjectID, iid, toRemove); err != nil {
			warn("remove", err)
			return out, added, nil
		}
		out = slices.DeleteFunc(out, func(l string) bool { return slices.Contains(toRemove, l) })
		removed = toRemove
	}
	return out, added, removed
}

func labelSet(labels []string) map[string]bool {
	m := make(map[string]bool, len(labels))
	for _, l := range labels {
		m[l] = true
	}
	return m
}

func hasLabel(labels []string, target string) bool {
	for _, l := range labels {
		if l == target {
			return true
		}
	}
	return false
}

// isTrackerMirroredLabel returns false for the labels the hub owns
// ("agent-reviewed", "needs-human-review", "hub:done"): a tracker that does
// not carry them does not take them off the claim. For the others, only a label
// the sync saw on the issue (TrackerLabels) can be taken off when it leaves it.
func isTrackerMirroredLabel(l string) bool {
	switch l {
	case teamstate.LabelAgentReviewed,
		teamstate.LabelNeedsHumanReview,
		teamstate.LabelTrackerDone:
		return false
	}
	return true
}

// hubLabelsToSync returns the hub-generated labels that should be pushed to
// the tracker (those not already present on the issue).
func hubLabelsToSync(claimLabels, issueLabels []string) []string {
	issueSet := labelSet(issueLabels)
	var toAdd []string
	for _, l := range claimLabels {
		// Only push hub-specific labels that are meant to be mirrored outward.
		if l == teamstate.LabelAgentReviewed || l == teamstate.LabelTrackerDone {
			if !issueSet[l] {
				toAdd = append(toAdd, l)
			}
		}
	}
	return toAdd
}
