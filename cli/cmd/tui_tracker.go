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
		resolveWriteEnabledForTracker(a, teamCfg),
	)
	if !effTracker.Enabled {
		slog.Debug("tracker.resolve_engine.skip", "reason", "tracker disabled locally")
		return nil
	}

	credSrc := buildCredentialSource(a, teamCfg.MCP, &teamCfg.Tracker)
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
		PushLabels:           effTracker.PushLabels,
		TicketPatterns:       ticketPatterns,
		Projects:             projects,
		StatusMapping:        effTracker.StatusMapping,
	}

	return tracker.NewEngine(t, repo, engineCfg, config.HubDir())
}

// resolveWriteEnabledForTracker returns whether write ops are enabled for the
// tracker type specified in the team config.
func resolveWriteEnabledForTracker(a *app.App, teamCfg *teamstate.TeamConfig) bool {
	switch tracker.Type(teamCfg.Tracker.Type) {
	case tracker.TypeGitLab:
		return a.Config.MCP.Gitlab.WriteEnabled
	case tracker.TypeJira:
		return a.Config.MCP.Jira.WriteEnabled
	}
	return false
}

// buildCredentialSource constructs a tracker.CredentialSource from the app's
// MCP config, secret store, and (optionally) the team-state shared MCP config.
// This is the single place where hub.toml MCP settings and team-state shared
// settings are merged into the tracker package's credential abstraction.
//
// sharedMCP may be nil when no team-state config is available — the source
// degrades gracefully to hub.toml-only mode.
// trackerCfg may be nil — no tracker URL/token override is applied.
func buildCredentialSource(a *app.App, sharedMCP map[string]teamstate.SharedMCPConfig, trackerCfg *teamstate.TrackerConfig) tracker.CredentialSource {
	// Resolve effective MCP configs (local hub.toml merged with team-state recs).
	var sharedGitLab, sharedJira *teamstate.SharedMCPConfig
	if sharedMCP != nil {
		if g, ok := sharedMCP["gitlab"]; ok {
			sharedGitLab = &g
		}
		if j, ok := sharedMCP["jira"]; ok {
			sharedJira = &j
		}
	}
	effGitLab := tracker.ResolveMCPConfig(sharedGitLab, a.Config.MCP.Gitlab)
	effJira := tracker.ResolveMCPConfig(sharedJira, a.Config.MCP.Jira)

	src := tracker.CredentialSource{
		GitLabEnabled:      effGitLab.Enabled,
		GitLabTokenKey:     effGitLab.TokenKey,
		GitLabWriteEnabled: effGitLab.WriteEnabled,
		GitLabURL:          effGitLab.URL,
		JiraEnabled:        effJira.Enabled,
		JiraTokenKey:       effJira.TokenKey,
		JiraWriteEnabled:   effJira.WriteEnabled,
		JiraURL:            effJira.URL,
		Secrets:            a.Secrets,
	}

	// Apply tracker-specific overrides from team-state config
	if trackerCfg != nil {
		if trackerCfg.TrackerURL != "" {
			src.TrackerURL = trackerCfg.TrackerURL
		}
		tokenKey := trackerCfg.TrackerTokenKey
		if tokenKey == "" && trackerCfg.TrackerURL != "" && trackerCfg.Type != "" {
			// Default tracker token key when a dedicated URL is set
			tokenKey = "openhub.tracker." + trackerCfg.Type + ".token"
		}
		if tokenKey != "" {
			src.TrackerTokenKey = tokenKey
		}
	}

	return src
}

// resolveTrackerProjects builds the Projects and TicketPatterns maps for the
// tracker sync engine by iterating over all hub projects.
//
// For each active project, it resolves the TrackerProject and TicketPattern
// using the 3-level cascade: project override → team default → old maps.
// Returns the final maps ready for the engine config.
func resolveTrackerProjects(ctx context.Context, a *app.App, eff tracker.EffectiveTrackerConfig) (map[string]string, map[string]string) {
	// Start with old maps for backward compat
	projects := eff.Projects
	if projects == nil {
		projects = make(map[string]string)
	}
	ticketPatterns := eff.TicketPatterns
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
