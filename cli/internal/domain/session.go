package domain

import (
	"context"
	"time"
)

// Session represents a coding session launched via oh.
type Session struct {
	ID         string
	ProjectID  string
	StartedAt  time.Time
	EndedAt    *time.Time
	Status     SessionStatus
	Provider   string
	Model      string
	TokensIn   int64
	TokensOut  int64
	LaunchPath string  // filesystem path where the session was launched (base or worktree)
	MemberID   *string // team member who ran the session (nil = solo / unknown)

	// Platform enrichment fields (ADR-036, migration v24)
	Cost              float64 // session cost in USD from the backend
	TokensReasoning   int64   // reasoning/thinking tokens
	TokensCacheRead   int64   // tokens served from cache
	Platform          string  // backend name ("opencode", "directllm", ...)
	ExternalSessionID *string // session ID in the backend's own system
	Slug              *string // human-readable session identifier
	PID               int     // OS process ID that owns this session (0 = unknown/legacy)
	Title             *string // session title from the backend (e.g. "Fix auth bug")

	// Headless tracking fields (migration v27)
	Type          SessionType // "interactive" (default) or "headless"
	Label         *string     // human-readable use-case label (e.g. "brief-enrichment", "sweep-decomposition")
	CorrelationID *string     // groups related headless runs (e.g. all runs in a single sweep)
}

// SessionStatus represents the state of a session.
type SessionStatus string

const (
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
)

// SessionType distinguishes interactive (TUI) sessions from headless (batch) runs.
type SessionType string

const (
	SessionTypeInteractive SessionType = "interactive"
	SessionTypeHeadless    SessionType = "headless"
)

// SessionStore defines the contract for session persistence.
type SessionStore interface {
	// List returns sessions for a project. Empty projectID returns all.
	List(ctx context.Context, projectID string) ([]Session, error)
	// Get retrieves a session by ID.
	Get(ctx context.Context, id string) (*Session, error)
	// Create inserts a new session.
	Create(ctx context.Context, s *Session) error
	// Update modifies an existing session.
	Update(ctx context.Context, s *Session) error
	// ListRunning returns sessions with status "running" for a project.
	ListRunning(ctx context.Context, projectID string) ([]Session, error)
}
