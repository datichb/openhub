package platform

import (
	"context"
	"encoding/json"
)

// ParallelRunner orchestrates multiple concurrent coding sessions.
//
// This is the high-level abstraction for parallel task execution. Each platform
// adapter provides its own implementation based on its capabilities:
//   - OpenCode: via opencode serve HTTP API (wraps SessionServer instances)
//   - Claude Code (future): via claude --bg daemon + claude agents --json
//   - Cline (future): via --team-name multi-agent coordination
//   - Direct API / Aider (future): via N concurrent headless goroutines
//
// The coordinator (internal/parallel/) depends on this interface. It handles
// worktree management, conflict detection, notifications, and state tracking.
// The ParallelRunner handles the platform-specific lifecycle of individual tasks.
//
// See ADR-036 for the architectural decision.
type ParallelRunner interface {
	// LaunchTask starts a new coding session for the given task.
	// The task should be fully configured (worktree path, prompt, agent).
	// Returns a handle with the backend-assigned session ID.
	LaunchTask(ctx context.Context, opts TaskOpts) (TaskHandle, error)

	// GetAllStatuses returns the current status of all launched tasks.
	// Keys are task IDs (matching TaskOpts.TaskID).
	GetAllStatuses(ctx context.Context) (map[string]TaskStatus, error)

	// GetModifiedFiles returns files changed by a specific task.
	GetModifiedFiles(ctx context.Context, taskID string) ([]FileChange, error)

	// SendMessage sends a notification or follow-up message to a running task.
	// Used by the coordinator for inter-task conflict and completion notifications.
	SendMessage(ctx context.Context, taskID, message string) error

	// AbortTask cancels a running task.
	AbortTask(ctx context.Context, taskID string) error

	// AttachTask gives the user interactive access to a running task.
	// Blocks until the user detaches. Returns an error if the backend
	// does not support attaching (e.g. headless-only backends).
	AttachTask(ctx context.Context, taskID string) error

	// Cleanup shuts down all tasks and releases resources.
	// Called when the parallel run completes or is cancelled.
	Cleanup(ctx context.Context)
}

// TaskOpts configures a single task within a parallel run.
type TaskOpts struct {
	TaskID       string // hub-assigned task/ticket identifier
	Title        string // human-readable title for the session
	WorktreePath string // filesystem path (worktree) for this task
	Prompt       string // initial prompt to send
	Agent        string // agent profile to use
}

// TaskHandle is returned after a task is successfully launched.
type TaskHandle struct {
	TaskID    string // correlates with TaskOpts.TaskID
	SessionID string // backend-assigned session identifier
}

// TaskStatus represents the current state of a parallel task.
type TaskStatus struct {
	Status        string   // "pending", "starting", "running", "completed", "failed", "idle", "retrying"
	SessionID     string   // backend-assigned session ID (may change after recovery)
	FilesModified []string // files touched by this task
	Error         string   // error message if status is "failed"
}

// ParallelRunnerOpts configures the creation of a ParallelRunner.
// Backend-specific configuration is passed via the Config field.
type ParallelRunnerOpts struct {
	ProjectPath string
	ProjectID   string
	HubDir      string          // path to ~/.oh directory (for log files, state)
	Provider    string          // LLM provider name for credential injection
	Credentials Credentials     // Provider credentials injected into each task subprocess
	Config      json.RawMessage // backend-specific config (port range, daemon opts, etc.)
}

// EventSource is an optional interface for real-time event streaming.
//
// Platform adapters that support event streaming (e.g. OpenCode's SSE endpoint)
// implement this alongside ParallelRunner. When available, the coordinator uses
// events instead of polling GetAllStatuses(). When not available, the coordinator
// falls back to periodic polling.
//
// This is a separate interface (not embedded in ParallelRunner) because not all
// backends support it. Use a type assertion to check:
//
//	if es, ok := runner.(EventSource); ok {
//	    ch, _ := es.Subscribe(ctx)
//	    // event-driven mode
//	}
type EventSource interface {
	// Subscribe returns a channel of events from the parallel runner.
	// The channel is closed when the context is cancelled or the runner
	// shuts down. Events are delivered in order per task but may interleave
	// across tasks.
	Subscribe(ctx context.Context) (<-chan Event, error)
}

// Event represents a real-time notification from a parallel runner.
type Event struct {
	TaskID string          // which task this event concerns (empty = global)
	Type   string          // "status_change", "file_change", "message", "error", "heartbeat"
	Data   json.RawMessage // event-specific payload (shape depends on Type)
}
