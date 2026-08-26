// Package tracker provides an abstraction layer for syncing team-state claims
// with external issue trackers (GitLab, Jira).
//
// Architecture:
//   - Tracker interface: the minimal read/write surface needed for sync.
//   - IssueState: the normalised representation of an issue from any tracker.
//   - New(cfg, token, baseURL): factory that returns the right implementation.
//
// The connection credentials (token, base URL) are NOT stored in the team-state
// config — they are resolved at runtime from the hub's MCP configuration
// ([mcp.gitlab] / [mcp.jira] in hub.toml) via the App.Secrets store.
package tracker

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Type identifies the tracker backend.
type Type string

const (
	TypeGitLab Type = "gitlab"
	TypeJira   Type = "jira"
)

// IssueState is the normalised representation of an issue on any tracker.
// Only the fields needed for sync are included; tracker-specific data stays
// inside the implementation.
type IssueState struct {
	// IID is the tracker-internal issue number (GitLab IID or Jira issue key number).
	IID int
	// Key is the human-readable issue key (e.g. "SRU-42" for Jira; empty for GitLab).
	Key string
	// State is "open" or "closed" (normalised from tracker-specific values).
	State string
	// Labels is the list of labels currently applied on the tracker.
	Labels []string
	// Assignees is the list of tracker usernames assigned to this issue.
	Assignees []string
	// Title is the issue title (for display / logging only).
	Title string
	// UpdatedAt is when the issue was last modified on the tracker.
	UpdatedAt time.Time
}

// IsClosed reports whether the issue is in a terminal/closed state.
func (s IssueState) IsClosed() bool { return s.State == "closed" }

// ListOpts filters for ListAssignedIssues.
type ListOpts struct {
	// AssigneeUsername filters by tracker username.
	AssigneeUsername string
	// UpdatedAfter filters to issues updated after this time (zero = no filter).
	UpdatedAfter time.Time
	// MaxResults caps the number of returned issues (0 = use tracker default).
	MaxResults int
}

// Tracker is the minimal interface a tracker backend must satisfy.
// Implementations must be safe for concurrent use.
type Tracker interface {
	// FetchIssue returns the current state of a single issue.
	FetchIssue(ctx context.Context, projectID string, iid int) (*IssueState, error)

	// ListAssignedIssues returns open issues assigned to the given username.
	ListAssignedIssues(ctx context.Context, projectID string, opts ListOpts) ([]IssueState, error)

	// AddLabels appends labels to an issue. Requires write_enabled in the MCP config.
	AddLabels(ctx context.Context, projectID string, iid int, labels []string) error

	// RemoveLabels removes labels from an issue. Requires write_enabled.
	RemoveLabels(ctx context.Context, projectID string, iid int, labels []string) error

	// CreateIssue creates a new issue on the tracker. Requires write_enabled.
	CreateIssue(ctx context.Context, opts CreateIssueOpts) (*CreatedIssue, error)

	// TestConnection verifies that the configured token is valid and returns the
	// authenticated username on the tracker. Used by the setup wizard and the
	// TUI config view to surface connectivity issues early.
	TestConnection(ctx context.Context) (username string, err error)

	// TestProject verifies that a specific project is accessible with the current
	// token. Returns the project name/path for display, or an error if not found
	// or not accessible (404, 403).
	TestProject(ctx context.Context, projectID string) (projectName string, err error)
}

// CreateIssueOpts holds the parameters for creating a new issue.
type CreateIssueOpts struct {
	ProjectID   string // project identifier (e.g., "namespace/project" for GitLab, "KEY" for Jira)
	Title       string // issue summary/title
	Description string // body/description (optional)
	IssueType   string // e.g., "Task", "Bug", "Story" (Jira); ignored for GitLab
	Labels      []string // labels to apply (optional)
	AssignTo    string // username to assign (optional)
}

// CreatedIssue holds the result of a successful issue creation.
type CreatedIssue struct {
	ID  int    // issue IID/number
	URL string // web URL to the created issue
}

// Config holds the connection parameters for a Tracker implementation.
// The token field is intentionally not stored in team-state config.toml —
// it is resolved at runtime from the hub keychain / GITLAB_TOKEN env var.
type Config struct {
	Type         Type
	BaseURL      string // e.g. "https://gitlab.example.com"
	Token        string // personal access token or API token
	WriteEnabled bool   // allows AddLabels / RemoveLabels
}

// ErrWriteDisabled is returned by AddLabels/RemoveLabels when WriteEnabled is false.
var ErrWriteDisabled = errors.New("tracker write operations are disabled (set write_enabled in [mcp.gitlab] or [mcp.jira])")

// ErrIssueNotFound is returned when the requested issue does not exist.
var ErrIssueNotFound = errors.New("issue not found")

// ErrRateLimited is returned when the tracker API returns HTTP 429.
type ErrRateLimited struct {
	RetryAfter time.Duration
}

func (e *ErrRateLimited) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("tracker rate limited — retry after %s", e.RetryAfter)
	}
	return "tracker rate limited"
}

// IsRateLimited reports whether err is an ErrRateLimited.
func IsRateLimited(err error) bool {
	var e *ErrRateLimited
	return errors.As(err, &e)
}

// ErrTokenInvalid is returned on HTTP 401 / 403 from the tracker.
var ErrTokenInvalid = errors.New("tracker authentication failed — token invalide ou expiré. Reconfigurer : oh secrets set <token_key> <new-token>")

// New returns a Tracker implementation for the given config.
func New(cfg Config) (Tracker, error) {
	switch cfg.Type {
	case TypeGitLab:
		return newGitLab(cfg), nil
	case TypeJira:
		return newJira(cfg), nil
	default:
		return nil, fmt.Errorf("unknown tracker type %q (supported: gitlab, jira)", cfg.Type)
	}
}
