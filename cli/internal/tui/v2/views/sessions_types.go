package views

import (
	"context"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Sessions view models (P3-T16): display-ready data produced by the wiring
// layer (cmd) from the SessionService. The views package never reads stores.
// ─────────────────────────────────────────────────────────────────────────────

// Decision kinds understood by the Sessions view.
const (
	DecisionKindPermission = "permission"
	DecisionKindQuestion   = "question"
	DecisionKindCheckpoint = "checkpoint"
	DecisionKindBudget     = "budget"
	DecisionKindError      = "error"
)

// SessionDecisionField is one field of an agent question.
type SessionDecisionField struct {
	Key         string
	Title       string
	Description string
	Type        string // string | number | integer | boolean | multiselect | external
	Options     []SelectOption
	Custom      bool // free answer allowed besides the options
	Required    bool
}

// SessionDecision is a decision waiting in a session.
type SessionDecision struct {
	ID        string
	SessionID string
	Kind      string
	Icon      string
	Summary   string
	Message   string
	Created   time.Time
	Fields    []SessionDecisionField
}

// SessionRow is one session of the Sessions view.
type SessionRow struct {
	ID         string
	ProjectID  string
	Project    string
	Title      string
	Workflow   string
	Agent      string
	Mode       string
	Runtime    string
	Location   string
	Bundle     string
	State      string // run state (active, waiting, idle, sleeping, stopped…)
	StateLabel string
	StateIcon  string
	Cost       float64
	Started    time.Time
	Changed    time.Time
	Finished   bool
	Decisions  []SessionDecision
}

// SessionFeedLine is one line of a session live feed.
type SessionFeedLine struct {
	Time time.Time
	Text string // empty for cost-only updates
	Cost float64
}

// SessionsBackend is what the Sessions view needs from the SessionService
// and the RunService. Calls may block: the view runs them off the event loop.
type SessionsBackend interface {
	List(ctx context.Context, projectID string, all bool) ([]SessionRow, error)
	// Changes signals that sessions or decisions changed (closed with ctx).
	Changes(ctx context.Context) <-chan struct{}
	Follow(ctx context.Context, sessionID string) (<-chan SessionFeedLine, error)
	// Decide answers a decision: choice (permission once|always|reject,
	// dismiss, checkpoint choice), message, textual answers by field key.
	Decide(ctx context.Context, decisionID, choice, message string, answers map[string]string) error
	Send(ctx context.Context, sessionID, text string) error
	Interrupt(ctx context.Context, sessionID string) error
	SwitchModel(ctx context.Context, sessionID, model string) error
	Stop(ctx context.Context, sessionID string) error
	Resume(ctx context.Context, sessionID string) error
	// Attach opens the session client (new terminal tab, tmux…, or the
	// current terminal); how is "" for the configured preference.
	Attach(sessionID, how string)
	OpenBrowser(ctx context.Context, sessionID string) (string, error)
	MRDescription(ctx context.Context, sessionID string) (string, error)
}

// SessionsSummary is the cached overview shown on the home, project and team
// landings and in the mode bar badge.
type SessionsSummary struct {
	Running   int
	Decisions int
	Lines     []SessionLine // a few sessions, those waiting first
}

// SessionLine is one session of a summary.
type SessionLine struct {
	ID    string
	Icon  string
	Label string
	Desc  string
}
