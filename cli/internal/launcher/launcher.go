// Package launcher provides a unified session launch pipeline.
//
// Every path that starts an opencode session (CLI oh start, TUI project mode,
// TUI omnibar, board quick actions, oh audit/review/debug) converges through
// Launcher.Launch(). This ensures consistent behaviour: compatibility checks,
// auto-deploy, session tracking, team-state events, and provider credential
// resolution happen once, in one place.
package launcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/platform"
	"github.com/datichb/openhub/cli/internal/prompt"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Launcher orchestrates the full session launch pipeline.
type Launcher struct {
	App *app.App
	UI  LaunchUI
}

// New creates a Launcher with the given app and UI layer.
func New(a *app.App, ui LaunchUI) *Launcher {
	return &Launcher{App: a, UI: ui}
}

// Launch executes the full session launch pipeline.
//
// The caller is responsible for resolving the project (opts.ProjectID/ProjectPath)
// and the agent+prompt before calling Launch. The launcher handles:
//   - compatibility check
//   - provider + credentials resolution
//   - stack detection
//   - auto-deploy (unless SkipDeploy)
//   - session summary (unless SkipSummary)
//   - launch confirmation (unless SkipConfirm)
//   - session persistence (create + update)
//   - opencode.Run()
//   - post-session team-state events
func (l *Launcher) Launch(ctx context.Context, opts LaunchOpts) error {
	a := l.App

	// ── 1. Compatibility check ──
	if a.Platform != nil {
		if ocVersion, err := a.Platform.Version(); err == nil {
			compat := checkPlatformCompat(buildinfo.Version, ocVersion)
			if compat != "" {
				l.UI.Notify(compat, LevelWarning)
			}
		}
	}

	// ── 2. Resolve launch path ──
	launchPath := opts.ProjectPath
	if launchPath == "" {
		return fmt.Errorf("launcher: project path is required")
	}

	// ── 3. Resolve provider ──
	prov := opts.Provider
	if prov == "" {
		// Look up from project DB
		if opts.ProjectID != "" && a.Projects != nil {
			if proj, err := a.Projects.Get(ctx, opts.ProjectID); err == nil && proj.Provider != "" {
				prov = proj.Provider
			}
		}
	}
	if prov == "" {
		prov = a.Config.Opencode.DefaultProvider
	}
	if prov == "" {
		prov = "bedrock"
	}

	// ── 4. Resolve credentials ──
	var bearerToken, apiKey, awsProfile, awsRegion string
	if a.Secrets != nil {
		bearerToken, apiKey, awsProfile, awsRegion = l.resolveCredentials(ctx, opts.ProjectID, prov)
	}

	// ── 5. Detect stack ──
	stack := prompt.DetectStack(launchPath)

	// ── 6. Auto-deploy ──
	if !opts.SkipDeploy && opts.DeployFunc != nil {
		opts.DeployFunc(a, prov)
	}

	// ── 7. Summary ──
	if !opts.SkipSummary && opts.SummaryFunc != nil {
		opts.SummaryFunc(prov, stack, bearerToken)
	}

	// ── 8. Confirmation ──
	if !opts.SkipConfirm {
		ok, err := l.UI.Confirm("Press Enter to launch opencode...")
		if err != nil || !ok {
			return err
		}
	}

	// ── 9. Persist session ──
	session := &domain.Session{
		ID:         uuid.New().String(),
		ProjectID:  opts.ProjectID,
		Status:     domain.SessionStatusRunning,
		Provider:   prov,
		LaunchPath: launchPath,
		PID:        os.Getpid(),
	}

	// Inject member_id from team config if available
	var resolved config.ResolvedTeamConfig
	if opts.ProjectID != "" && a.Projects != nil {
		if proj, err := a.Projects.Get(ctx, opts.ProjectID); err == nil {
			resolved = config.ResolveTeamForProject(a.Config, proj)
			if resolved.Enabled && resolved.MemberID != "" {
				mid := resolved.MemberID
				session.MemberID = &mid
			}
		}
	}

	if a.Sessions != nil {
		if err := a.Sessions.Create(ctx, session); err != nil {
			slog.Warn("session tracking failed", "error", err)
		}
	}

	// ── 10. Build platform RunOpts and run (ADR-036) ──
	platformOpts := platform.RunOpts{
		ProjectPath: launchPath,
		ProjectID:   opts.ProjectID,
		Agent:       opts.Agent,
		Prompt:      opts.Prompt,
		Provider:    prov,
		Credentials: platform.Credentials{
			BearerToken: bearerToken,
			APIKey:      apiKey,
			AWSProfile:  awsProfile,
			AWSRegion:   awsRegion,
		},
		ExtraArgs: opts.ExtraArgs,
	}

	var result *platform.RunResult
	var runErr error
	if l.UI.SuspendAndExec() != nil {
		// TUI mode: suspend the UI, run the platform, then resume
		runErr = l.UI.SuspendAndExec()(func() error {
			var err error
			result, err = a.Platform.RunInteractive(ctx, platformOpts)
			return err
		})
	} else {
		// CLI mode: run directly
		result, runErr = a.Platform.RunInteractive(ctx, platformOpts)
	}

	// ── 11. Post-run: update session with enrichment data ──
	if a.Sessions != nil && session.ID != "" {
		if runErr != nil {
			session.Status = domain.SessionStatusFailed
		} else {
			session.Status = domain.SessionStatusCompleted
		}
		now := time.Now()
		session.EndedAt = &now

		// Enrich from platform result (ADR-036 — fixes zero-token problem)
		if result != nil {
			if result.Model != "" {
				session.Model = result.Model
			}
			session.Cost = result.Cost
			session.TokensIn = result.TokensIn
			session.TokensOut = result.TokensOut
			session.TokensReasoning = result.TokensReasoning
			session.TokensCacheRead = result.TokensCacheRead
			session.Platform = string(a.Platform.Name())
			if result.ExternalSessionID != "" {
				session.ExternalSessionID = &result.ExternalSessionID
			}
			if result.Slug != "" {
				session.Slug = &result.Slug
			}
		}

		if err := a.Sessions.Update(ctx, session); err != nil {
			slog.Warn("post-run session update failed", "session_id", session.ID, "error", err)
		}

		// Emit session.complete event to team-state (async, non-blocking)
		if resolved.Enabled && resolved.MemberID != "" && resolved.StateRepo != "" {
			repo := teamstate.NewRepo(resolved.StateRepo, resolved.StatePath)
			if repo.IsCloned() {
				durationSec := 0.0
				if session.EndedAt != nil {
					durationSec = session.EndedAt.Sub(session.StartedAt).Seconds()
				}
				event := teamstate.NewSessionCompleteEvent(resolved.MemberID, session.ProjectID, map[string]interface{}{
					"session_id": session.ID,
					"duration_s": durationSec,
					"tokens_in":  session.TokensIn,
					"tokens_out": session.TokensOut,
					"provider":   session.Provider,
					"model":      session.Model,
					"status":     string(session.Status),
				})
				repo.AppendEventAsync(event)
			}
		}
	}

	return runErr
}

// resolveCredentials extracts provider-specific credentials from secrets.
// Uses provider.KeychainKey() for canonical key naming.
func (l *Launcher) resolveCredentials(ctx context.Context, projectID, prov string) (bearerToken, apiKey, awsProfile, awsRegion string) {
	a := l.App
	provName := provider.Name(prov)
	switch provName {
	case provider.Bedrock:
		bearerToken, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, projectID))
		if bearerToken == "" {
			bearerToken, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, ""))
		}
		// AWS profile/region: project override → hub config
		if projectID != "" && a.Projects != nil {
			if proj, err := a.Projects.Get(ctx, projectID); err == nil && proj.ProviderConfig != nil {
				if proj.ProviderConfig.AWSProfile != "" {
					awsProfile = proj.ProviderConfig.AWSProfile
				}
				if proj.ProviderConfig.AWSRegion != "" {
					awsRegion = proj.ProviderConfig.AWSRegion
				}
			}
		}
		if awsProfile == "" {
			awsProfile = a.Config.Provider.Bedrock.AWSProfile
		}
		if awsRegion == "" {
			awsRegion = a.Config.Provider.Bedrock.AWSRegion
		}
	case provider.Anthropic:
		apiKey, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, projectID))
		if apiKey == "" {
			apiKey, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, ""))
		}
	case provider.OpenRouter:
		apiKey, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, projectID))
		if apiKey == "" {
			apiKey, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, ""))
		}
	}
	return
}

// checkPlatformCompat performs a basic compatibility check between the hub CLI
// and the platform backend version. Returns a warning message or empty string.
// Detailed compatibility logic remains in the opencode adapter package; this is
// a lightweight placeholder until the Platform interface exposes CompatCheck().
func checkPlatformCompat(_, _ string) string {
	// TODO: delegate to platform.CompatibilityCheck() when available (Phase 4).
	// For now, the opencode-specific compatibility matrix is still checked by
	// cmd/start.go before entering the launcher.
	return ""
}
