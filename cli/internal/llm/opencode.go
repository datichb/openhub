package llm

import (
	"context"
	"fmt"

	"github.com/datichb/openhub/cli/internal/opencode"
)

// OpenCodeCompleter implements Completer by delegating to the opencode binary
// in headless mode (opencode run --auto). It is compatible with every provider
// configured in opencode (Bedrock, Anthropic, OpenRouter, GitHub Copilot).
//
// This is the default implementation. To switch to a direct API call, create a
// new Completer implementation and inject it in place of this one.
type OpenCodeCompleter struct {
	// ProjectID is the hub project ID passed to the opencode environment.
	// Optional — leave empty when project context is not needed.
	ProjectID string
}

// NewOpenCodeCompleter returns a Completer backed by opencode run --auto.
func NewOpenCodeCompleter(projectID string) *OpenCodeCompleter {
	return &OpenCodeCompleter{ProjectID: projectID}
}

// Complete sends the request to opencode in headless mode and returns the
// captured output.
func (c *OpenCodeCompleter) Complete(ctx context.Context, req Request) (*Response, error) {
	output, err := opencode.RunHeadless(opencode.HeadlessOpts{
		ProjectPath: req.ProjectPath,
		ProjectID:   c.ProjectID,
		Prompt:      req.Prompt,
		Format:      req.Format,
		Agent:       req.Agent,
		Model:       req.Model,
		Files:       req.Files,
	})
	if err != nil {
		return nil, fmt.Errorf("opencode headless completion failed: %w", err)
	}
	return &Response{Content: output}, nil
}
