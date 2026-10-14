package domain

import (
	"context"
	"time"
)

// Fan-out of a session (R6, E3, migration v46, experimental): a parent
// session whose tickets are each implemented by a child session in its own
// worktree; oh merges each child branch into an integration branch, asks a
// merger session to resolve a conflict it cannot, and merges the
// integration branch once the merge checkpoint of the parent is validated.
// The record is written by oh only (the fan-out driver and the merge
// checkpoint), never by an agent; SessionStore.Update leaves it as is.

// FanoutRole is the role of a session in a fan-out.
type FanoutRole string

const (
	RoleParent FanoutRole = "parent"
	RoleChild  FanoutRole = "child"
	RoleMerger FanoutRole = "merger"
)

// FanoutStatus is the progress of a fan-out (parent).
type FanoutStatus string

const (
	// FanoutWaiting: the parent has not passed the checkpoints before the
	// children yet (cp-plan).
	FanoutWaiting FanoutStatus = "waiting"
	// FanoutRunning: children are launched and merged.
	FanoutRunning FanoutStatus = "running"
	// FanoutReady: every child is done; the merge checkpoint may pass.
	FanoutReady FanoutStatus = "ready"
	// FanoutMerged: the merge checkpoint passed, oh merged the integration
	// branch into the branch of the parent.
	FanoutMerged FanoutStatus = "merged"
	// FanoutFailed: the fan-out stopped (integration branch moved by
	// someone else than oh, git error); FanoutStopped: stopped by the user.
	FanoutFailed  FanoutStatus = "failed"
	FanoutStopped FanoutStatus = "stopped"
)

// Terminal reports whether no child runs any more.
func (s FanoutStatus) Terminal() bool {
	return s == FanoutReady || s == FanoutMerged || s == FanoutFailed || s == FanoutStopped
}

// FanoutChildState is the state of a child of a fan-out.
type FanoutChildState string

const (
	ChildPending FanoutChildState = "pending"
	ChildRunning FanoutChildState = "running"
	// ChildMerged: its branch is merged into the integration branch.
	ChildMerged FanoutChildState = "merged"
	// ChildConflict: its branch could not be merged (conflict unresolved).
	ChildConflict FanoutChildState = "conflict"
	// ChildFailed: no commit, session failed or stopped.
	ChildFailed FanoutChildState = "failed"
	// ChildBudget: not started, or stopped, by the global budget (F-04).
	ChildBudget FanoutChildState = "budget"
)

// Done reports whether the child will not change any more.
func (s FanoutChildState) Done() bool { return s != ChildPending && s != ChildRunning }

// SessionFanout is the fan-out record of a session.
type SessionFanout struct {
	Role FanoutRole `json:"role"`

	// Child and merger.
	Parent string `json:"parent,omitempty"`
	Ticket string `json:"ticket,omitempty"`
	// Branch is the branch of a child, or the branch a merger merges.
	Branch string `json:"branch,omitempty"`

	// Parent.
	// Experimental: the fan-out was enabled in the configuration of oh
	// when the parent was launched ([fanout] experimental).
	Experimental bool         `json:"experimental,omitempty"`
	Status       FanoutStatus `json:"status,omitempty"`
	Parallel     int          `json:"parallel,omitempty"`
	// ChildInputs are inputs of the parent given to each child (method).
	ChildInputs map[string]string  `json:"child_inputs,omitempty"`
	Integration *FanoutIntegration `json:"integration,omitempty"`
	Children    []FanoutChild      `json:"children,omitempty"`
	// BudgetUSD is the global budget of the parent and its children (F-04,
	// 0 = none); SpentUSD what they spent at the last check.
	BudgetUSD float64       `json:"budget_usd,omitempty"`
	SpentUSD  float64       `json:"spent_usd,omitempty"`
	Events    []FanoutEvent `json:"events,omitempty"`
	// Merged is the commit of the branch of the parent after the merge of
	// the integration branch by oh.
	Merged string `json:"merged,omitempty"`
}

// FanoutIntegration is the integration branch of a fan-out and its
// worktree. Head is the last commit oh made or saw on it: another value
// means someone else moved it (refused).
type FanoutIntegration struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Base   string `json:"base"`
	Head   string `json:"head"`
}

// FanoutChild is a child of a fan-out.
type FanoutChild struct {
	Ticket  string           `json:"ticket"`
	Session string           `json:"session,omitempty"`
	Branch  string           `json:"branch,omitempty"`
	Path    string           `json:"path,omitempty"`
	State   FanoutChildState `json:"state"`
	// Head is the commit of the child merged; Merge the merge commit on the
	// integration branch.
	Head  string `json:"head,omitempty"`
	Merge string `json:"merge,omitempty"`
	// Merger is the session that resolved (or tried to) its conflict.
	Merger    string   `json:"merger,omitempty"`
	Conflicts []string `json:"conflicts,omitempty"`
	CostUSD   float64  `json:"cost_usd,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// FanoutEvent is an entry of the trace of a fan-out.
type FanoutEvent struct {
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"`
	Ticket  string    `json:"ticket,omitempty"`
	Session string    `json:"session,omitempty"`
	Message string    `json:"message,omitempty"`
}

// Kinds of fan-out events.
const (
	FanoutEventStarted    = "started"    // integration branch created
	FanoutEventChild      = "child"      // child session launched
	FanoutEventChildDone  = "child_done" // child session finished
	FanoutEventMerged     = "merged"     // child merged by oh
	FanoutEventConflict   = "conflict"   // merge conflict
	FanoutEventMerger     = "merger"     // merger session launched
	FanoutEventResolved   = "resolved"   // conflict resolved by the merger
	FanoutEventBudget     = "budget"     // global budget reached
	FanoutEventReady      = "ready"      // every child done
	FanoutEventTampered   = "tampered"   // integration branch moved outside oh
	FanoutEventIntegrated = "integrated" // integration branch merged by oh (merge checkpoint)
	FanoutEventClosed     = "closed"     // ticket closed by oh
	FanoutEventFailed     = "failed"     // fan-out stopped on an error
	FanoutEventDelegated  = "delegated"  // a delegated checkpoint passed without pause in a child (CP-R3)
	FanoutEventResumed    = "resumed"    // parent told the result
)

// fanoutEventsMax caps the stored events.
const fanoutEventsMax = 300

// AddEvent appends an event to the trace of the fan-out.
func (f *SessionFanout) AddEvent(kind, ticket, session, msg string) {
	f.Events = append(f.Events, FanoutEvent{At: time.Now().UTC(), Kind: kind, Ticket: ticket, Session: session, Message: msg})
	if n := len(f.Events); n > fanoutEventsMax {
		f.Events = f.Events[n-fanoutEventsMax:]
	}
}

// Child returns the child of a ticket.
func (f *SessionFanout) Child(ticket string) (*FanoutChild, bool) {
	if f == nil {
		return nil, false
	}
	for i := range f.Children {
		if f.Children[i].Ticket == ticket {
			return &f.Children[i], true
		}
	}
	return nil, false
}

// HasChild reports whether session is a child (or a merger) of the fan-out.
func (f *SessionFanout) HasChild(session string) bool {
	if f == nil || session == "" {
		return false
	}
	for _, c := range f.Children {
		if c.Session == session || c.Merger == session {
			return true
		}
	}
	return false
}

// SessionFanoutStore writes the fan-out record of a session (only oh
// does; SessionStore.Update never changes it).
type SessionFanoutStore interface {
	SetSessionFanout(ctx context.Context, id string, f *SessionFanout) error
}
