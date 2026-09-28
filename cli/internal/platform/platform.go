// Package platform defines the interfaces for AI session backends.
//
// The hub interacts with coding session runtimes (OpenCode, future direct LLM
// APIs, or other coding assistants) exclusively through these interfaces.
// Concrete implementations live in dedicated adapter packages (e.g.
// internal/opencode for the OpenCode adapter).
//
// Three interfaces partition the surface area:
//   - SessionPlatform: execution lifecycle (interactive, headless, resume, guard)
//   - SessionServer: serve-mode HTTP API (parallel sessions)
//   - StatsProvider: metrics and session history
//
// See ADR-036 for the architectural decision.
package platform

import (
	"context"
	"time"
)

// Name identifies a platform backend.
type Name string

const (
	// OpenCode is the default platform — external TypeScript binary with
	// TUI, CLI headless, and HTTP serve modes.
	OpenCode Name = "opencode"

	// DirectLLM is a future platform that calls LLM provider APIs directly
	// without an intermediate binary.
	DirectLLM Name = "directllm"
)

// Credentials holds provider-specific authentication tokens.
type Credentials struct {
	BearerToken string // Bedrock bearer token
	APIKey      string // Provider API key (Anthropic, OpenRouter)
	AWSProfile  string // AWS profile override
	AWSRegion   string // AWS region override
}

// RunOpts configures an interactive coding session.
type RunOpts struct {
	ProjectPath string
	ProjectID   string
	Agent       string
	Prompt      string
	Provider    string
	Credentials Credentials
	Model       string   // Model override (empty = backend default)
	Files       []string // Files to pre-attach to the session context
	ResumeID    string   // Resume an existing session (empty = new session)
	ExtraArgs   []string // Backend-specific passthrough arguments
}

// RunResult holds structured data returned after an interactive session completes.
// The platform adapter is responsible for populating these fields from whatever
// data source it has (DB lookup, API response, parsed output, etc.).
type RunResult struct {
	ExternalSessionID string   // Session ID in the backend's own system
	Slug              string   // Human-readable identifier (e.g. "shiny-nebula")
	Title             string   // Session title from the backend (e.g. "Fix auth bug")
	Model             string   // Model that was actually used
	Cost              float64  // Session cost in USD
	TokensIn          int64    // Input tokens consumed
	TokensOut         int64    // Output tokens generated
	TokensReasoning   int64    // Reasoning/thinking tokens
	TokensCacheRead   int64    // Tokens served from cache
	FilesModified     []string // Files changed during the session
	SummaryAdditions  int      // Lines added
	SummaryDeletions  int      // Lines removed
	SummaryFiles      int      // Number of files changed
}

// HeadlessOpts configures a non-interactive (batch) run.
type HeadlessOpts struct {
	ProjectPath string
	ProjectID   string
	Agent       string
	Prompt      string
	Format      string      // "" (free text) or "json"
	Model       string      // Model override (empty = backend default)
	Files       []string    // Files to attach to the prompt context
	ExtraArgs   []string    // Backend-specific passthrough arguments
	Provider    string      // Provider name (same cascade as RunOpts)
	Credentials Credentials // Provider credentials (same as RunOpts)
}

// HeadlessResult holds structured output from a headless run.
type HeadlessResult struct {
	Content         string  // Generated text content
	Model           string  // Model that produced the response
	TokensIn        int64   // Input tokens consumed
	TokensOut       int64   // Output tokens generated
	TokensReasoning int64   // Reasoning/thinking tokens
	Cost            float64 // Run cost in USD
	RawOutput       string  // Unprocessed output (for debugging)
}

// ActiveSession represents a session that may still be running.
// Used by the session guard to prevent conflicting concurrent sessions.
type ActiveSession struct {
	SessionID  string
	StartedAt  time.Time
	LaunchPath string
	PID        int // Process ID, 0 = unknown
}

// Capabilities describes optional features a platform supports.
type Capabilities struct {
	Parallel bool // can run multiple concurrent sessions (via ParallelRunner)
	Events   bool // supports real-time event streaming (EventSource)
}

// SessionPlatform is the primary abstraction for an AI coding session backend.
//
// Implementations:
//   - internal/opencode: OpenCode binary (TUI, headless, serve)
//   - (future) internal/claudecode: Claude Code CLI (TUI, headless, --bg daemon)
//   - (future) internal/directllm: direct Anthropic/Bedrock API calls
//
// Consumers (launcher, parallel coordinator, cmd/, tui/) depend on this
// interface, never on a concrete adapter package.
type SessionPlatform interface {
	// Name returns the platform identifier.
	Name() Name

	// Available reports whether the platform backend is usable
	// (e.g. binary found, API reachable).
	Available() bool

	// Version returns the backend's version string.
	Version() (string, error)

	// RunInteractive starts an interactive coding session (with TUI or
	// inherited stdin/stdout). Blocks until the session completes.
	// The returned RunResult contains post-session enrichment data.
	RunInteractive(ctx context.Context, opts RunOpts) (*RunResult, error)

	// ExecReplace replaces the current process with the backend
	// (Unix exec semantics). Used for session resume. Never returns on success.
	ExecReplace(opts RunOpts) error

	// RunHeadless executes a non-interactive run and returns structured output.
	RunHeadless(ctx context.Context, opts HeadlessOpts) (*HeadlessResult, error)

	// FindActiveSessions returns sessions that appear to be running on the
	// given path. Ghost sessions (dead processes) should be excluded.
	FindActiveSessions(ctx context.Context, projectPath string) ([]ActiveSession, error)

	// IsGhostSession reports whether a "running" session is likely dead.
	IsGhostSession(s ActiveSession) bool

	// RequiresDeploy reports whether this platform needs a deploy step
	// (agents, skills, config written to the project directory) before
	// sessions can run.
	RequiresDeploy() bool

	// Capabilities reports which optional interfaces this platform supports.
	Capabilities() Capabilities

	// NewParallelRunner creates a runner for parallel task orchestration.
	// Returns an error if the platform does not support parallel execution
	// (i.e. Capabilities().Parallel is false).
	NewParallelRunner(opts ParallelRunnerOpts) (ParallelRunner, error)
}
