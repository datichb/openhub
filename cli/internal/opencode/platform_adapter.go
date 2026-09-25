package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/platform"
)

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

	result := &platform.HeadlessResult{
		Content:   output,
		RawOutput: output,
	}

	// Attempt to parse structured metadata from JSON output.
	// opencode --format json emits JSONL; we try to extract metadata
	// from the last parseable JSON object that contains usage info.
	if output != "" {
		parseHeadlessJSON(output, result)
	}

	return result, err
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
	bin, err := FindBinary()
	if err != nil {
		return nil, fmt.Errorf("opencode binary not found for serve mode: %w", err)
	}
	// The actual server creation is done by the caller (parallel package)
	// which has access to NewServerAdapter without import cycle.
	// We return the binary path via a ServerFactory pattern.
	if p.serverFactory != nil {
		return p.serverFactory(port, dir, id, bin)
	}
	return nil, fmt.Errorf("platform opencode: server factory not configured (call SetServerFactory)")
}

// ServerFactory creates a platform.SessionServer given port, dir, id, and binary path.
type ServerFactory func(port int, dir, id, bin string) (platform.SessionServer, error)

// SetServerFactory configures the factory used by NewServer.
// This breaks the import cycle: the parallel package sets the factory during wiring.
func (p *Platform) SetServerFactory(f ServerFactory) {
	p.serverFactory = f
}

// Platform implements platform.SessionPlatform for the OpenCode binary.
type Platform struct {
	serverFactory ServerFactory
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

// parseHeadlessJSON attempts to extract structured metadata from opencode's
// JSON/JSONL output. It scans lines from the end looking for objects that
// contain usage/model information. This is best-effort — if parsing fails
// the result keeps its raw Content and zero-valued metadata fields.
//
// Known output shapes from opencode --format json:
//   - JSONL with event objects: {"type":"text","text":"..."}
//   - Final summary object: {"model":"...","usage":{"input_tokens":N,...},"cost":N}
//   - Plain text content (non-JSON) mixed with JSON lines
func parseHeadlessJSON(output string, result *platform.HeadlessResult) {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Scan from the end to find the most recent metadata-bearing line.
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || line[0] != '{' {
			continue
		}

		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		// Look for model field
		if m, ok := obj["model"].(string); ok && result.Model == "" {
			result.Model = m
		}

		// Look for cost field
		if c, ok := obj["cost"].(float64); ok && result.Cost == 0 {
			result.Cost = c
		}

		// Look for usage/tokens in various shapes
		if usage, ok := obj["usage"].(map[string]interface{}); ok {
			if v, ok := usage["input_tokens"].(float64); ok {
				result.TokensIn = int64(v)
			}
			if v, ok := usage["output_tokens"].(float64); ok {
				result.TokensOut = int64(v)
			}
			if v, ok := usage["reasoning_tokens"].(float64); ok {
				result.TokensReasoning = int64(v)
			}
			// Found usage data — this is likely the summary line
			break
		}

		// Alternative: flat tokens fields
		if v, ok := obj["tokens_input"].(float64); ok {
			result.TokensIn = int64(v)
			if v2, ok := obj["tokens_output"].(float64); ok {
				result.TokensOut = int64(v2)
			}
			break
		}

		// If we found model or cost, keep scanning for usage
		if result.Model != "" || result.Cost != 0 {
			continue
		}
	}

	// Extract text content from JSONL events if the output is structured.
	// Look for {"type":"text","text":"..."} patterns and concatenate them.
	var textParts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		if t, ok := obj["type"].(string); ok && t == "text" {
			if text, ok := obj["text"].(string); ok {
				textParts = append(textParts, text)
			}
		}
	}
	if len(textParts) > 0 {
		result.Content = strings.Join(textParts, "")
	}
}
