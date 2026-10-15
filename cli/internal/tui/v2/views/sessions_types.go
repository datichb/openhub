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
	DecisionKindCircuit    = "circuit" // circuit breaker (checkpoint track)
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
	// Approver lists the approvers a checkpoint names (CP-R9, "" = none).
	Approver string
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
	// Next is the workflow suggested to chain with (« Enchaîner avec… »).
	Next string
	// Timeline is the checkpoint timeline, rendered ("" without a workflow).
	Timeline string
	// Remote is the remote status ("" = not remote; RemoteStatus*) and
	// RemoteDetail its line (pipeline, MR, journal).
	Remote       string
	RemoteDetail string
	// Outputs are the outputs oh recorded for the session (session.Outputs,
	// `record:`), sorted by name.
	Outputs []SessionOutput
	// FanoutRole is the role of the session in a fan-out (parent, child,
	// merger, fixer; "" = none), FanoutParent the parent of a child, and
	// FanoutLine the state of the fan-out of a parent (detail).
	FanoutRole   string
	FanoutParent string
	FanoutLine   string
}

// Fan-out roles of SessionRow.FanoutRole.
const (
	FanoutRoleParent = "parent"
	FanoutRoleChild  = "child"
	FanoutRoleMerger = "merger"
	FanoutRoleFixer  = "fixer"
)

// InFanout reports whether the session is part of a fan-out.
func (r SessionRow) InFanout() bool { return r.FanoutRole != "" }

// SessionOutput is an output recorded by oh for a session.
type SessionOutput struct {
	Name  string
	Value string
}

// SessionNextStep is a `next:` step of the workflow of a session whose
// condition holds (R5): what oh proposes after it, never launched alone.
type SessionNextStep struct {
	Workflow string
	Label    string
	Inputs   string // "branch=feat/auth tickets=oh-12"
}

// SessionNexter is implemented by backends that read the `next:` steps of
// a session (detail).
type SessionNexter interface {
	NextSteps(ctx context.Context, sessionID string) ([]SessionNextStep, error)
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

// SessionChainer is implemented by backends able to suggest the workflows to
// chain after a session (« Enchaîner avec… », O7, P1-T27).
type SessionChainer interface {
	// ChainOptions lists the workflows to chain after a session.
	ChainOptions(ctx context.Context, sessionID string) ([]SelectOption, error)
	// Chain opens the launch form of a suggestion.
	Chain(sessionID, workflowID string)
}

// SessionDeleter is implemented by backends able to delete a session for
// good (D, `oh session delete`).
type SessionDeleter interface {
	// PlanDelete lists what deleting the session loses (confirmation).
	PlanDelete(ctx context.Context, sessionID string) (DeleteSummary, error)
	// DeleteSession deletes it; discardBase also discards the uncommitted
	// changes of the project directory it worked in. It returns the summary
	// shown in a toast.
	DeleteSession(ctx context.Context, sessionID string, discardBase bool) (string, error)
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
