package teamstate

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"golang.org/x/sync/errgroup"
)

// Claim status constants — the full lifecycle of a ticket on the team board.
const (
	// ClaimStatusPlanned represents a ticket reserved but not yet started.
	// Appears in the TODO column of the team board.
	ClaimStatusPlanned = "planned"
	// ClaimStatusInProgress is the default status when a claim is created.
	// Appears in the IN PROGRESS column.
	ClaimStatusInProgress = "in_progress"
	// ClaimStatusReview means the work is done and waiting for human review.
	// Appears in the REVIEW column.
	ClaimStatusReview = "review"
	// ClaimStatusValidation means the work passed review and is being validated
	// (testing, QA, staging, pre-release). Appears in the VALIDATION column.
	ClaimStatusValidation = "validation"
	// ClaimStatusBlocked means the ticket is stalled on an external dependency.
	// Appears in the BLOCKED column.
	ClaimStatusBlocked = "blocked"
	// ClaimStatusDone means the work is accepted and complete.
	// Appears in the DONE column until cleaned up by done_retention_days.
	ClaimStatusDone = "done"
)

// AllClaimStatuses is the ordered list of valid claim statuses.
var AllClaimStatuses = []string{
	ClaimStatusPlanned,
	ClaimStatusInProgress,
	ClaimStatusReview,
	ClaimStatusValidation,
	ClaimStatusBlocked,
	ClaimStatusDone,
}

// ValidTransitions defines the allowed status transitions.
// Each key maps to the set of statuses it can transition to.
var ValidTransitions = map[string][]string{
	ClaimStatusPlanned:    {ClaimStatusInProgress},
	ClaimStatusInProgress: {ClaimStatusReview, ClaimStatusBlocked},
	ClaimStatusReview:     {ClaimStatusValidation, ClaimStatusDone, ClaimStatusInProgress},
	ClaimStatusValidation: {ClaimStatusDone, ClaimStatusReview, ClaimStatusInProgress},
	ClaimStatusBlocked:    {ClaimStatusInProgress},
	ClaimStatusDone:       {ClaimStatusInProgress}, // reopen
}

// IsValidTransition checks whether transitioning from one status to another is allowed.
func IsValidTransition(from, to string) bool {
	allowed, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// Well-known claim label constants used by the hub and tracker sync.
const (
	// LabelAgentReviewed is applied when an AI agent completes a review pass.
	LabelAgentReviewed = "agent-reviewed"
	// LabelNeedsHumanReview is applied when the agent flags a need for human attention.
	LabelNeedsHumanReview = "needs-human-review"
	// LabelTrackerDone is the label pushed to the external tracker (GitLab/Jira)
	// when a claim reaches the "done" status (push direction, opt-in).
	LabelTrackerDone = "hub:done"
)

// Claim represents a ticket reservation by a team member.
type Claim struct {
	TicketID     string    `toml:"-"` // derived from filename
	Project      string    `toml:"-"` // derived from directory
	ClaimedBy    string    `toml:"claimed_by"`
	ClaimedAt    time.Time `toml:"claimed_at"`
	Worktree     string    `toml:"worktree,omitempty"`      // associated branch
	Status       string    `toml:"status"`                  // see ClaimStatus* constants
	LastActivity time.Time `toml:"last_activity,omitempty"` // last session/commit activity
	// Title is the issue title from the external tracker.
	// Populated and updated by the tracker sync engine.
	Title string `toml:"title,omitempty"`
	// Description is a truncated extract of the issue description from the
	// external tracker (max ~500 chars). Populated by the tracker sync engine.
	// The full description can be fetched on-demand from the tracker API.
	Description string `toml:"description,omitempty"`
	// TrackerStatus is the raw status name from the external tracker
	// (e.g. "In Progress", "Code Review"). Stored for auditability and
	// to detect status changes on the tracker side.
	TrackerStatus string `toml:"tracker_status,omitempty"`
	// Labels is an open-ended list of tags applied to the ticket.
	// Well-known values: see Label* constants above.
	// The tracker sync also mirrors GitLab/Jira labels here.
	Labels []string `toml:"labels,omitempty"`
	// ExternalIID is the issue number on the external tracker (GitLab IID or Jira key number).
	// Resolved from TicketID via TrackerConfig.TicketPatterns at sync time; stored here
	// to avoid re-parsing on every sync cycle.
	ExternalIID int `toml:"external_iid,omitempty"`
	// MRURL is the web URL of the merge request created for this claim.
	// Set by PublishReviewBatch (oh review --publish); read by oh review feedback
	// and TUI board detail for context. Empty for claims without an MR.
	MRURL string `toml:"mr_url,omitempty"`
	// Remote is the remote session working on the ticket (v5 phase 5),
	// published by the machine at sending time and by the runner as it goes.
	Remote *ClaimRemote `toml:"remote,omitempty"`
}

// IsValidStatus reports whether s is one of the known claim statuses.
func IsValidStatus(s string) bool {
	for _, v := range AllClaimStatuses {
		if v == s {
			return true
		}
	}
	return false
}

// IsValidBoardStatus reports whether s is a valid status for the given
// board statuses. Falls back to IsValidStatus if boardStatuses is empty.
// This allows custom column IDs to be accepted as valid claim statuses.
func IsValidBoardStatus(s string, boardStatuses []string) bool {
	if len(boardStatuses) == 0 {
		return IsValidStatus(s)
	}
	for _, v := range boardStatuses {
		if v == s {
			return true
		}
	}
	// Legacy compat: "planned" is always valid.
	return s == ClaimStatusPlanned
}

// IsValidBoardTransition reports whether the transition from→to is allowed.
// For built-in statuses, it enforces ValidTransitions.
// For custom statuses (not in AllClaimStatuses), all transitions are allowed.
func IsValidBoardTransition(from, to string) bool {
	// If both are built-in, use the strict transition rules.
	if IsValidStatus(from) && IsValidStatus(to) {
		return IsValidTransition(from, to)
	}
	// Custom statuses: allow any transition.
	return true
}

// ListClaims returns all active claims for a project.
// If project is empty, returns claims across all projects.
func (r *Repo) ListClaims(project string) ([]Claim, error) {
	if project != "" {
		if _, err := SafeName(project); err != nil {
			return nil, fmt.Errorf("invalid project name: %w", err)
		}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if project != "" {
		return r.listClaimsForProject(project)
	}
	// List all projects
	projectsDir := filepath.Join(r.path, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading projects dir: %w", err)
	}
	var all []Claim
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		claims, err := r.listClaimsForProject(e.Name())
		if err != nil {
			return nil, err
		}
		all = append(all, claims...)
	}
	return all, nil
}

// GetClaim retrieves a specific claim by project and ticket ID.
func (r *Repo) GetClaim(project, ticketID string) (*Claim, error) {
	if _, err := SafeName(project); err != nil {
		return nil, fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return nil, fmt.Errorf("invalid ticket ID: %w", err)
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getClaim(project, ticketID)
}

// getClaim is the internal (unlocked) implementation of GetClaim.
func (r *Repo) getClaim(project, ticketID string) (*Claim, error) {
	path := r.claimFilePath(project, ticketID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrClaimNotFound
		}
		return nil, fmt.Errorf("reading claim %s/%s: %w", project, ticketID, err)
	}
	var c Claim
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing claim %s/%s: %w", project, ticketID, err)
	}
	c.TicketID = ticketID
	c.Project = project
	return &c, nil
}

// CreateClaim reserves a ticket for a member.
// Returns ErrClaimExists if already claimed (warning — not blocking).
func (r *Repo) CreateClaim(ctx context.Context, c Claim) (*Claim, error) {
	if _, err := SafeName(c.Project); err != nil {
		return nil, fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(c.TicketID); err != nil {
		return nil, fmt.Errorf("invalid ticket ID: %w", err)
	}
	var existing *Claim
	err := r.withWriteLock(ctx, func(ctx context.Context) error {
		// Check if already claimed
		ex, err := r.getClaim(c.Project, c.TicketID)
		if err == nil {
			existing = ex
			return ErrClaimExists
		}

		// Ensure project claims directory
		dir := filepath.Join(r.path, "projects", c.Project, "claims")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating claims dir: %w", err)
		}

		// Write claim file
		if c.ClaimedAt.IsZero() {
			c.ClaimedAt = time.Now().UTC()
		}
		if c.Status == "" {
			c.Status = ClaimStatusInProgress
		}
		if len(r.boardStatuses) > 0 && !IsValidBoardStatus(c.Status, r.boardStatuses) {
			return fmt.Errorf("creating claim: %w: %q", ErrInvalidStatus, c.Status)
		}

		data, err := toml.Marshal(&c)
		if err != nil {
			return fmt.Errorf("marshaling claim: %w", err)
		}

		path := r.claimFilePath(c.Project, c.TicketID)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}

		// Commit and push
		relPath := r.claimRelPath(c.Project, c.TicketID)
		msg := fmt.Sprintf("claim: %s takes %s/%s", c.ClaimedBy, c.Project, c.TicketID)
		slog.Info("teamstate.claim.create", "project", c.Project, "ticket", c.TicketID, "member", c.ClaimedBy)
		return r.commitAndPush(ctx, msg, relPath)
	})
	if errors.Is(err, ErrClaimExists) {
		return existing, err
	}
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// CreateClaimLocal writes a claim file to disk without committing or pushing.
// The caller is responsible for calling CommitAndPush after creating all claims.
// Returns the repo-relative file path for use in the commit, or ErrClaimExists
// if the ticket is already claimed.
//
// This is designed for batch operations (e.g. autoplan) where multiple claims
// should be committed in a single git operation for performance.
//
// The caller MUST hold the repo write lock (via withWriteLock or mu.Lock).
func (r *Repo) CreateClaimLocal(c Claim) (relPath string, err error) {
	if _, err := SafeName(c.Project); err != nil {
		return "", fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(c.TicketID); err != nil {
		return "", fmt.Errorf("invalid ticket ID: %w", err)
	}

	// Check if already claimed
	if _, err := r.getClaim(c.Project, c.TicketID); err == nil {
		return "", ErrClaimExists
	}

	// Ensure project claims directory
	dir := filepath.Join(r.path, "projects", c.Project, "claims")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating claims dir: %w", err)
	}

	// Write claim file
	if c.ClaimedAt.IsZero() {
		c.ClaimedAt = time.Now().UTC()
	}
	if c.Status == "" {
		c.Status = ClaimStatusInProgress
	}
	if len(r.boardStatuses) > 0 && !IsValidBoardStatus(c.Status, r.boardStatuses) {
		return "", fmt.Errorf("creating claim: %w: %q", ErrInvalidStatus, c.Status)
	}

	data, err := toml.Marshal(&c)
	if err != nil {
		return "", fmt.Errorf("marshaling claim: %w", err)
	}

	absPath := r.claimFilePath(c.Project, c.TicketID)
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return "", fmt.Errorf("writing claim: %w", err)
	}

	slog.Info("teamstate.claim.create_local", "project", c.Project, "ticket", c.TicketID, "member", c.ClaimedBy)
	return r.claimRelPath(c.Project, c.TicketID), nil
}

// ReleaseClaim removes a claim (ticket is done or abandoned).
func (r *Repo) ReleaseClaim(ctx context.Context, project, ticketID string) error {
	if _, err := SafeName(project); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return fmt.Errorf("invalid ticket ID: %w", err)
	}
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		path := r.claimFilePath(project, ticketID)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return ErrClaimNotFound
		}

		if err := os.Remove(path); err != nil {
			return fmt.Errorf("removing claim file: %w", err)
		}

		relPath := r.claimRelPath(project, ticketID)
		msg := fmt.Sprintf("release: %s/%s", project, ticketID)
		slog.Info("teamstate.claim.release", "project", project, "ticket", ticketID)
		return r.commitAndPush(ctx, msg, relPath)
	})
}

// TransferClaim changes the owner of an existing claim.
func (r *Repo) TransferClaim(ctx context.Context, project, ticketID, newOwner string) error {
	if _, err := SafeName(project); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return fmt.Errorf("invalid ticket ID: %w", err)
	}
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return err
		}

		previousOwner := c.ClaimedBy
		c.ClaimedBy = newOwner

		data, marshalErr := toml.Marshal(c)
		if marshalErr != nil {
			return fmt.Errorf("marshaling claim: %w", marshalErr)
		}

		path := r.claimFilePath(project, ticketID)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}

		relPath := r.claimRelPath(project, ticketID)
		msg := fmt.Sprintf("transfer: %s/%s from %s to %s", project, ticketID, previousOwner, newOwner)
		slog.Info("teamstate.claim.transfer", "project", project, "ticket", ticketID, "from", previousOwner, "to", newOwner)
		return r.commitAndPush(ctx, msg, relPath)
	})
}

// ClaimPoolTicket assigns an unowned pool ticket (ClaimedBy="") to a member.
// Returns ErrClaimNotFound if no claim exists, ErrClaimAlreadyOwned if the
// claim already has an owner. ClaimedAt is updated to now.
// If workStatus is empty, defaults to ClaimStatusInProgress.
func (r *Repo) ClaimPoolTicket(ctx context.Context, project, ticketID, memberID, workStatus string) error {
	if _, err := SafeName(project); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return fmt.Errorf("invalid ticket ID: %w", err)
	}
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return err
		}
		if c.ClaimedBy != "" {
			return ErrClaimAlreadyOwned
		}

		c.ClaimedBy = memberID
		c.ClaimedAt = time.Now().UTC()
		if workStatus != "" {
			c.Status = workStatus
		} else {
			c.Status = ClaimStatusInProgress
		}
		c.LastActivity = time.Now().UTC()

		data, marshalErr := toml.Marshal(c)
		if marshalErr != nil {
			return fmt.Errorf("marshaling claim: %w", marshalErr)
		}

		path := r.claimFilePath(project, ticketID)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}

		relPath := r.claimRelPath(project, ticketID)
		msg := fmt.Sprintf("pool-claim: %s claims %s/%s", memberID, project, ticketID)
		slog.Info("teamstate.claim.pool_claim", "project", project, "ticket", ticketID, "member", memberID)
		return r.commitAndPush(ctx, msg, relPath)
	})
}

// UpdateClaimStatus changes the status of an existing claim and bumps LastActivity.
// Returns ErrInvalidStatus if newStatus is not a known value.
// Returns ErrClaimNotFound if the claim does not exist.
func (r *Repo) UpdateClaimStatus(ctx context.Context, project, ticketID, newStatus string) error {
	if _, err := SafeName(project); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return fmt.Errorf("invalid ticket ID: %w", err)
	}
	if !IsValidBoardStatus(newStatus, r.boardStatuses) {
		return fmt.Errorf("%w: %q", ErrInvalidStatus, newStatus)
	}

	return r.withWriteLock(ctx, func(ctx context.Context) error {
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return err
		}

		if !IsValidBoardTransition(c.Status, newStatus) {
			return fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidTransition, c.Status, newStatus)
		}

		c.Status = newStatus
		c.LastActivity = time.Now().UTC()

		data, err := toml.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshaling claim: %w", err)
		}

		path := r.claimFilePath(project, ticketID)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}

		relPath := r.claimRelPath(project, ticketID)
		msg := fmt.Sprintf("status: %s/%s → %s", project, ticketID, newStatus)
		slog.Info("teamstate.claim.status", "project", project, "ticket", ticketID, "status", newStatus)
		return r.commitAndPush(ctx, msg, relPath)
	})
}

// AddClaimLabel adds a label to an existing claim if not already present.
// Returns ErrClaimNotFound if the claim does not exist.
func (r *Repo) AddClaimLabel(ctx context.Context, project, ticketID, label string) error {
	if _, err := SafeName(project); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return fmt.Errorf("invalid ticket ID: %w", err)
	}
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return err
		}

		// Idempotent — skip if already present.
		for _, l := range c.Labels {
			if l == label {
				return nil
			}
		}

		c.Labels = append(c.Labels, label)
		c.LastActivity = time.Now().UTC()

		data, err := toml.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshaling claim: %w", err)
		}

		path := r.claimFilePath(project, ticketID)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}

		relPath := r.claimRelPath(project, ticketID)
		msg := fmt.Sprintf("label: %s/%s +%s", project, ticketID, label)
		return r.commitAndPush(ctx, msg, relPath)
	})
}

// RemoveClaimLabel removes a label from an existing claim.
// No-op if the label is not present. Returns ErrClaimNotFound if the claim does not exist.
func (r *Repo) RemoveClaimLabel(ctx context.Context, project, ticketID, label string) error {
	if _, err := SafeName(project); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return fmt.Errorf("invalid ticket ID: %w", err)
	}
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return err
		}

		filtered := c.Labels[:0]
		found := false
		for _, l := range c.Labels {
			if l == label {
				found = true
				continue
			}
			filtered = append(filtered, l)
		}
		if !found {
			return nil // idempotent
		}

		c.Labels = filtered
		c.LastActivity = time.Now().UTC()

		data, err := toml.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshaling claim: %w", err)
		}

		path := r.claimFilePath(project, ticketID)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}

		relPath := r.claimRelPath(project, ticketID)
		msg := fmt.Sprintf("label: %s/%s -%s", project, ticketID, label)
		return r.commitAndPush(ctx, msg, relPath)
	})
}

// UpdateClaimStatusFromTracker changes the status of an existing claim without
// enforcing the normal ValidTransitions rules. This is used by the tracker sync
// engine which may need to jump between arbitrary statuses (e.g. planned → review)
// based on the external tracker state.
// Returns ErrInvalidStatus if newStatus is not a known value.
// Returns ErrClaimNotFound if the claim does not exist.
// No-op if the claim already has the requested status.
func (r *Repo) UpdateClaimStatusFromTracker(ctx context.Context, project, ticketID, newStatus string) error {
	if _, err := SafeName(project); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return fmt.Errorf("invalid ticket ID: %w", err)
	}
	if !IsValidBoardStatus(newStatus, r.boardStatuses) {
		return fmt.Errorf("%w: %q", ErrInvalidStatus, newStatus)
	}

	return r.withWriteLock(ctx, func(ctx context.Context) error {
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return err
		}

		if c.Status == newStatus {
			return nil // no-op
		}

		c.Status = newStatus
		c.LastActivity = time.Now().UTC()

		data, err := toml.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshaling claim: %w", err)
		}

		path := r.claimFilePath(project, ticketID)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}

		relPath := r.claimRelPath(project, ticketID)
		msg := fmt.Sprintf("tracker-sync: %s/%s → %s", project, ticketID, newStatus)
		slog.Info("teamstate.claim.status_from_tracker", "project", project, "ticket", ticketID, "status", newStatus)
		return r.commitAndPush(ctx, msg, relPath)
	})
}

// UpdateClaimMetadata updates the title, description, and tracker status of an
// existing claim without changing the claim status. Used by the tracker sync
// engine to keep display data in sync with the external tracker.
// Returns ErrClaimNotFound if the claim does not exist.
// No-op if no fields have changed.
func (r *Repo) UpdateClaimMetadata(ctx context.Context, project, ticketID, title, description, trackerStatus string) error {
	if _, err := SafeName(project); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return fmt.Errorf("invalid ticket ID: %w", err)
	}

	return r.withWriteLock(ctx, func(ctx context.Context) error {
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return err
		}

		changed := false
		if title != "" && c.Title != title {
			c.Title = title
			changed = true
		}
		if description != "" && c.Description != description {
			c.Description = description
			changed = true
		}
		if trackerStatus != "" && c.TrackerStatus != trackerStatus {
			c.TrackerStatus = trackerStatus
			changed = true
		}

		if !changed {
			return nil
		}

		data, err := toml.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshaling claim: %w", err)
		}

		path := r.claimFilePath(project, ticketID)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}

		relPath := r.claimRelPath(project, ticketID)
		msg := fmt.Sprintf("metadata: %s/%s", project, ticketID)
		slog.Info("teamstate.claim.metadata", "project", project, "ticket", ticketID)
		return r.commitAndPush(ctx, msg, relPath)
	})
}

// SetClaimExternalIID stores the external tracker issue number on a claim.
// Used by the tracker sync engine to avoid re-parsing the ticket ID pattern on every cycle.
func (r *Repo) SetClaimExternalIID(ctx context.Context, project, ticketID string, iid int) error {
	if _, err := SafeName(project); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(ticketID); err != nil {
		return fmt.Errorf("invalid ticket ID: %w", err)
	}
	return r.withWriteLock(ctx, func(ctx context.Context) error {
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			return err
		}

		if c.ExternalIID == iid {
			return nil // already set, nothing to do
		}

		c.ExternalIID = iid

		data, err := toml.Marshal(c)
		if err != nil {
			return fmt.Errorf("marshaling claim: %w", err)
		}

		path := r.claimFilePath(project, ticketID)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("writing claim: %w", err)
		}

		relPath := r.claimRelPath(project, ticketID)
		msg := fmt.Sprintf("claim: %s/%s external_iid=%d", project, ticketID, iid)
		return r.commitAndPush(ctx, msg, relPath)
	})
}

// CleanupDoneClaims releases all claims in "done" status whose LastActivity is
// older than retentionDays. Returns the list of released claims.
// Called by the board timer and the oh team sync-tracker command.
// terminalStatuses specifies which statuses are considered "done" for cleanup.
// If empty, only ClaimStatusDone is used (backward compat).
func (r *Repo) CleanupDoneClaims(ctx context.Context, retentionDays int, terminalStatuses ...string) ([]Claim, error) {
	// Build terminal status set.
	termSet := make(map[string]bool, len(terminalStatuses)+1)
	if len(terminalStatuses) == 0 {
		termSet[ClaimStatusDone] = true
	} else {
		for _, s := range terminalStatuses {
			termSet[s] = true
		}
	}
	var released []Claim
	err := r.withWriteLock(ctx, func(ctx context.Context) error {
		// List all claims without acquiring RLock (we already hold the write lock).
		claims, err := r.listAllClaims()
		if err != nil {
			return err
		}

		cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)

		for _, c := range claims {
			if !termSet[c.Status] {
				continue
			}
			activity := c.LastActivity
			if activity.IsZero() {
				activity = c.ClaimedAt
			}
			if activity.Before(cutoff) {
				path := r.claimFilePath(c.Project, c.TicketID)
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					continue // best-effort, skip errors
				}
				released = append(released, c)
			}
		}

		if len(released) == 0 {
			return nil
		}

		// Batch commit all releases.
		msg := fmt.Sprintf("cleanup: released %d done claims (retention %d days)", len(released), retentionDays)
		return r.commitAndPush(ctx, msg, ".")
	})
	if err != nil {
		return nil, err
	}
	return released, nil
}

// listAllClaims is the internal (unlocked) implementation that lists claims
// across all projects in parallel. Must be called while holding the write lock.
func (r *Repo) listAllClaims() ([]Claim, error) {
	projectsDir := filepath.Join(r.path, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading projects dir: %w", err)
	}

	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(runtime.NumCPU())

	var mu sync.Mutex
	var all []Claim

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		project := e.Name()
		g.Go(func() error {
			claims, err := r.listClaimsForProject(project)
			if err != nil {
				return nil //nolint:nilerr // skip broken projects
			}
			mu.Lock()
			all = append(all, claims...)
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return all, nil
}

// claimRelPath returns the repo-relative path to a claim file.
func (r *Repo) claimRelPath(project, ticketID string) string {
	safe := strings.ReplaceAll(ticketID, "/", "_")
	return filepath.Join("projects", project, "claims", safe+".toml")
}

// claimFilePath returns the absolute path to a claim TOML file.
func (r *Repo) claimFilePath(project, ticketID string) string {
	return filepath.Join(r.path, r.claimRelPath(project, ticketID))
}

func (r *Repo) listClaimsForProject(project string) ([]Claim, error) {
	if _, err := SafeName(project); err != nil {
		return nil, fmt.Errorf("invalid project name: %w", err)
	}
	dir := filepath.Join(r.path, "projects", project, "claims")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading claims for %s: %w", project, err)
	}

	var claims []Claim
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		ticketID := strings.TrimSuffix(e.Name(), ".toml")
		c, err := r.getClaim(project, ticketID)
		if err != nil {
			slog.Warn("teamstate.claim.parse_failed", "project", project, "ticket", ticketID, "error", err)
			continue // skip malformed
		}
		claims = append(claims, *c)
	}
	return claims, nil
}

// ── Local helpers (no commit/push, caller MUST hold write lock) ─────────────

// updateClaimFieldsLocal reads a claim, applies fn, writes back to disk.
// Returns the repo-relative path of the modified file.
// Caller MUST hold the write lock via withWriteLock.
func (r *Repo) updateClaimFieldsLocal(project, ticketID string, fn func(c *Claim) error) (string, error) {
	c, err := r.getClaim(project, ticketID)
	if err != nil {
		return "", err
	}

	if err := fn(c); err != nil {
		return "", err
	}

	c.LastActivity = time.Now().UTC()

	data, err := toml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshaling claim: %w", err)
	}

	path := r.claimFilePath(project, ticketID)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("writing claim: %w", err)
	}

	return r.claimRelPath(project, ticketID), nil
}

// updateClaimStatusLocal changes a claim status on disk without commit/push.
// Validates the transition. Caller MUST hold the write lock.
func (r *Repo) updateClaimStatusLocal(project, ticketID, newStatus string) (string, error) {
	return r.updateClaimFieldsLocal(project, ticketID, func(c *Claim) error {
		if !IsValidBoardTransition(c.Status, newStatus) {
			return fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidTransition, c.Status, newStatus)
		}
		c.Status = newStatus
		return nil
	})
}

// addClaimLabelLocal adds a label to a claim on disk without commit/push.
// Idempotent: returns the relPath even if label was already present.
// Caller MUST hold the write lock.
func (r *Repo) addClaimLabelLocal(project, ticketID, label string) (string, error) {
	return r.updateClaimFieldsLocal(project, ticketID, func(c *Claim) error {
		for _, l := range c.Labels {
			if l == label {
				return nil // already present
			}
		}
		c.Labels = append(c.Labels, label)
		return nil
	})
}

// setClaimMRURLLocal sets the MR URL on a claim without commit/push.
// Caller MUST hold the write lock.
func (r *Repo) setClaimMRURLLocal(project, ticketID, mrURL string) (string, error) {
	return r.updateClaimFieldsLocal(project, ticketID, func(c *Claim) error {
		c.MRURL = mrURL
		return nil
	})
}

// ── PublishReviewBatch ──────────────────────────────────────────────────────

// PublishReviewParams holds the parameters for a PublishReviewBatch operation.
type PublishReviewParams struct {
	Project  string
	TicketID string
	Actor    string // member ID performing the publish
	Branch   string
	MRURL    string // merge request URL (may be empty if MR was not created)
	Label    string // label to add (e.g. LabelAgentReviewed); empty = skip
}

// PublishReviewBatch atomically transitions a claim to review status, sets the
// MR URL, appends a review.ready event, and adds a label — all in a single
// git commit+push. Returns the Event that was appended (for notification).
//
// This replaces the pattern of calling UpdateClaimStatus + AppendEvent +
// AddClaimLabel as 3+ separate locked/committed operations.
func (r *Repo) PublishReviewBatch(ctx context.Context, p PublishReviewParams) (Event, error) {
	if _, err := SafeName(p.Project); err != nil {
		return Event{}, fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(p.TicketID); err != nil {
		return Event{}, fmt.Errorf("invalid ticket ID: %w", err)
	}

	var event Event

	err := r.withWriteLock(ctx, func(ctx context.Context) error {
		// ── 1. Transition claim status: * → review ──
		_, err := r.updateClaimStatusLocal(p.Project, p.TicketID, ClaimStatusReview)
		if err != nil {
			// If already in review, skip silently (idempotent on double-publish).
			if errors.Is(err, ErrInvalidTransition) {
				c, getErr := r.getClaim(p.Project, p.TicketID)
				if getErr == nil && c.Status == ClaimStatusReview {
					slog.Debug("teamstate.publish.already_review", "project", p.Project, "ticket", p.TicketID)
				} else {
					return fmt.Errorf("status transition: %w", err)
				}
			} else {
				return fmt.Errorf("status transition: %w", err)
			}
		}

		// ── 2. Set MR URL on claim ──
		if p.MRURL != "" {
			if _, err := r.setClaimMRURLLocal(p.Project, p.TicketID, p.MRURL); err != nil {
				slog.Warn("teamstate.publish.mr_url_failed", "ticket", p.TicketID, "error", err)
			}
		}

		// ── 3. Add label (non-fatal) ──
		if p.Label != "" {
			if _, err := r.addClaimLabelLocal(p.Project, p.TicketID, p.Label); err != nil {
				slog.Warn("teamstate.publish.label_failed", "ticket", p.TicketID, "label", p.Label, "error", err)
			}
		}

		// ── 4. Append review.ready event ──
		event = Event{
			Timestamp: time.Now().UTC(),
			Actor:     p.Actor,
			Type:      EventReviewReady,
			Project:   p.Project,
			Ticket:    p.TicketID,
			Data:      map[string]interface{}{"branch": p.Branch},
		}
		if p.MRURL != "" {
			event.Data["mr_url"] = p.MRURL
		}
		eventRelPath, err := r.appendEventLocal(event)
		if err != nil {
			return fmt.Errorf("appending event: %w", err)
		}

		// ── 5. Single commit+push ──
		claimRelPath := r.claimRelPath(p.Project, p.TicketID)
		msg := fmt.Sprintf("publish-review: %s/%s → review", p.Project, p.TicketID)
		slog.Info("teamstate.publish.review", "project", p.Project, "ticket", p.TicketID, "actor", p.Actor)
		return r.commitAndPush(ctx, msg, claimRelPath, eventRelPath)
	})

	return event, err
}
