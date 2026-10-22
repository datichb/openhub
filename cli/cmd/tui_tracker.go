package cmd

import (
	"context"
	"errors"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// teamService is the team service on the hub of the running command.
func teamService(a *app.App) *teamsvc.Service {
	return teamsvc.New(a.Config, a.Secrets, a.Projects)
}

// resolveTrackerEngine builds the tracker Engine of the team of the active
// project (the hub team when the project has none), with the rules of the
// team service. It returns nil, without error, when there is nothing to sync
// — no team, not cloned, no tracker, tracker disabled (enabled = false),
// credentials missing: callers treat nil as "skip tracker sync".
func resolveTrackerEngine(ctx context.Context, a *app.App) *tracker.Engine {
	project, _ := resolveActiveProject(a)
	engine, err := teamService(a).TrackerEngine(ctx, project)
	if err != nil {
		var credErr *teamsvc.CredentialsError
		var initErr *teamsvc.TrackerInitError
		switch {
		case errors.As(err, &credErr):
			slog.Warn("tracker.resolve_engine.credentials_failed", "error", err)
		case errors.As(err, &initErr):
			slog.Warn("tracker.resolve_engine.init_failed", "error", err)
		default:
			slog.Debug("tracker.resolve_engine.skip", "reason", err)
		}
		return nil
	}
	return engine
}

// buildCredentialSource constructs a tracker.CredentialSource from the app's
// secret store and a resolved EffectiveTrackerConfig, with the team of the
// hub for the GitLab token cascade.
func buildCredentialSource(a *app.App, eff tracker.EffectiveTrackerConfig) tracker.CredentialSource {
	return teamService(a).CredentialSource(tracker.NewCredentialSource(eff, nil), nil)
}
