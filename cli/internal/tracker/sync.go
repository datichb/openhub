package tracker

import (
	"context"
	"errors"
	"fmt"
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
		if m.GitLabUsername != "" {
			memberByGitLab[strings.ToLower(m.GitLabUsername)] = m
		}
	}

	trackerType := Type(e.cfg.Type)

	for hubProjectID, trackerProjectID := range e.cfg.Projects {
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
	_ = e.state.Save(e.stateDir)

	return result, nil
}

// ShouldAutoSync reports whether the engine is eligible for an automatic sync.
// Returns false after 3 consecutive token errors to avoid spamming the UI.
func (e *Engine) ShouldAutoSync() bool {
	return e.consecutiveTokenErrors < 3
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
			continue // no tracker link for this ticket
		}

		issue, err := e.tracker.FetchIssue(ctx, trackerProjectID, iid)
		if err != nil {
			if errors.Is(err, ErrIssueNotFound) {
				continue // ticket deleted on tracker — ignore
			}
			return pr, err // propagate fatal errors
		}
		pr.IssuesFetched++

		// Store the resolved IID if not already set (avoids re-parsing next time).
		if c.ExternalIID == 0 {
			_ = e.repo.SetClaimExternalIID(ctx, hubProjectID, c.TicketID, iid)
		}

		// State transitions.
		if issue.IsClosed() && c.Status != teamstate.ClaimStatusDone {
			if err := e.repo.UpdateClaimStatus(ctx, hubProjectID, c.TicketID, teamstate.ClaimStatusDone); err == nil {
				pr.ClaimsUpdated++
			}
		} else if !issue.IsClosed() && c.Status == teamstate.ClaimStatusDone {
			if err := e.repo.UpdateClaimStatus(ctx, hubProjectID, c.TicketID, teamstate.ClaimStatusInProgress); err == nil {
				pr.ClaimsUpdated++
			}
		}

		// Label sync: mirror tracker labels onto the claim.
		trackerLabelSet := labelSet(issue.Labels)
		for _, l := range issue.Labels {
			if !hasLabel(c.Labels, l) {
				_ = e.repo.AddClaimLabel(ctx, hubProjectID, c.TicketID, l)
			}
		}
		for _, l := range c.Labels {
			// Remove claim labels that were deleted on the tracker side
			// (only those that originated from the tracker, not hub-specific ones).
			if isTrackerMirroredLabel(l) && !trackerLabelSet[l] {
				_ = e.repo.RemoveClaimLabel(ctx, hubProjectID, c.TicketID, l)
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
		if err := e.autoplan(ctx, hubProjectID, trackerProjectID, trackerType, memberByGitLab, claimedTickets, &pr); err != nil {
			// Non-fatal — log but continue.
			_ = err
		}
	}

	return pr, nil
}

// autoplan creates "planned" claims for tracker issues assigned to team members
// but not yet claimed. Respects MaxAutoPlanPerMember.
func (e *Engine) autoplan(
	ctx context.Context,
	hubProjectID, trackerProjectID string,
	trackerType Type,
	memberByGitLab map[string]teamstate.Member,
	claimedTickets map[string]bool,
	pr *ProjectSyncResult,
) error {
	lastSync := e.state.LastSync(trackerType, trackerProjectID)

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
			continue // skip this member on transient errors
		}

		for _, issue := range issues {
			if plannedByMember[member.ID] >= e.cfg.MaxAutoPlanPerMember {
				break
			}

			ticketID := ticketIDFromIssue(issue, e.cfg.TicketPatterns[hubProjectID])
			if claimedTickets[ticketID] {
				continue
			}

			_, claimErr := e.repo.CreateClaim(ctx, teamstate.Claim{
				TicketID:    ticketID,
				Project:     hubProjectID,
				ClaimedBy:   member.ID,
				Status:      teamstate.ClaimStatusPlanned,
				ExternalIID: issue.IID,
			})
			if claimErr != nil && claimErr != teamstate.ErrClaimExists {
				continue
			}
			claimedTickets[ticketID] = true
			plannedByMember[member.ID]++
			pr.ClaimsCreated++
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

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
