package cmd

import (
	"context"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The views of the team: team mode, takeover briefs, team detail, MCP and
// models of the team.

// buildTeamModeView is the team mode: stats, sessions of the others, and the
// actions on the tracker and the board.
func buildTeamModeView(a *app.App, sessionsSection views.SessionsSectionConfig, startSection views.StartSectionConfig) views.View {
	return views.NewTeamModeView(views.TeamModeConfig{
		Sessions: sessionsSection,
		OnNavigate: func(viewID string) {
			if tuiShell != nil {
				tuiShell.NavigateTo(viewID)
			}
		},
		Start: startSection,
		OnExitTeamMode: func() {
			if tuiShell != nil {
				tuiShell.SetMode(views.ModeHub)
			}
		},
		TeamStats: func() views.TeamModeStats {
			resolveTeam := makeResolveTeamFunc(a)
			tr := resolveTeam()
			if !tr.Enabled {
				return views.TeamModeStats{}
			}
			repo := teamstate.NewRepo(tr.StateRepo, tr.StatePath)
			if !repo.IsCloned() {
				return views.TeamModeStats{}
			}
			members, _ := repo.ListMembers()
			tickets := views.FetchTeamTickets(repo, nil)
			active := 0
			for _, t := range tickets {
				if t.Status == "in_progress" || t.Status == "review" || t.Status == "validation" {
					active++
				}
			}
			// The sessions of the other members, read from the claims.
			var others []views.TeamMemberSession
			if claims, err := repo.ListClaims(""); err == nil {
				cfg, _ := repo.LoadConfig()
				others = views.OtherMemberSessions(claims, members, cfg, tr.MemberID, time.Now())
			}
			return views.TeamModeStats{
				MemberCount: len(members),
				ActiveCount: active,
				Others:      others,
			}
		},
		OnSyncTracker: func() {
			actionSyncTracker()
		},
		OnBoardConfig: func() {
			actionBoardColumnConfig()
		},
	})
}

// buildTeamDetailView is the configuration of the team: tracker, MCP, profile
// of the member, provider keys.
func buildTeamDetailView(a *app.App) views.View {
	return views.NewTeamDetailView(views.TeamDetailViewConfig{
		GetMCPConfig: func() config.MCPConfig {
			return a.Config.MCP
		},
		GetTrackerLocalConfig: func() config.TrackerLocalConfig {
			return a.Config.Tracker
		},
		ResolveTeam:      makeResolveTeamFunc(a),
		PromoteSolo:      tuiPromoteSolo,
		SaveTeamConfig:   func() views.TeamConfigWriter { return teamConfigWriter(a) },
		SaveLocalMCP:     func(key, value string) error { return nil },
		SaveLocalTracker: func(key, value string) error { return nil },
		GetSecrets: func() tracker.SecretGetter {
			return a.Secrets
		},
		GetHubConfig: func() *config.Config {
			return a.Config
		},
		UpdateHubConfig: func(fn func(*config.Config) error) error { return updateHubConfigOffLoop(a, fn) },
		ListProjects:    projectInfos(a),
		SyncTracker: func(ctx context.Context) (*views.SyncTrackerResult, error) {
			return runSyncTrackerForTUI(a, ctx)
		},
		CheckSecret:       secretChecker(a),
		TeamProviderKey:   teamProviderKey(a),
		SetSecret:         secretSetter(a),
		OnDiscoverTracker: actionTrackerDiscovery,
		GetMember: func(teamID string) *teamstate.Member {
			m, err := teamService(a).Profile(teamID)
			if err != nil {
				return nil
			}
			return m
		},
		SaveMemberField: func(ctx context.Context, teamID, field, value string) error {
			_, err := teamService(a).UpdateProfile(ctx, teamID, profilePatchForField(field, value))
			if err != nil {
				return profileError(err)
			}
			return nil
		},
	})
}

// buildTeamMCPView is the MCP services of the team.
func buildTeamMCPView(a *app.App) views.View {
	return views.NewTeamMCPView(views.TeamMCPViewConfig{
		ResolveTeam:     makeResolveTeamFunc(a),
		GetMCPConfig:    func() config.MCPConfig { return a.Config.MCP },
		SaveTeamConfig:  func() views.TeamConfigWriter { return teamConfigWriter(a) },
		UpdateHubConfig: func(fn func(*config.Config) error) error { return updateHubConfigOffLoop(a, fn) },
		CheckSecret:     secretChecker(a),
		SetSecret:       secretSetter(a),
		TestMCP:         func(service string) func(context.Context) (string, error) { return prepareMCPTest(a, service) },
	})
}

// buildTeamModelsView is the models recommended by the team.
func buildTeamModelsView(a *app.App) views.View {
	return views.NewTeamModelsView(views.TeamModelsViewConfig{
		ResolveTeam:    makeResolveTeamFunc(a),
		SaveTeamConfig: func() views.TeamConfigWriter { return teamConfigWriter(a) },
	})
}

// buildTakeoverView is the takeover briefs view, which enriches a brief
// through the agent.
func buildTakeoverView(a *app.App) views.View {
	takeoverView := views.NewTakeoverView(makeResolveTeamFunc(a))
	takeoverView.SetOnEnrich(func(project, ticketID string) error {
		return runTakeoverEnrich(a, project, ticketID)
	})
	return takeoverView
}

// projectInfos lists the projects for the team detail (all of them, whatever their status).
func projectInfos(a *app.App) func(ctx context.Context) []views.ProjectInfo {
	return func(ctx context.Context) []views.ProjectInfo {
		projects, err := a.Projects.List(ctx, "")
		if err != nil {
			return nil
		}
		result := make([]views.ProjectInfo, len(projects))
		for i, p := range projects {
			result[i] = views.ProjectInfo{ID: p.ID, Name: p.Name}
		}
		return result
	}
}

// teamProviderKey is the keychain key of a provider for the team of the shell, or of the active project.
func teamProviderKey(a *app.App) func(prov string) string {
	return func(prov string) string {
		teamID := ""
		if tuiShell != nil && tuiShell.ActiveTeam() != nil {
			teamID = tuiShell.ActiveTeam().ID
		} else if p, err := resolveActiveProject(a); err == nil {
			if tc := resolvedTeamConfig(a, p); tc.Enabled {
				teamID = tc.TeamID
			}
		}
		return provider.TeamKeychainKey(provider.Name(prov), teamID)
	}
}
