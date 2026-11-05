package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The views of the hub: home, board, teams, provider, MCP, settings, secrets.

// buildHomeView is the home of the hub: teams, projects, sessions and the start section.
func buildHomeView(a *app.App, sessionsSection views.SessionsSectionConfig, startSection views.StartSectionConfig) views.View {
	return views.NewHomeView(views.HomeViewConfig{
		Sessions: sessionsSection,
		HasProject: func() bool {
			projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
			return len(projects) > 0
		},
		ListTeams: func() []views.TeamEntry { return homeTeamEntries(a) },
		ListProjects: func() []views.ProjectEntry {
			projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
			entries := make([]views.ProjectEntry, 0, len(projects))
			for _, p := range projects {
				entry := views.ProjectEntry{ID: p.ID, Name: p.Name, Path: p.Path}
				entries = append(entries, entry)
			}
			return entries
		},
		OnSelectTeam: func(teamID, teamName string) {
			if tuiShell != nil {
				tuiShell.SetActiveTeam(&views.ActiveTeam{ID: teamID, Name: teamName})
				tuiShell.SetMode(views.ModeTeam)
			}
		},
		OnSelectProject: func(projectID, projectName, projectPath string) {
			if tuiShell != nil {
				branch := resolveGitBranch(projectPath)
				tuiShell.SetActiveProject(&views.ActiveProject{ID: projectID, Name: projectName, Path: projectPath, Branch: branch})
				tuiShell.SetMode(views.ModeProject)
			}
		},
		OnAddProject: func() {
			actionProjectAdd()
		},
		OnNewTeam: actionTeamNew,
		Start:     startSection,
		ToolLine:  tuiToolLine,
	})
}

// homeTeamEntries are the teams of the landing: the enabled ones that are not
// a solo space (it has no team features), the default team first, with the
// figures of their team-state when it is cloned. It reads files: call it off
// the event loop.
func homeTeamEntries(a *app.App) []views.TeamEntry {
	def := a.Config.DefaultTeam()
	teams := attachableTeams(a.Config)
	entries := make([]views.TeamEntry, 0, len(teams))
	for _, t := range teams {
		entry := views.TeamEntry{ID: t.ID, Name: t.DisplayName(), Default: def != nil && def.ID == t.ID}
		repo := teamstate.NewRepo(t.StateRepo, t.StatePath)
		if repo.IsCloned() {
			members, _ := repo.ListMembers(tuiContext())
			entry.MemberCount = len(members)
			tickets := views.FetchTeamTickets(tuiContext(), repo, nil)
			for _, tk := range tickets {
				if tk.Status == "in_progress" || tk.Status == "review" || tk.Status == "validation" {
					entry.ActiveCount++
				}
			}
		} else {
			entry.NotSynced = true
		}
		entries = append(entries, entry)
	}
	return entries
}

// buildBoardView is the board of the active project (beads tickets).
func buildBoardView(a *app.App) views.View {
	return views.NewBoardView(views.BoardViewConfig{
		Tickets: fetchBoardTicketsForPath(resolveActiveProjectPath(a)),
		RefreshFunc: func() []views.BoardTicket {
			return fetchBoardTicketsForPath(resolveActiveProjectPath(a))
		},
		RefreshRate: 5 * time.Second,
		ProjectPath: func() string {
			return resolveActiveProjectPath(a)
		},
		CheckInitialized: func() bool {
			path := resolveActiveProjectPath(a)
			if path == "" {
				return true
			}
			if err := beads.Available(); err != nil {
				return true
			}
			return beads.IsInitialized(path)
		},
		OnInitBeads: func() { initBeadsForActiveProject(a) },
		OnLinkTracker: func(ticketID, externalRef string) error {
			path := resolveActiveProjectPath(a)
			if path == "" {
				return errors.New(i18n.T("tui.views.no_active_project"))
			}
			return beads.LinkToTracker(path, ticketID, externalRef)
		},
		QuickActions: buildBoardQuickActions(a),
	})
}

// buildTeamsView lists the teams of the hub and syncs one of them.
func buildTeamsView(a *app.App) views.View {
	return views.NewTeamsView(views.TeamsViewDeps{
		Config:      a.Config,
		MemberNames: teamMemberNames,
		OnSync: func(teamID string) {
			tc := a.Config.FindTeam(teamID)
			if tc == nil || !tc.Enabled {
				return
			}
			repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
			if tuiShell != nil {
				tuiShell.ShowToast(i18n.Tf("tui.views.sync_in_progress", tc.DisplayName()), shell.ToastInfo)
			}
			safego.Go(func() {
				ctx := tuiShell.Context()
				err := repo.Pull(ctx)
				if err == nil || teamstate.IsPullWarning(err) {
					views.AfterTeamSync(ctx, repo.Path())
				}
				select {
				case <-ctx.Done():
					return
				default:
				}
				queueOnShell(tuiShell, func() {
					if err != nil && !teamstate.IsPullWarning(err) {
						tuiShell.ShowToast(i18n.T("tui.views.sync_failed")+err.Error(), shell.ToastError)
					} else {
						tuiShell.ShowToast(i18n.Tf("tui.views.sync_done", tc.DisplayName()), shell.ToastSuccess)
					}
				})
			})
		},
		OnUpdate:     func(fn func(*config.Config) error) error { return updateHubConfig(a, fn) },
		OnSelectTeam: func(teamID string) { pickTeam(a, teamID) },
		// n opens the window that starts a team: create, rejoin (the assistant
		// of « Rejoindre une équipe » does the repository, its access, the clone,
		// the card of the member and the state_path in hub.toml) or a solo space
		OnAdd:        actionTeamNew,
		OnOpen:       func(teamID string) { enterTeamMode(a, teamID) },
		OnSetDefault: func(teamID string) error { return setDefaultTeam(a, teamID) },
		OnArchive: func(teamID string, archive bool) (string, error) {
			if archive {
				return archiveTeam(a, teamID)
			}
			return "", restoreTeam(a, teamID)
		},
		OnRemove: func(teamID string) ([]string, string, error) {
			return removeTeam(context.Background(), a, teamID)
		},
		PrepareLeave: func(teamID string, removeMember bool) views.TeamLeave {
			env := tuiLeaveEnv(a) // on the loop: the copy of hub.toml is taken here
			opts := leaveOptions{RemoveMember: removeMember}
			return views.TeamLeave{
				Plan: func(ctx context.Context) []string {
					plan, err := planLeave(ctx, env, teamID)
					if err != nil {
						return []string{err.Error()}
					}
					return plan.Lines(opts)
				},
				Leave: func(ctx context.Context) ([]string, error) {
					rep, err := leaveTeam(ctx, env, teamID, opts)
					if rep == nil {
						return nil, err
					}
					return rep.Summary(teamID), err
				},
			}
		},
		ProjectsOf: func(teamID string) []string { return teamProjectNames(context.Background(), a, teamID) },
		OnNavigate: func(viewID string) {
			if tuiShell != nil {
				tuiShell.NavigateTo(viewID)
			}
		},
	})
}

// buildProviderView is the provider of the hub and its keys.
func buildProviderView(a *app.App) views.View {
	return views.NewProviderView(a, views.ProviderViewConfig{
		GetConfig:    func() *config.Config { return a.Config },
		UpdateConfig: func(fn func(*config.Config) error) error { return updateHubConfig(a, fn) },
		CheckSecret:  secretChecker(a),
		SetSecret:    secretSetter(a),
		DeleteSecret: func(ctx context.Context, key string) error {
			if a.Secrets == nil {
				return errors.New(i18n.T("tui.views.keychain_unavailable"))
			}
			return a.Secrets.Delete(ctx, key)
		},
	})
}

// buildMCPView is the MCP services of the hub.
func buildMCPView(a *app.App) views.View {
	return views.NewMCPView(a, views.MCPViewConfig{
		GetConfig:    func() *config.Config { return a.Config },
		EnforcedURLs: func() func() map[string]*views.MCPEnforcedURL { return tuiEnforcedURLs(a) },
		HostInfo:     tuiHostInfo,
		UpdateConfig: func(fn func(*config.Config) error) error { return updateHubConfig(a, fn) },
	})
}

// buildSettingsView is the configuration of the hub (hub.toml).
func buildSettingsView(a *app.App) views.View {
	return views.NewSettingsView(views.SettingsViewConfig{
		ToolVersion: detectedToolVersion,
		TrackerType: teamTrackerType,
		GetConfig: func() *config.Config {
			// Return the live config directly — no copy.
			// The SettingsView uses undo (reload from disk) instead of
			// working on a detached copy that could go stale.
			return a.Config
		},
		// Read hub.toml again without touching a.Config: the view copies it
		// in on the event loop, which owns a.Config.
		ReadConfig: config.Read,
		ReloadConfig: func() *config.Config {
			// Reload from disk after an external modification.
			_ = refreshSharedConfig(a)
			return a.Config
		},
		UpdateConfig: func(fn func(*config.Config) error) error { return updateHubConfigOffLoop(a, fn) },
		CheckSecret:  secretChecker(a),
		SetSecret:    secretSetter(a),
	})
}

// buildSecretsView lists the secrets of the keychain and the ones that are expected.
func buildSecretsView(a *app.App) views.View {
	return views.NewSecretsView(views.SecretsViewConfig{
		GetStore: func() *keychain.Store {
			if a.Secrets == nil {
				return nil
			}
			if ks, ok := a.Secrets.(*keychain.Store); ok {
				return ks
			}
			return nil
		},
		GetExpectedKeys: expectedSecrets(a),
		GetActiveProjectID: func() string {
			if p, _ := resolveActiveProject(a); p != nil {
				return p.ID
			}
			return ""
		},
		GetActiveProjectName: func() string {
			if p, _ := resolveActiveProject(a); p != nil {
				return p.Name
			}
			return ""
		},
	})
}

// expectedSecrets are the keys the keychain should hold: the tokens of the MCP services of the hub, and those of the active project.
func expectedSecrets(a *app.App) func() []views.ExpectedSecret {
	return func() []views.ExpectedSecret {
		var expected []views.ExpectedSecret
		// Global keys from hub.toml MCP config.
		if a.Config.MCP.Gitlab.Token != "" {
			expected = append(expected, views.ExpectedSecret{
				Key: a.Config.MCP.Gitlab.Token, Source: "mcp.gitlab", Scope: "global"})
		}
		if a.Config.MCP.Jira.Token != "" {
			expected = append(expected, views.ExpectedSecret{
				Key: a.Config.MCP.Jira.Token, Source: "mcp.jira", Scope: "global"})
		}
		if a.Config.MCP.Figma.Token != "" {
			expected = append(expected, views.ExpectedSecret{
				Key: a.Config.MCP.Figma.Token, Source: "mcp.figma", Scope: "global"})
		}
		if a.Config.MCP.Gslides.Token != "" {
			expected = append(expected, views.ExpectedSecret{
				Key: a.Config.MCP.Gslides.Token, Source: "mcp.gslides", Scope: "global"})
		}
		// Project-level keys.
		if p, _ := resolveActiveProject(a); p != nil && p.MCPConfig != nil {
			for _, svc := range p.MCPConfig.Services {
				if svc.TokenKey != "" {
					expected = append(expected, views.ExpectedSecret{
						Key:    svc.TokenKey,
						Source: fmt.Sprintf("projet %s → mcp.%s", p.Name, svc.Name),
						Scope:  p.ID,
					})
				}
			}
		}
		return expected
	}
}
