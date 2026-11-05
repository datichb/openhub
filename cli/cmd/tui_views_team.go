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
		OpenTeamDetail: openTeamDetail,
		Start:          startSection,
		OnExitTeamMode: func() {
			if tuiShell != nil {
				tuiShell.SetMode(views.ModeHub)
			}
		},
		TeamStats: func() views.TeamModeStats { return teamModeStats(a) },
		OnSyncTracker: func() {
			actionSyncTracker()
		},
		OnBoardConfig: func() {
			actionBoardColumnConfig(nil)
		},
		OnSwitchTeam: actionSwitchTeam,
		OnNewTeam:    actionTeamNew,
	})
}

// teamDetailView is the team detail view the shell was given (set when the
// views are built): openTeamDetail asks it for a section.
var teamDetailView *views.TeamDetailView

// openTeamDetail opens the team detail on a section (views.TeamSection*): the
// section is asked before the view is mounted, so that the first render puts
// the cursor on it.
func openTeamDetail(section string) {
	if tuiShell == nil {
		return
	}
	if teamDetailView != nil {
		teamDetailView.OpenSection(section)
	}
	tuiShell.NavigateTo("team.detail")
}

// buildTeamDetailView is the configuration of the team: tracker, MCP, profile
// of the member, provider keys.
func buildTeamDetailView(a *app.App) views.View {
	teamDetailView = views.NewTeamDetailView(views.TeamDetailViewConfig{
		GetMCPConfig: func() config.MCPConfig {
			return a.Config.MCP
		},
		GetTrackerLocalConfig: func() config.TrackerLocalConfig {
			return a.Config.Tracker
		},
		GetTeamTrackerConfig: func() config.TrackerLocalConfig {
			return a.Config.TeamTracker(makeResolveTeamFunc(a)().TeamID)
		},
		CanEnforce: func(ctx context.Context, teamID string) bool {
			tc := makeResolveTeamFunc(a)()
			if tc.StatePath == "" || tc.TeamID != teamID {
				return true
			}
			return teamstate.NewRepo(tc.StateRepo, tc.StatePath).CanEnforce(ctx, tc.MemberID)
		},
		GetTeamInfo: func(teamID string) views.TeamInfo {
			t := a.Config.FindTeam(teamID)
			if t == nil {
				return views.TeamInfo{}
			}
			return views.TeamInfo{Name: t.Name, StateRepo: t.StateRepo, StatePath: t.StatePath, MemberID: t.MemberID, Default: t.Default}
		},
		SetTeamField: func(teamID, field, value string) error {
			return applyTeamField(tuiContext(), a, teamID, field, value)
		},
		OnReconfigure: func(teamID string) {
			if t := a.Config.FindTeam(teamID); t != nil {
				reconfigureTeam(t)
			}
		},
		OnEditBoard:    func() { actionBoardColumnConfig(nil) },
		OnNewTeam:      actionTeamNew,
		ResolveTeam:    makeResolveTeamFunc(a),
		PromoteSolo:    tuiPromoteSolo,
		SaveTeamConfig: func() views.TeamConfigWriter { return teamConfigWriter(a) },
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
		GetMember: func(ctx context.Context, teamID string) *teamstate.Member {
			m, err := teamService(a).Profile(ctx, teamID)
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
	return teamDetailView
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

// newTeamStatusView is who works on what, with the names of the members and of
// the projects.
func newTeamStatusView(a *app.App) views.View {
	v := views.NewTeamStatusView(makeResolveTeamFunc(a))
	v.SetNames(hubNames(a))
	return v
}

// newActivityView is the activity of the team, with the names of the members
// and of the projects.
func newActivityView(a *app.App) views.View {
	v := views.NewActivityView(makeResolveTeamFunc(a))
	v.SetNames(hubNames(a))
	return v
}

// buildTakeoverView is the takeover briefs view, which enriches a brief
// through the agent.
func buildTakeoverView(a *app.App) views.View {
	takeoverView := views.NewTakeoverView(makeResolveTeamFunc(a))
	takeoverView.SetNames(hubNames(a))
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

// teamProviderKey is the keychain key of a provider for the team the TUI is on
// (ResolveTeam: the team of the shell, else the one of the active project,
// else the default team).
func teamProviderKey(a *app.App) func(prov string) string {
	resolve := makeResolveTeamFunc(a)
	return func(prov string) string {
		return provider.TeamKeychainKey(provider.Name(prov), resolve().TeamID)
	}
}

// teamModeStats reads the figures of the team mode from the team-state: the
// members, the claims alive on the board of the team, the sessions of the others.
func teamModeStats(a *app.App) views.TeamModeStats {
	resolveTeam := makeResolveTeamFunc(a)
	tr := resolveTeam()
	if !tr.Enabled {
		return views.TeamModeStats{}
	}
	repo := teamstate.NewRepo(tr.StateRepo, tr.StatePath)
	if !repo.IsCloned() {
		return views.TeamModeStats{}
	}
	ctx := tuiContext()
	members, _ := repo.ListMembers(ctx)
	// The sessions of the other members and the active tickets, read from
	// the claims; « active » are the active columns of the board of the team.
	var others []views.TeamMemberSession
	active := 0
	if claims, err := repo.ListClaims(ctx, ""); err == nil {
		cfg, _ := repo.LoadConfig(ctx)
		var board teamstate.BoardConfig
		if cfg != nil {
			board = cfg.Board
		}
		active = teamstate.CountActive(claims, board)
		others = views.OtherMemberSessions(claims, members, cfg, tr.MemberID, time.Now())
		if len(others) > 0 {
			names := readNames(ctx, a, nil)
			for i := range others {
				others[i].ProjectName = names.ProjectLabel(others[i].Project)
			}
		}
	}
	return views.TeamModeStats{
		MemberCount: len(members),
		ActiveCount: active,
		Others:      others,
	}
}
