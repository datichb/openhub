// Package llm provides an abstraction for LLM inference.
//
// The default implementation delegates to the opencode binary in headless mode
// (opencode run --auto), which supports all configured providers (Bedrock,
// Anthropic, OpenRouter, GitHub Copilot).
//
// Future implementations may call provider APIs directly without requiring the
// opencode binary — add a new file implementing the Completer interface and
// swap the injection site in cmd/start_sweep_mode.go.
package llm

import (
	"context"

	"github.com/datichb/openhub/cli/internal/platform"
)

// Request describes an LLM completion request.
type Request struct {
	// Prompt is the user-facing prompt to send (required).
	Prompt string

	// Format constrains the output shape: "" (free text) or "json".
	Format string

	// SystemHint is an optional system-level instruction.
	// Some implementations may ignore it (e.g. opencode headless).
	SystemHint string

	// ProjectPath is the working directory for context resolution.
	ProjectPath string

	// Agent selects a specific agent profile (opencode-specific, ignored by
	// direct API implementations).
	Agent string

	// Model overrides the default model (e.g. "anthropic/claude-sonnet-4-20250514").
	// Empty = use the provider's default.
	Model string

	// Files attaches local files to the prompt context.
	Files []string

	// Provider is the LLM provider name for credential injection.
	// Empty = use the platform's default resolution.
	Provider string

	// Credentials holds provider-specific authentication tokens.
	// Passed through to the underlying platform.RunHeadless call.
	Credentials platform.Credentials
}

// Response holds the LLM's output.
type Response struct {
	// Content is the generated text.
	Content string

	// Model is the model that produced the response (informational).
	Model string
}

// Completer is the abstraction for LLM inference.
//
// Implementations:
//   - OpenCodeCompleter: delegates to opencode run --auto (default)
//   - (future) AnthropicCompleter: calls the Anthropic Messages API directly
//   - (future) BedrockCompleter: calls AWS Bedrock InvokeModel directly
//
// The sweep package and any other consumer should depend on this interface,
// never on a concrete implementation. The injection site (typically in
// cmd/start_sweep_mode.go) selects which implementation to use.
type Completer interface {
	Complete(ctx context.Context, req Request) (*Response, error)
}
