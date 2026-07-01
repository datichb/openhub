package domain

import "time"

// Session represents an opencode session launched via oh.
type Session struct {
	ID        string
	ProjectID string
	StartedAt time.Time
	EndedAt   *time.Time
	Status    SessionStatus
	Provider  string
	Model     string
	TokensIn  int64
	TokensOut int64
}

// SessionStatus represents the state of a session.
type SessionStatus string

const (
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
)

// SessionStore defines the contract for session persistence.
type SessionStore interface {
	// List returns sessions for a project. Empty projectID returns all.
	List(projectID string) ([]Session, error)
	// Get retrieves a session by ID.
	Get(id string) (*Session, error)
	// Create inserts a new session.
	Create(s *Session) error
	// Update modifies an existing session.
	Update(s *Session) error
}
