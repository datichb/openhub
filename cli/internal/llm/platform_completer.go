package llm

import (
	"context"
	"fmt"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/headlesstrack"
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
	// Provider is the default provider for headless runs.
	// Used when the Request does not specify a provider.
	Provider string
	// Credentials are the default credentials for headless runs.
	// Used when the Request does not specify credentials.
	Credentials platform.Credentials

	// Sessions enables headless run tracking. If non-nil, each Complete() call
	// creates a session record with cost/token data in the hub's session store.
	Sessions domain.SessionStore
	// CorrelationID groups related headless runs (e.g. all calls within a sweep).
	CorrelationID string
	// TrackLabel identifies the use-case for tracked runs (e.g. "sweep-decomposition").
	TrackLabel string
	// MemberID is the team member running these completions.
	MemberID string
}

// NewPlatformCompleter returns a Completer backed by the given platform's headless mode.
func NewPlatformCompleter(p platform.SessionPlatform, projectID string) *PlatformCompleter {
	return &PlatformCompleter{Platform: p, ProjectID: projectID}
}

// Complete sends the request to the platform in headless mode and returns the
// captured output.
func (c *PlatformCompleter) Complete(ctx context.Context, req Request) (*Response, error) {
	// Use request-level provider/credentials if set, otherwise fall back to
	// completer defaults (configured at construction by the caller).
	prov := req.Provider
	if prov == "" {
		prov = c.Provider
	}
	creds := req.Credentials
	if creds == (platform.Credentials{}) {
		creds = c.Credentials
	}

	headlessOpts := platform.HeadlessOpts{
		ProjectPath: req.ProjectPath,
		ProjectID:   c.ProjectID,
		Agent:       req.Agent,
		Prompt:      req.Prompt,
		Format:      req.Format,
		Model:       req.Model,
		Files:       req.Files,
		Provider:    prov,
		Credentials: creds,
	}

	// Wrap with tracking if a session store is configured.
	var result *platform.HeadlessResult
	var err error
	if c.Sessions != nil {
		result, err = headlesstrack.Track(ctx, headlesstrack.Opts{
			Sessions:      c.Sessions,
			PlatformName:  string(c.Platform.Name()),
			ProjectID:     c.ProjectID,
			ProjectPath:   req.ProjectPath,
			Provider:      prov,
			Label:         c.TrackLabel,
			CorrelationID: c.CorrelationID,
			MemberID:      c.MemberID,
		}, func(ctx context.Context) (*platform.HeadlessResult, error) {
			return c.Platform.RunHeadless(ctx, headlessOpts)
		})
	} else {
		result, err = c.Platform.RunHeadless(ctx, headlessOpts)
	}

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
