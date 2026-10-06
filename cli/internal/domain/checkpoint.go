package domain

import (
	"context"
	"time"
)

// DecisionCircuit is the circuit breaker alert (✗): too many consecutive
// subagent calls without the user; delegation is denied until it is dismissed.
const DecisionCircuit DecisionKind = "circuit"

// Checkpoint timeline entries.
const (
	CheckpointPassed   = "passed"   // validated (or let through by the mode)
	CheckpointRefused  = "refused"  // the user asked for another way
	CheckpointWaiting  = "waiting"  // waiting for a decision
	CheckpointDelegate = "delegate" // an agent of the workflow ran
	CheckpointBreaker  = "breaker"  // circuit breaker tripped
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
	// Timeline is the history shown in the session detail (oldest first, capped).
	Timeline []CheckpointEvent `json:"timeline,omitempty"`
}

// CheckpointApproval is a validation waiting for the agent's call to run.
type CheckpointApproval struct {
	By      string `json:"by"`
	Message string `json:"message,omitempty"`
}

// CheckpointEvent is an entry of the checkpoint timeline.
type CheckpointEvent struct {
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"` // passed | refused | waiting | delegate | breaker
	ID      string    `json:"id"`   // checkpoint or agent
	By      string    `json:"by,omitempty"`
	Message string    `json:"message,omitempty"`
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
