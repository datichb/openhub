package parallel

import (
	"context"
	"time"

	"github.com/datichb/openhub/cli/internal/platform"
)

// SessionServer manages a single serve-mode instance of a platform backend.
//
// This interface is specific to the OpenCode-style HTTP serve mode. It lives in
// the parallel package because it is an implementation detail of the coordinator's
// current architecture. Future backends will use platform.ParallelRunner instead.
//
// In Lot 3 (ADR-036), the coordinator will be refactored to depend on
// platform.ParallelRunner, and SessionServer will become an internal detail of
// the OpenCode ParallelRunner implementation.
//
// Implementations:
//   - parallel.ServerAdapter: wraps OpenCodeServer for the opencode serve HTTP API
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
	GetModifiedFiles() ([]platform.FileChange, error)

	// AbortSession cancels a running session.
	AbortSession(sessionID string) error

	// TicketID returns the task/ticket identifier this server is working on.
	TicketID() string

	// Port returns the port this server is listening on.
	Port() int

	// Dir returns the working directory of this server.
	Dir() string
}
