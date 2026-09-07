package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
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
		return nil
	}
	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return nil
	}

	teamCfg, err := repo.LoadConfig()
	if err != nil || teamCfg.Tracker.Type == "" {
		return nil
	}

	// Merge shared team-state config with local hub.toml overrides.
	effTracker := tracker.ResolveTrackerConfig(
		&teamCfg.Tracker,
		a.Config.Tracker,
		resolveWriteEnabledForTracker(a, teamCfg),
	)
	if !effTracker.Enabled {
		return nil
	}

	credSrc := buildCredentialSource(a, teamCfg.MCP, &teamCfg.Tracker)
	cfg, err := tracker.ResolveCredentials(ctx, credSrc, tracker.Type(effTracker.Type))
	if err != nil {
		return nil
	}

	t, err := tracker.New(cfg)
	if err != nil {
		return nil
	}

	// Build a teamstate.TrackerConfig from the effective config for the engine.
	engineCfg := teamstate.TrackerConfig{
		Type:                 effTracker.Type,
		Enabled:              effTracker.Enabled,
		AutoSync:             effTracker.AutoSync,
		SyncIntervalMinutes:  effTracker.SyncIntervalMinutes,
		AutoPlanAssigned:     effTracker.AutoPlanAssigned,
		MaxAutoPlanPerMember: effTracker.MaxAutoPlanPerMember,
		PushLabels:           effTracker.PushLabels,
		TicketPatterns:       effTracker.TicketPatterns,
		Projects:             effTracker.Projects,
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
