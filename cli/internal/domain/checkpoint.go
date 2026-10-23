package domain

import (
	"context"
	"time"
)

// DecisionCircuit is the circuit breaker alert (⊘): too many consecutive
// subagent calls without the user; delegation is denied until it is dismissed.
const DecisionCircuit DecisionKind = "circuit"

// Checkpoint timeline entries.
const (
	CheckpointPassed   = "passed"   // validated (or let through by the mode)
	CheckpointRefused  = "refused"  // the user asked for another way
	CheckpointWaiting  = "waiting"  // waiting for a decision
	CheckpointDelegate = "delegate" // an agent of the workflow ran
	CheckpointBreaker  = "breaker"  // circuit breaker tripped
	CheckpointImitated = "imitated" // a question imitated a checkpoint (closed by oh)
	CheckpointReport   = "report"   // a report was stored (workflow_report)
	CheckpointWiki     = "wiki"     // oh applied the wiki proposals accepted on the card (LD-J)
	CheckpointTasks    = "tasks"    // oh recorded the breakdown checked at the checkpoint (sweep-fanout)
)

// CheckpointState is the workflow state of a session (sessions.checkpoint_state,
// migration v35), kept by the CheckpointService.
type CheckpointState struct {
	// Passed maps the checkpoints passed to when they were.
	Passed map[string]time.Time `json:"passed,omitempty"`
	// Waiting is the checkpoint waiting for a decision ("" = none).
	Waiting string `json:"waiting,omitempty"`
	// Approved holds the validations not yet seen by the agent: checkpoint
	// → who validated it and the instruction given with it (returned by
	// workflow_checkpoint when the call runs).
	Approved map[string]CheckpointApproval `json:"approved,omitempty"`
	// Ran lists the workflow agents that completed at least one delegation.
	Ran []string `json:"ran,omitempty"`
	// Consecutive counts the subagent calls since the last user intervention.
	Consecutive int `json:"consecutive,omitempty"`
	// Breaker is set while the circuit breaker holds delegation.
	Breaker bool `json:"breaker,omitempty"`
	// Unlocks are the operations unlocked by a checkpoint passed (`unlocks:`),
	// by operation; removed when the window closes again.
	Unlocks map[string]CheckpointUnlock `json:"unlocks,omitempty"`
	// Claimed and Closed are the Beads tickets the session took and closed
	// through the oh gateway.
	Claimed []string `json:"claimed,omitempty"`
	Closed  []string `json:"closed,omitempty"`
	// Finished is when oh saw the end of the workflow.
	Finished *time.Time `json:"finished,omitempty"`
	// Timeline is the history shown in the session detail (oldest first, capped).
	Timeline []CheckpointEvent `json:"timeline,omitempty"`

	// R3 (checkpoint_verify.go): verification run by oh, evidence, shell
	// calls observed and developer → reviewer rounds.
	Verify   map[string]CheckpointVerify     `json:"verify,omitempty"`
	Evidence map[string][]CheckpointEvidence `json:"evidence,omitempty"`
	Shell    []ShellCall                     `json:"shell,omitempty"`
	Rounds   map[string]int                  `json:"rounds,omitempty"`
	// LastDelegate is the last agent delegated to (fix rounds).
	LastDelegate string `json:"last_delegate,omitempty"`
	// Unresolved is the ticket whose fix rounds reached the cap ("" = none):
	// the developer is locked and the commit checkpoint shows it.
	Unresolved string `json:"unresolved,omitempty"`
	// RemoteVerify are the verifications run by oh in the remote job of
	// the session (R3-remote, imported at fetch): shown on the card, never
	// taken for a verification of the work on this machine.
	RemoteVerify map[string]CheckpointVerify `json:"remote_verify,omitempty"`

	// R7: Bases are the commits of the location when a checkpoint that
	// starts a count of the changes (CheckpointDef.Base: cp-seams) was last
	// passed, by checkpoint.
	Bases map[string]string `json:"bases,omitempty"`

	// R8: Committed are the commits that closed the commit window of a
	// checkpoint, by checkpoint: a checkpoint passed again on that same
	// commit (nothing new to commit) does not hold its tag back.
	Committed map[string]string `json:"committed,omitempty"`

	// sweep-fanout: the breakdown read by oh at the checkpoint with
	// `tasks:`. Shown holds the one shown on the card of the checkpoint
	// waiting (the call must give the same to pass), Tasks the one
	// recorded once it passed (the children of a fan-out `source: tasks`).
	ShownTasks []PlanTask `json:"shown_tasks,omitempty"`
	Tasks      []PlanTask `json:"tasks,omitempty"`
}

// PlanTask is a task of a breakdown read and checked by oh (sweep-fanout):
// never an agent's word, the record oh made of it.
type PlanTask struct {
	// ID is given by oh (t1, t2…), in the order of the breakdown.
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	// Files is the file scope of the task: paths, folders or patterns (*,
	// **) of the repository, or new files in existing folders.
	Files  []string `json:"files"`
	Domain string   `json:"domain,omitempty"`
	// Parallel: outside a fan-out, the task may run at the same time as
	// the other parallel ones (their scopes are disjoint).
	Parallel bool `json:"parallel,omitempty"`
}

// CheckpointUnlock is an operation unlocked by a checkpoint.
type CheckpointUnlock struct {
	Checkpoint string    `json:"checkpoint"`
	At         time.Time `json:"at"`
	// Head is the commit of the session location when the checkpoint was
	// passed ("" = unknown): a ticket is closed only once it moved.
	Head string `json:"head,omitempty"`
	// Tags is the fingerprint of the local tags when a tag window opened
	// ("" = unknown): the window closes once they change (the first tag).
	Tags string `json:"tags,omitempty"`
}

// CheckpointApproval is a validation waiting for the agent's call to run.
type CheckpointApproval struct {
	By      string `json:"by"`
	Message string `json:"message,omitempty"`
	// Override: the user validated despite a failed verification (traced).
	Override bool `json:"override,omitempty"`

	// approver-1 (CP-R9): Identity is the person who validated (git
	// user.name <user.email> where the session works, ADR-052; "" for a
	// validation by the mode or a delegation). ApproverMismatch: the
	// checkpoint names approvers and this person is none of them (warned
	// and traced, not refused on this machine).
	Identity         string `json:"identity,omitempty"`
	ApproverMismatch bool   `json:"approver_mismatch,omitempty"`
}

// CheckpointEvent is an entry of the checkpoint timeline.
type CheckpointEvent struct {
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"` // passed | refused | waiting | delegate | breaker
	ID      string    `json:"id"`   // checkpoint or agent
	By      string    `json:"by,omitempty"`
	Message string    `json:"message,omitempty"`

	// approver-1 (CP-R9): who validated or refused (CheckpointApproval),
	// the approvers the checkpoint names and whether the person is none of
	// them.
	Identity         string   `json:"identity,omitempty"`
	Approver         []string `json:"approver,omitempty"`
	ApproverMismatch bool     `json:"approver_mismatch,omitempty"`
}

// HasPassed reports whether the checkpoint was passed.
func (s *CheckpointState) HasPassed(id string) bool {
	if s == nil {
		return false
	}
	_, ok := s.Passed[id]
	return ok
}

// HasRun reports whether the agent completed a delegation.
func (s *CheckpointState) HasRun(agent string) bool {
	if s == nil {
		return false
	}
	for _, a := range s.Ran {
		if a == agent {
			return true
		}
	}
	return false
}

// CheckpointStore persists the checkpoint state of sessions.
type CheckpointStore interface {
	// GetCheckpointState returns the state of a session (zero state when unset).
	GetCheckpointState(ctx context.Context, sessionID string) (CheckpointState, error)
	// UpdateCheckpointState applies fn to the state atomically (concurrent
	// writers — daemon, CLI, TUI — never lose an update) and returns the
	// new state. fn returning an error leaves the state unchanged.
	UpdateCheckpointState(ctx context.Context, sessionID string, fn func(*CheckpointState) error) (CheckpointState, error)
}

// SessionOutputStore records the typed outputs declared by a session
// (sessions.outputs, by output id) without rewriting the session row.
type SessionOutputStore interface {
	SetSessionOutput(ctx context.Context, sessionID, key string, value any) error
}

// CheckpointStorage is the database side of the checkpoints: their state and
// the typed outputs recorded next to it (one store serves both).
type CheckpointStorage interface {
	CheckpointStore
	SessionOutputStore
}
