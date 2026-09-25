package llm

import (
	"context"
	"fmt"

	"github.com/datichb/openhub/cli/internal/platform"
)

// PlatformCompleter implements Completer by delegating to a SessionPlatform's
// headless mode. It works with any platform backend (OpenCode, direct API, etc.).
//
// This replaces the former OpenCodeCompleter. The injection site is
// cmd/start_sweep_mode.go.
type PlatformCompleter struct {
	// Platform is the AI session backend to delegate to.
	Platform platform.SessionPlatform
	// ProjectID is the hub project ID passed to the platform.
	// Optional — leave empty when project context is not needed.
	ProjectID string
}

// NewPlatformCompleter returns a Completer backed by the given platform's headless mode.
func NewPlatformCompleter(p platform.SessionPlatform, projectID string) *PlatformCompleter {
	return &PlatformCompleter{Platform: p, ProjectID: projectID}
}

// Complete sends the request to the platform in headless mode and returns the
// captured output.
func (c *PlatformCompleter) Complete(ctx context.Context, req Request) (*Response, error) {
	result, err := c.Platform.RunHeadless(ctx, platform.HeadlessOpts{
		ProjectPath: req.ProjectPath,
		ProjectID:   c.ProjectID,
		Agent:       req.Agent,
		Prompt:      req.Prompt,
		Format:      req.Format,
		Model:       req.Model,
		Files:       req.Files,
	})
	if err != nil {
		return nil, fmt.Errorf("platform headless completion failed: %w", err)
	}
	return &Response{
		Content: result.Content,
		Model:   result.Model,
	}, nil
}

// Deprecated: OpenCodeCompleter is an alias for PlatformCompleter.
// Use NewPlatformCompleter instead.
type OpenCodeCompleter = PlatformCompleter

// Deprecated: NewOpenCodeCompleter creates a PlatformCompleter.
// It requires the platform to be available in the app container.
// Prefer NewPlatformCompleter(a.Platform, projectID) instead.
func NewOpenCodeCompleter(p platform.SessionPlatform, projectID string) *OpenCodeCompleter {
	return NewPlatformCompleter(p, projectID)
}
