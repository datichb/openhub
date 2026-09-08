package tracker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

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
}

// SyncWarning is a non-fatal condition worth surfacing (e.g. reassignment).
type SyncWarning struct {
	ProjectID string
	TicketID  string
	Message   string
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

// Engine reconciles team-state claims with an external tracker.
type Engine struct {
	tracker  Tracker
	repo     teamstate.TeamStateWriter
	cfg      teamstate.TrackerConfig
	state    *SyncState
	stateDir string
	// consecutiveTokenErrors tracks auth failures to mute auto-sync after 3.
	consecutiveTokenErrors int
}

// NewEngine creates a sync engine.
// stateDir is the local directory for sync-state.json (~/.oh/).
func NewEngine(t Tracker, repo teamstate.TeamStateWriter, cfg teamstate.TrackerConfig, stateDir string) *Engine {
	return &Engine{
		tracker:  t,
		repo:     repo,
		cfg:      cfg,
		stateDir: stateDir,
	}
}

// Run executes a full sync cycle and returns the aggregated result.
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
func (e *Engine) Run(ctx context.Context) (*SyncResult, error) {
	slog.Debug("tracker.sync.start", "projects", len(e.cfg.Projects), "autoplan", e.cfg.AutoPlanAssigned)
	start := time.Now()

	// Global timeout to bound the total sync wall time.
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Load (or initialise) the local sync state.
	state, err := LoadSyncState(e.stateDir)
	if err != nil {
		return nil, fmt.Errorf("sync engine: loading state: %w", err)
	}
	e.state = state

	result := &SyncResult{SyncedAt: time.Now().UTC()}

	// Pull to start from a fresh baseline.
	if pullErr := e.repo.Pull(ctx); pullErr != nil && !teamstate.IsPullWarning(pullErr) {
		return nil, fmt.Errorf("sync engine: pull failed: %w", pullErr)
	}

	// Load members once (needed for auto-plan and assignee lookup).
	members, err := e.repo.ListMembers()
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

	trackerType := Type(e.cfg.Type)

	for hubProjectID, trackerProjectID := range e.cfg.Projects {
		slog.Debug("tracker.sync.project", "hub_project", hubProjectID, "tracker_project", trackerProjectID)
		start := time.Now()
		pr, projectErr := e.reconcileProject(ctx, hubProjectID, trackerProjectID, trackerType, memberByGitLab)
		pr.Duration = time.Since(start)
		result.Projects = append(result.Projects, pr)
		result.ClaimsCreated += pr.ClaimsCreated
		result.ClaimsUpdated += pr.ClaimsUpdated
		result.LabelsPushed += pr.LabelsPushed

		if projectErr != nil {
			// Fatal errors (token invalid, rate limit) abort the entire run.
			if errors.Is(projectErr, ErrTokenInvalid) {
				e.consecutiveTokenErrors++
				return result, projectErr
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

	// Reset token error counter on a successful run.
	e.consecutiveTokenErrors = 0

	// Persist the updated sync timestamps.
	if err := e.state.Save(e.stateDir); err != nil {
		slog.Warn("tracker.sync.state_save_failed", "error", err)
	}

	// Commit and push all claim updates (status, metadata, labels) from the reconciliation loop.
	// Without this, local TOML changes would be discarded on the next Pull.
	if result.ClaimsCreated+result.ClaimsUpdated+result.LabelsPushed > 0 {
		commitMsg := fmt.Sprintf("sync: %d created, %d updated, %d labels pushed",
			result.ClaimsCreated, result.ClaimsUpdated, result.LabelsPushed)
		if err := e.repo.CommitAndPush(ctx, commitMsg, "."); err != nil {
			slog.Warn("tracker.sync.push_failed", "error", err)
		}
	}

	slog.Debug("tracker.sync.complete", "duration", time.Since(start), "created", result.ClaimsCreated, "updated", result.ClaimsUpdated)

	return result, nil
}

// ShouldAutoSync reports whether the engine is eligible for an automatic sync.
// Returns false after 3 consecutive token errors to avoid spamming the UI.
func (e *Engine) ShouldAutoSync() bool {
	return e.consecutiveTokenErrors < 3
}

// FetchTicketDetail fetches the full title and description of a ticket from the
// external tracker. Used for on-demand display in the TUI detail view.
// Returns the title and the full (non-truncated) description.
func (e *Engine) FetchTicketDetail(ctx context.Context, hubProjectID, ticketID string) (title, description string, err error) {
	trackerProjectID, ok := e.cfg.Projects[hubProjectID]
	if !ok {
		// Try single-project config fallback
		if len(e.cfg.Projects) == 1 {
			for _, v := range e.cfg.Projects {
				trackerProjectID = v
			}
		} else {
			return "", "", fmt.Errorf("no tracker project configured for %q", hubProjectID)
		}
	}

	pattern := e.cfg.TicketPatterns[hubProjectID]

	// Try to get the claim to read the stored ExternalIID.
	var storedIID int
	if c, err := e.repo.GetClaim(hubProjectID, ticketID); err == nil {
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
	pattern := e.cfg.TicketPatterns[hubProjectID]

	// Load all claims for this project.
	claims, err := e.repo.ListClaims(hubProjectID)
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
		}

		// Status sync: use configurable mapping.
		mappedStatus := MapTrackerStatus(issue, e.cfg.StatusMapping, e.cfg.LabelStatusMapping)
		if mappedStatus != c.Status {
			slog.Debug("tracker.reconcile.transition", "ticket", c.TicketID, "from", c.Status, "to", mappedStatus, "tracker_status", issue.StatusName)
			if err := e.repo.UpdateClaimStatusFromTracker(ctx, hubProjectID, c.TicketID, mappedStatus); err == nil {
				pr.ClaimsUpdated++
			} else {
				slog.Warn("tracker.reconcile.status_failed", "ticket", c.TicketID, "error", err)
			}
		}

		// Label sync: mirror tracker labels onto the claim.
		trackerLabelSet := labelSet(issue.Labels)
		for _, l := range issue.Labels {
			if !hasLabel(c.Labels, l) {
				if err := e.repo.AddClaimLabel(ctx, hubProjectID, c.TicketID, l); err != nil {
					slog.Warn("tracker.reconcile.label_failed", "ticket", c.TicketID, "label", l, "error", err)
				}
			}
		}
		for _, l := range c.Labels {
			// Remove claim labels that were deleted on the tracker side
			// (only those that originated from the tracker, not hub-specific ones).
			if isTrackerMirroredLabel(l) && !trackerLabelSet[l] {
				if err := e.repo.RemoveClaimLabel(ctx, hubProjectID, c.TicketID, l); err != nil {
					slog.Warn("tracker.reconcile.label_failed", "ticket", c.TicketID, "label", l, "error", err)
				}
			}
		}

		// Assignee change warning.
		if len(issue.Assignees) > 0 {
			assignee := strings.ToLower(issue.Assignees[0])
			if m, ok := memberByGitLab[assignee]; ok && m.ID != c.ClaimedBy {
				pr.ClaimsUpdated++ // counted as a warning-producing update
			}
		}

		// ── Push direction: claims → tracker ─────────────────────────────────
		if e.cfg.PushLabels {
			labelsToAdd := hubLabelsToSync(c.Labels, issue.Labels)
			if len(labelsToAdd) > 0 {
				if err := e.tracker.AddLabels(ctx, trackerProjectID, iid, labelsToAdd); err == nil {
					pr.LabelsPushed += len(labelsToAdd)
				}
			}
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

	// ── Orphan recovery: detect and restore deleted/corrupted claims ─────────
	if err := e.recoverOrphans(ctx, hubProjectID, trackerProjectID, claimedTickets, &pr); err != nil {
		slog.Warn("tracker.orphan.recovery_failed", "project", hubProjectID, "error", err)
	}

	// ── Snapshot known tickets for future orphan detection ───────────────────
	updatedClaims, _ := e.repo.ListClaims(hubProjectID)
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
// missing locally (deleted, corrupted, or lost to git conflicts). For each
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

	// Find orphans: tickets in KnownTickets but absent from current claims
	var orphans []KnownTicket
	for _, kt := range known {
		if !claimedTickets[kt.TicketID] {
			orphans = append(orphans, kt)
		}
	}
	if len(orphans) == 0 {
		return nil
	}

	slog.Warn("tracker.orphan.detected", "project", hubProjectID, "count", len(orphans))

	// Fetch each orphan individually and recreate the claim
	var relPaths []string
	for _, orphan := range orphans {
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
		initialStatus := MapTrackerStatus(issue, e.cfg.StatusMapping, e.cfg.LabelStatusMapping)
		truncDesc := TruncateDescription(issue.Description)
		claim := teamstate.Claim{
			TicketID:      orphan.TicketID,
			Project:       hubProjectID,
			ClaimedBy:     orphan.ClaimedBy,
			Status:        initialStatus,
			Title:         issue.Title,
			Description:   truncDesc,
			TrackerStatus: issue.StatusName,
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
		claimedTickets[orphan.TicketID] = true
		pr.ClaimsCreated++
		slog.Info("tracker.orphan.recovered", "ticket", orphan.TicketID, "status", initialStatus)
	}

	if len(relPaths) == 0 {
		return nil
	}

	msg := fmt.Sprintf("orphan-recovery: %d tickets for %s", len(relPaths), hubProjectID)
	return e.repo.CommitAndPush(ctx, msg, relPaths...)
}

// autoplan creates "planned" claims for tracker issues assigned to team members
// but not yet claimed. Respects MaxAutoPlanPerMember.
// Claims are created locally in batch and committed in a single git operation.
func (e *Engine) autoplan(
	ctx context.Context,
	hubProjectID, trackerProjectID string,
	trackerType Type,
	memberByGitLab map[string]teamstate.Member,
	claimedTickets map[string]bool,
	firstSync bool,
	pr *ProjectSyncResult,
) error {
	var lastSync time.Time
	if !firstSync {
		lastSync = e.state.LastSync(trackerType, trackerProjectID)
	}
	// firstSync: lastSync is zero → no updated_after filter → fetch all open issues
	slog.Debug("tracker.autoplan.start", "project", hubProjectID, "members", len(memberByGitLab), "first_sync", firstSync)

	// Collect all claims to create, then batch commit.
	type pendingClaim struct {
		claim   teamstate.Claim
		relPath string
	}
	var pending []pendingClaim

	plannedByMember := make(map[string]int)
	for username, member := range memberByGitLab {
		issues, err := e.tracker.ListAssignedIssues(ctx, trackerProjectID, ListOpts{
			AssigneeUsername: username,
			UpdatedAfter:     lastSync,
			MaxResults:       e.cfg.MaxAutoPlanPerMember,
		})
		if err != nil {
			if errors.Is(err, ErrTokenInvalid) {
				return err
			}
			slog.Warn("tracker.autoplan.member_error", "username", username, "error", err)
			continue // skip this member on transient errors
		}
		slog.Debug("tracker.autoplan.member", "username", username, "issues", len(issues))

		for _, issue := range issues {
			if plannedByMember[member.ID] >= e.cfg.MaxAutoPlanPerMember {
				break
			}

			ticketID := ticketIDFromIssue(issue, e.cfg.TicketPatterns[hubProjectID])
			if claimedTickets[ticketID] {
				continue
			}

			// Use the configurable status mapping instead of always "planned".
			initialStatus := MapTrackerStatus(&issue, e.cfg.StatusMapping, e.cfg.LabelStatusMapping)
			truncDesc := TruncateDescription(issue.Description)

			pending = append(pending, pendingClaim{
				claim: teamstate.Claim{
					TicketID:      ticketID,
					Project:       hubProjectID,
					ClaimedBy:     member.ID,
					Status:        initialStatus,
					Title:         issue.Title,
					Description:   truncDesc,
					TrackerStatus: issue.StatusName,
					ExternalIID:   issue.IID,
				},
			})
			claimedTickets[ticketID] = true
			plannedByMember[member.ID]++
		}
	}

	if len(pending) == 0 {
		return nil
	}

	// Batch create: single pull → create all files → single commit+push
	if err := e.repo.Pull(ctx); err != nil {
		slog.Warn("tracker.autoplan.pull_failed", "error", err)
		// Continue anyway — local creates may still succeed
	}

	var relPaths []string
	for i := range pending {
		relPath, err := e.repo.CreateClaimLocal(pending[i].claim)
		if err != nil {
			if err == teamstate.ErrClaimExists {
				continue
			}
			slog.Warn("tracker.autoplan.create_failed", "ticket", pending[i].claim.TicketID, "error", err)
			continue
		}
		relPaths = append(relPaths, relPath)
		slog.Debug("tracker.autoplan.created", "ticket", pending[i].claim.TicketID, "member", pending[i].claim.ClaimedBy)
		pr.ClaimsCreated++
	}

	if len(relPaths) == 0 {
		return nil
	}

	msg := fmt.Sprintf("autoplan: %d tickets for %s", len(relPaths), hubProjectID)
	return e.repo.CommitAndPush(ctx, msg, relPaths...)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// MapTrackerStatus maps a tracker issue's status to a claim status using the
// configured mappings. Falls back to category-based mapping when no match.
//
// Resolution order:
//  1. LabelStatusMapping[label] — first matching label wins (order matters)
//  2. StatusMapping[issue.StatusName] (case-insensitive)
//  3. Category fallback: Jira statusCategory / GitLab state
//     - "done" / "closed" → ClaimStatusDone
//     - "new" → ClaimStatusPlanned
//     - "indeterminate" / "opened" → ClaimStatusInProgress
func MapTrackerStatus(issue *IssueState, statusMapping, labelStatusMapping map[string]string) string {
	// 1. Try label-based mapping first (most relevant for GitLab label workflows).
	// Iterate over issue.Labels in tracker order — the FIRST matching label wins.
	// Pre-build a lowercase lookup map for O(1) matching.
	if len(labelStatusMapping) > 0 && len(issue.Labels) > 0 {
		lowerMap := make(map[string]string, len(labelStatusMapping))
		for k, v := range labelStatusMapping {
			lowerMap[strings.ToLower(k)] = v
		}
		for _, label := range issue.Labels {
			if v, ok := lowerMap[strings.ToLower(label)]; ok {
				if teamstate.IsValidStatus(v) {
					slog.Debug("tracker.sync.label_mapped",
						"ticket", issue.IID,
						"label", label,
						"status", v,
					)
					return v
				}
			}
		}
	}

	// 2. Try explicit status name mapping (case-insensitive).
	if len(statusMapping) > 0 && issue.StatusName != "" {
		nameLower := strings.ToLower(issue.StatusName)
		for k, v := range statusMapping {
			if strings.ToLower(k) == nameLower {
				if teamstate.IsValidStatus(v) {
					return v
				}
			}
		}
	}

	// 3. Category fallback.
	switch strings.ToLower(issue.StatusCategory) {
	case "done", "closed":
		return teamstate.ClaimStatusDone
	case "new":
		return teamstate.ClaimStatusPlanned
	default: // "indeterminate", "opened", or unknown
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

// ticketIDFromIssue derives the hub ticket ID from a tracker IssueState.
// For GitLab: "IID" (e.g. "42"). For Jira: "Key" (e.g. "SRU-42").
func ticketIDFromIssue(issue IssueState, _ string) string {
	if issue.Key != "" {
		return issue.Key
	}
	return strconv.Itoa(issue.IID)
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

// isTrackerMirroredLabel returns true for labels that were mirrored from the
// external tracker (i.e. not hub-specific ones like "agent-reviewed").
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
