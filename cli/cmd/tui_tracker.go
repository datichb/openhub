package cmd

import (
	"context"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// resolveTrackerEngine builds a tracker Engine for the active team repo, if the
// tracker is configured and enabled in the team-state config.toml.
// Returns nil (no error) when tracker is not configured — callers should treat
// nil as "skip tracker sync".
func resolveTrackerEngine(ctx context.Context, a *app.App) *tracker.Engine {
	project, _ := resolveActiveProject(a)
	tc := resolvedTeamConfig(a, project)
	if !tc.Enabled {
		// Fallback: use hub-level team config (matches CLI sync-tracker behavior)
		hubTC := a.Config.ActiveTeam()
		if hubTC.StateRepo == "" {
			slog.Debug("tracker.resolve_engine.skip", "reason", "team not enabled")
			return nil
		}
		tc.Enabled = true
		tc.StateRepo = hubTC.StateRepo
		tc.StatePath = hubTC.StatePath
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		slog.Debug("tracker.resolve_engine.skip", "reason", "repo not cloned")
		return nil
	}

	teamCfg, err := repo.LoadConfig()
	if err != nil || teamCfg.Tracker.Type == "" {
		slog.Debug("tracker.resolve_engine.skip", "reason", "no tracker type configured")
		return nil
	}

	// Merge shared team-state config with local hub.toml overrides.
	effTracker := tracker.ResolveTrackerConfig(
		&teamCfg.Tracker,
		a.Config.Tracker,
	)
	if !effTracker.Enabled {
		slog.Debug("tracker.resolve_engine.skip", "reason", "tracker disabled locally")
		return nil
	}

	credSrc := buildCredentialSource(a, effTracker)
	cfg, err := tracker.ResolveCredentials(ctx, credSrc, tracker.Type(effTracker.Type))
	if err != nil {
		slog.Warn("tracker.resolve_engine.credentials_failed", "error", err)
		return nil
	}

	t, err := tracker.New(cfg)
	if err != nil {
		slog.Warn("tracker.resolve_engine.init_failed", "error", err)
		return nil
	}

	// Build a teamstate.TrackerConfig from the effective config for the engine.
	// Resolve the Projects map from hub projects associated with the team.
	projects, ticketPatterns := resolveTrackerProjects(ctx, a, effTracker)
	slog.Debug("tracker.resolve_engine.ready", "type", effTracker.Type, "projects", len(projects))

	engineCfg := teamstate.TrackerConfig{
		Type:                 effTracker.Type,
		Enabled:              effTracker.Enabled,
		AutoSync:             effTracker.AutoSync,
		SyncIntervalMinutes:  effTracker.SyncIntervalMinutes,
		AutoPlanAssigned:     effTracker.AutoPlanAssigned,
		MaxAutoPlanPerMember: effTracker.MaxAutoPlanPerMember,
		AutoPlanUnassigned:   effTracker.AutoPlanUnassigned,
		UnassignedLabels:     effTracker.UnassignedLabels,
		MaxUnassignedIssues:  effTracker.MaxUnassignedIssues,
		PushLabels:           effTracker.PushLabels,
		TicketPatterns:       ticketPatterns,
		Projects:             projects,
		StatusMapping:        effTracker.StatusMapping,
		LabelStatusMapping:   effTracker.LabelStatusMapping,
	}

	return tracker.NewEngine(t, repo, engineCfg, config.HubDir())
}

// buildCredentialSource constructs a tracker.CredentialSource from the app's
// secret store and a resolved EffectiveTrackerConfig.
func buildCredentialSource(a *app.App, eff tracker.EffectiveTrackerConfig) tracker.CredentialSource {
	return tracker.NewCredentialSource(eff, a.Secrets)
}

// resolveTrackerProjects builds the Projects and TicketPatterns maps for the
// tracker sync engine by iterating over all hub projects.
//
// For each active project, it resolves the TrackerProject and TicketPattern
// using the 3-level cascade: project override → team default → old maps.
// Returns the final maps ready for the engine config.
func resolveTrackerProjects(ctx context.Context, a *app.App, eff tracker.EffectiveTrackerConfig) (projects, ticketPatterns map[string]string) {
	// Start with old maps for backward compat
	projects = eff.Projects
	if projects == nil {
		projects = make(map[string]string)
	}
	ticketPatterns = eff.TicketPatterns
	if ticketPatterns == nil {
		ticketPatterns = make(map[string]string)
	}

	// If old maps already have entries, use them as-is (backward compat)
	if len(projects) > 0 {
		// Apply default ticket pattern to projects missing one
		if eff.TicketPattern != "" {
			for k := range projects {
				if ticketPatterns[k] == "" {
					ticketPatterns[k] = eff.TicketPattern
				}
			}
		}
		return projects, ticketPatterns
	}

	// New approach: resolve from hub projects
	if eff.TrackerProject == "" {
		return projects, ticketPatterns
	}

	if a.Projects == nil {
		return projects, ticketPatterns
	}
	allProjects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil || len(allProjects) == 0 {
		return projects, ticketPatterns
	}

	for _, p := range allProjects {
		trackerProject := eff.TrackerProject // team default
		if p.TrackerConfig != nil && p.TrackerConfig.TrackerProject != "" {
			trackerProject = p.TrackerConfig.TrackerProject // per-project override
		}
		projects[p.ID] = trackerProject

		pattern := eff.TicketPattern
		if p.TrackerConfig != nil && p.TrackerConfig.TicketPattern != "" {
			pattern = p.TrackerConfig.TicketPattern
		}
		if pattern != "" {
			ticketPatterns[p.ID] = pattern
		}
	}

	slog.Debug("tracker.resolve_projects", "count", len(projects), "projects", projects)
	return projects, ticketPatterns
}
