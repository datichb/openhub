package opencode

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/platform"
)

// Platform implements platform.SessionPlatform for the OpenCode binary.
type Platform struct{}

// Compile-time check.
var _ platform.SessionPlatform = (*Platform)(nil)

// NewPlatform returns a SessionPlatform backed by the OpenCode binary.
func NewPlatform() *Platform {
	return &Platform{}
}

func (p *Platform) Name() platform.Name { return platform.OpenCode }

func (p *Platform) Available() bool {
	_, err := FindBinary()
	return err == nil
}

func (p *Platform) Version() (string, error) {
	return Version()
}

func (p *Platform) RunInteractive(ctx context.Context, opts platform.RunOpts) (*platform.RunResult, error) {
	startOpts := toStartOpts(opts)

	runErr := Run(startOpts)

	// Post-run enrichment: read opencode.db for the session that just completed
	result := &platform.RunResult{}
	if enrichErr := p.enrichFromDB(opts.ProjectPath, result); enrichErr != nil {
		slog.Debug("platform: post-run enrichment failed", "error", enrichErr)
	}

	return result, runErr
}

func (p *Platform) ExecReplace(opts platform.RunOpts) error {
	return Exec(toStartOpts(opts))
}

func (p *Platform) RunHeadless(ctx context.Context, opts platform.HeadlessOpts) (*platform.HeadlessResult, error) {
	output, err := RunHeadless(HeadlessOpts{
		ProjectPath: opts.ProjectPath,
		ProjectID:   opts.ProjectID,
		Agent:       opts.Agent,
		Prompt:      opts.Prompt,
		Format:      opts.Format,
		Model:       opts.Model,
		Files:       opts.Files,
	})
	if err != nil {
		return &platform.HeadlessResult{
			Content:   output,
			RawOutput: output,
		}, err
	}
	return &platform.HeadlessResult{
		Content:   output,
		RawOutput: output,
	}, nil
}

func (p *Platform) FindActiveSessions(ctx context.Context, projectPath string) ([]platform.ActiveSession, error) {
	// This method works at the platform level — it doesn't have access to the
	// SessionStore. The launcher uses it for guard checks.
	// For now, return nil; the tui_board.go call site will be migrated
	// separately since it uses the domain SessionStore directly.
	return nil, nil
}

func (p *Platform) IsGhostSession(s platform.ActiveSession) bool {
	const ghostThreshold = 24 * time.Hour
	return time.Since(s.StartedAt) > ghostThreshold
}

func (p *Platform) SupportsServeMode() bool { return true }

func (p *Platform) NewServer(port int, dir string, id string) (platform.SessionServer, error) {
	// Phase 3 — to be implemented when parallel mode is migrated
	return nil, fmt.Errorf("platform opencode: NewServer not yet implemented (Phase 3)")
}

func (p *Platform) RequiresDeploy() bool { return true }

// --- Internal helpers ---

func toStartOpts(opts platform.RunOpts) StartOpts {
	return StartOpts{
		ProjectPath:     opts.ProjectPath,
		ProjectID:       opts.ProjectID,
		Agent:           opts.Agent,
		Prompt:          opts.Prompt,
		Provider:        opts.Provider,
		BearerToken:     opts.Credentials.BearerToken,
		APIKey:          opts.Credentials.APIKey,
		AWSProfile:      opts.Credentials.AWSProfile,
		AWSRegion:       opts.Credentials.AWSRegion,
		ResumeSessionID: opts.ResumeID,
		ExtraArgs:       opts.ExtraArgs,
	}
}

// enrichFromDB reads opencode.db to fill in tokens, cost, model from the most
// recent session matching the project path. Uses a retry with backoff since
// opencode may not have flushed the DB row yet when the process exits.
func (p *Platform) enrichFromDB(projectPath string, result *platform.RunResult) error {
	if projectPath == "" {
		return nil
	}

	var lastErr error
	for _, delay := range []time.Duration{0, 500 * time.Millisecond, 1 * time.Second, 2 * time.Second} {
		if delay > 0 {
			time.Sleep(delay)
		}

		db, err := openDB()
		if err != nil || db == nil {
			lastErr = err
			continue
		}

		sessions, err := ProjectSessions(db, projectPath, 1)
		db.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if len(sessions) == 0 {
			lastErr = fmt.Errorf("no sessions found for path %s", projectPath)
			continue
		}

		latest := sessions[0]
		result.ExternalSessionID = latest.ID
		result.Model = latest.Model
		result.Cost = latest.Cost
		result.TokensIn = latest.TokensInput
		result.TokensOut = latest.TokensOutput
		result.TokensReasoning = latest.TokensReasoning
		result.TokensCacheRead = latest.TokensCacheRead
		return nil
	}
	return lastErr
}
