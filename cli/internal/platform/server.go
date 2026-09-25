package platform

import (
	"context"
	"time"
)

// FileChange represents a file modification detected during a session.
type FileChange struct {
	Path      string // Relative path within the project
	Operation string // "created", "modified", or "deleted"
}

// SessionServer manages a single serve-mode instance of a platform backend.
//
// Used by the parallel coordinator to orchestrate multiple concurrent sessions.
// Not all platforms support serve mode — check SessionPlatform.SupportsServeMode()
// before calling NewServer().
//
// Implementations:
//   - internal/opencode: wraps `opencode serve` HTTP API
type SessionServer interface {
	// Start launches the server subprocess.
	Start(ctx context.Context) error

	// WaitReady blocks until the server is accepting connections or the
	// timeout expires.
	WaitReady(ctx context.Context, timeout time.Duration) error

	// IsAlive reports whether the server process is still running and
	// responding to health checks.
	IsAlive() bool

	// Dispose performs a graceful shutdown of the server.
	Dispose() error

	// Kill force-terminates the server process.
	Kill()

	// CreateSession creates a new coding session on the server and returns
	// its backend-assigned ID.
	CreateSession(title string) (sessionID string, err error)

	// SendPrompt sends a prompt to a running session. The agent parameter
	// selects the agent profile (empty = default).
	SendPrompt(sessionID, prompt, agent string) error

	// GetStatus returns the status of all sessions on this server.
	// Keys are session IDs, values are status strings (e.g. "running",
	// "completed", "idle", "error").
	GetStatus() (map[string]string, error)

	// GetModifiedFiles returns files changed by the active session(s).
	// Unlike the current OpenCode implementation that treats everything
	// as "modified", this returns typed FileChange values.
	GetModifiedFiles() ([]FileChange, error)

	// AbortSession cancels a running session.
	AbortSession(sessionID string) error

	// TicketID returns the task/ticket identifier this server is working on.
	TicketID() string

	// Port returns the port this server is listening on.
	Port() int

	// Dir returns the working directory of this server.
	Dir() string
}
