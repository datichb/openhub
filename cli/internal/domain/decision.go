package domain

import (
	"context"
	"time"
)

// DecisionKind classifies a decision waiting for a human (I3). The set is
// open: new kinds (e.g. checkpoint) need no schema change.
type DecisionKind string

const (
	DecisionPermission DecisionKind = "permission" // !
	DecisionQuestion   DecisionKind = "question"   // ?
	DecisionCheckpoint DecisionKind = "checkpoint" // ⏸ (fed by the CheckpointService)
	DecisionBudget     DecisionKind = "budget"     // $
	DecisionError      DecisionKind = "error"      // ✗
)

// DecisionResolver tells who resolved a decision.
type DecisionResolver string

const (
	ResolvedByOh     DecisionResolver = "oh"     // oh (TUI or CLI)
	ResolvedByTool   DecisionResolver = "tool"   // the tool UI, web or mobile client
	ResolvedByPolicy DecisionResolver = "policy" // automatic policy (CI responder…)
	ResolvedByGone   DecisionResolver = "gone"   // the session ended or its server stopped
)

// DecisionField is a typed field of a question.
type DecisionField struct {
	Key         string           `json:"key"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	Type        string           `json:"type,omitempty"`
	Options     []DecisionOption `json:"options,omitempty"`
	Custom      bool             `json:"custom,omitempty"` // free answer allowed
	Required    bool             `json:"required,omitempty"`
}

// DecisionOption is one choice of a DecisionField.
type DecisionOption struct {
	Value       string `json:"value"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

// DecisionPayload is the content of a decision. Kinds use the fields they
// need; Data carries kind-specific extensions (checkpoint id, summary…).
type DecisionPayload struct {
	Action    string          `json:"action,omitempty"`    // permission action (shell, edit, …)
	Resources []string        `json:"resources,omitempty"` // permission resources
	Title     string          `json:"title,omitempty"`     // question title / checkpoint label
	Fields    []DecisionField `json:"fields,omitempty"`    // question fields
	Message   string          `json:"message,omitempty"`   // error / budget detail
	Data      map[string]any  `json:"data,omitempty"`
}

// DecisionResolution records how a decision was answered.
type DecisionResolution struct {
	Decision string         `json:"decision,omitempty"` // permission: once | always | reject; checkpoint choice; error: dismiss
	Message  string         `json:"message,omitempty"`
	Answer   map[string]any `json:"answer,omitempty"` // question answers by field key
}

// Decision is a pending (or resolved) human decision of a session.
type Decision struct {
	ID         string // stable: DecisionID(kind, session, ref)
	SessionID  string
	GroupKey   string
	Kind       DecisionKind
	ToolRef    string // tool request id (permission/form id); empty for decisions raised by oh
	Payload    DecisionPayload
	CreatedAt  time.Time
	ResolvedAt *time.Time
	ResolvedBy DecisionResolver
	Resolution *DecisionResolution
}

// Open reports whether the decision still waits for an answer.
func (d Decision) Open() bool { return d.ResolvedAt == nil }

// DecisionID returns the stable identifier of a decision.
func DecisionID(kind DecisionKind, sessionID, ref string) string {
	return string(kind) + ":" + sessionID + ":" + ref
}

// DecisionFilter selects decisions. Empty fields match everything.
type DecisionFilter struct {
	SessionID string
	GroupKey  string
	Kind      DecisionKind
}

// DecisionStore persists pending decisions (table pending_decisions).
type DecisionStore interface {
	// Upsert inserts a decision, or refreshes the payload of an existing one
	// (its creation date and resolution are kept).
	Upsert(ctx context.Context, d *Decision) error
	Get(ctx context.Context, id string) (*Decision, error)
	// ListOpen returns the unresolved decisions, oldest first.
	ListOpen(ctx context.Context, f DecisionFilter) ([]Decision, error)
	// ListSince returns the decisions created or resolved after t, oldest first.
	ListSince(ctx context.Context, t time.Time) ([]Decision, error)
	// Resolve closes an open decision. It returns false when the decision was
	// already resolved (first answer wins) or does not exist.
	Resolve(ctx context.Context, id string, by DecisionResolver, r *DecisionResolution, at time.Time) (bool, error)
	// Reopen cancels a resolution (a claimed answer that could not be delivered).
	Reopen(ctx context.Context, id string) error
}
