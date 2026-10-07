package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/provider"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// domainProjectToViewItem converts a domain.Project to a views.ProjectItem.
func domainProjectToViewItem(p domain.Project) views.ProjectItem {
	var mcpOverrides map[string]string
	if p.MCPConfig != nil && len(p.MCPConfig.Services) > 0 {
		mcpOverrides = make(map[string]string, len(p.MCPConfig.Services))
		for _, svc := range p.MCPConfig.Services {
			switch {
			case svc.Enabled == nil:
				mcpOverrides[svc.Name] = "inherit"
			case *svc.Enabled:
				mcpOverrides[svc.Name] = "enabled"
			default:
				mcpOverrides[svc.Name] = "disabled"
			}
		}
	}
	return views.ProjectItem{
		ID:           p.ID,
		Name:         p.Name,
		Path:         p.Path,
		Language:     p.Language,
		Provider:     p.Provider,
		Model:        p.Model,
		Status:       string(p.Status),
		MCPOverrides: mcpOverrides,
	}
}

// loadProjectItems fetches all projects from the store and converts them to view items.
func loadProjectItems(store domain.ProjectStore) []views.ProjectItem {
	if store == nil {
		return nil
	}
	projects, err := store.List(context.Background(), "")
	if err != nil {
		slog.Warn("failed to list projects for view", "error", err)
		return nil
	}
	items := make([]views.ProjectItem, 0, len(projects))
	for _, p := range projects {
		items = append(items, domainProjectToViewItem(p))
	}
	return items
}

// buildViews constructs all registered views for the shell.
func buildViews(a *app.App, notifStore *shell.NotificationStore) []views.View {
	projectItems := loadProjectItems(a.Projects)
	tuiSess = newTUISessions(a)
	sessionsSection := tuiSess.sectionConfig()
	tuiStartWiring = newTUIStart(a)
	startSection := tuiStartWiring.sectionConfig()

	projectsView := views.NewProjectsView(views.ProjectsViewConfig{
		Projects:         projectItems,
		KnownMCPServices: []string{"figma", "gitlab", "gslides"},
		RefreshFunc: func() []views.ProjectItem {
			return loadProjectItems(a.Projects)
		},
	})
	if a.Projects != nil {
		projectsView.SetOnAdd(func(name, path string) {
			p := &domain.Project{
				ID:     generateProjectID(name),
				Name:   name,
				Path:   path,
				Status: domain.ProjectStatusActive,
			}
			if _, _, err := upsertProject(context.Background(), a.Projects, p); err != nil {
				slog.Warn("failed to create/update project", "error", err)
				if tuiShell != nil {
					tuiShell.ShowToastMsg("Project save failed: "+err.Error(), false)
				}
			}
		})
		projectsView.SetOnRemove(func(id string) {
			if err := a.Projects.Delete(context.Background(), id); err != nil {
				slog.Warn("failed to remove project", "error", err)
				if tuiShell != nil {
					tuiShell.ShowToastMsg("Project delete failed: "+err.Error(), false)
				}
			}
		})
		projectsView.SetOnConfigure(func(id string, cfg views.ProjectConfigUpdate) {
			ctx := context.Background()
			project, err := a.Projects.Get(ctx, id)
			if err != nil {
				slog.Warn("failed to get project for configure", "id", id, "error", err)
				return
			}
			project.Language = cfg.Language
			project.Provider = cfg.Provider
			project.Model = cfg.Model
			project.UpdatedAt = time.Now()

			// Persist per-project MCP overrides
			if len(cfg.MCPOverrides) > 0 {
				services := make([]domain.ProjectMCPService, 0, len(cfg.MCPOverrides))
				for svcName, state := range cfg.MCPOverrides {
					svc := domain.ProjectMCPService{Name: svcName}
					switch state {
					case "enabled":
						t := true
						svc.Enabled = &t
					case "disabled":
						f := false
						svc.Enabled = &f
						// "inherit" → nil (omit, let hub config win)
					}
					services = append(services, svc)
				}
				project.MCPConfig = &domain.ProjectMCPConfig{Services: services}
			} else {
				project.MCPConfig = nil
			}

			if err := a.Projects.Update(ctx, project); err != nil {
				slog.Warn("failed to update project config", "id", id, "error", err)
				if tuiShell != nil {
					tuiShell.ShowToastMsg("Config save failed: "+err.Error(), false)
				}
			}
		})
		projectsView.SetOnRename(func(id, newName string) {
			ctx := context.Background()
			project, err := a.Projects.Get(ctx, id)
			if err != nil {
				slog.Warn("failed to get project for rename", "id", id, "error", err)
				return
			}
			project.Name = newName
			project.UpdatedAt = time.Now()
			if err := a.Projects.Update(ctx, project); err != nil {
				slog.Warn("failed to rename project", "id", id, "error", err)
				if tuiShell != nil {
					tuiShell.ShowToastMsg("Rename failed: "+err.Error(), false)
				}
			}
		})
		projectsView.SetOnMove(func(id, newPath string) {
			ctx := context.Background()
			project, err := a.Projects.Get(ctx, id)
			if err != nil {
				slog.Warn("failed to get project for move", "id", id, "error", err)
				return
			}
			project.Path = newPath
			project.UpdatedAt = time.Now()
			if err := a.Projects.Update(ctx, project); err != nil {
				slog.Warn("failed to move project", "id", id, "error", err)
				if tuiShell != nil {
					tuiShell.ShowToastMsg("Move failed: "+err.Error(), false)
				}
			}
		})
		// Enter project mode from the projects list view
		projectsView.SetOnEnterProject(func(p *views.ActiveProject) {
			if tuiShell != nil {
				tuiShell.SetProjectMode(p)
			}
		})
		projectsView.SetOnInitBeads(func(id, name, path string) {
			initBeadsForProject(a, id, name, path)
		})
	}

	// ── Project mode view ────────────────────────────────────────────────────
	projectModeView := views.NewProjectModeView(views.ProjectModeConfig{
		Sessions: sessionsSection,
		Start:    startSection,
		OnNavigate: func(viewID string) {
			if tuiShell != nil {
				tuiShell.NavigateTo(viewID)
			}
		},
		OnExitProjectMode: func() {
			if tuiShell != nil {
				tuiShell.SetProjectMode(nil)
			}
		},
		ToolLine: tuiToolLine,
	})

	allViews := []views.View{
		views.NewHomeView(views.HomeViewConfig{
			Sessions: sessionsSection,
			HasProject: func() bool {
				projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
				return len(projects) > 0
			},
			ListTeams: func() []views.TeamEntry {
				teams := a.Config.Teams
				entries := make([]views.TeamEntry, 0, len(teams))
				for _, t := range teams {
					if !t.Enabled || t.StateRepo == "" {
						continue
					}
					entry := views.TeamEntry{ID: t.ID, Name: t.DisplayName()}
					repo := teamstate.NewRepo(t.StateRepo, t.StatePath)
					if repo.IsCloned() {
						members, _ := repo.ListMembers()
						entry.MemberCount = len(members)
						tickets := views.FetchTeamTickets(repo, nil)
						for _, tk := range tickets {
							if tk.Status == "in_progress" || tk.Status == "review" || tk.Status == "validation" {
								entry.ActiveCount++
							}
						}
					}
					entries = append(entries, entry)
				}
				return entries
			},
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
			Start: startSection,
		}),
		views.NewBoardView(views.BoardViewConfig{
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
		}),
		views.NewTeamBoardView(buildTeamBoardViewConfig(a)),
		views.NewTeamModeView(views.TeamModeConfig{
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
				return views.TeamModeStats{
					MemberCount: len(members),
					ActiveCount: active,
				}
			},
			OnSyncTracker: func() {
				actionSyncTracker()
			},
			OnBoardConfig: func() {
				actionBoardColumnConfig()
			},
		}),
		tuiSess.view,
		newWorkflowCatalogView(tuiStartWiring),
		projectsView,
		projectModeView,
		views.NewTeamStatusView(makeResolveTeamFunc(a)),
		views.NewActivityView(makeResolveTeamFunc(a)),
		views.NewWorktreeView(a, views.WorktreeViewConfig{
			OpenSession: func(path string) { openLaunchForm(a, tuiLaunchRequest{WorkflowID: "libre", Location: path}) },
		}),
		views.NewStatusView(a),
		views.NewMetricsView(views.MetricsViewConfig{
			AgentEvents: a.AgentEvents,
			Stats:       a.Stats,
		}),
		views.NewDoctorView(a),
		views.NewTeamsView(views.TeamsViewDeps{
			Config: a.Config,
			OnSync: func(teamID string) {
				tc := a.Config.FindTeam(teamID)
				if tc == nil || !tc.Enabled {
					return
				}
				repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
				if tuiShell != nil {
					tuiShell.ShowToast(i18n.Tf("tui.views.sync_in_progress", tc.DisplayName()), shell.ToastInfo)
				}
				go func() {
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
					tuiShell.App().QueueUpdateDraw(func() {
						if err != nil && !teamstate.IsPullWarning(err) {
							tuiShell.ShowToast(i18n.T("tui.views.sync_failed")+err.Error(), shell.ToastError)
						} else {
							tuiShell.ShowToast(i18n.Tf("tui.views.sync_done", tc.DisplayName()), shell.ToastSuccess)
						}
					})
				}()
			},
			OnSave: config.Save,
			OnNavigate: func(viewID string) {
				if tuiShell != nil {
					tuiShell.NavigateTo(viewID)
				}
			},
		}),
		views.NewModelsView(a),
		views.NewProviderView(a, views.ProviderViewConfig{
			GetConfig: func() *config.Config { return a.Config },
			SaveConfig: func(c *config.Config) error {
				if err := config.Save(c); err != nil {
					return err
				}
				config.Reset()
				newCfg, err := config.Load()
				if err == nil && newCfg != nil {
					*a.Config = *newCfg
				}
				return nil
			},
			CheckSecret: func(ctx context.Context, key string) (bool, string) {
				if a.Secrets == nil {
					return false, ""
				}
				val, err := a.Secrets.Get(ctx, key)
				if err != nil || val == "" {
					return false, ""
				}
				masked := "****"
				if len(val) > 4 {
					masked = "****" + val[len(val)-4:]
				}
				return true, masked
			},
			SetSecret: func(ctx context.Context, key, value string) error {
				if a.Secrets == nil {
					return errors.New(i18n.T("tui.views.keychain_unavailable"))
				}
				return a.Secrets.Set(ctx, key, value)
			},
			DeleteSecret: func(ctx context.Context, key string) error {
				if a.Secrets == nil {
					return errors.New(i18n.T("tui.views.keychain_unavailable"))
				}
				return a.Secrets.Delete(ctx, key)
			},
		}),
		views.NewMCPView(a, views.MCPViewConfig{
			GetConfig: func() *config.Config { return a.Config },
			SaveConfig: func(c *config.Config) error {
				if err := config.Save(c); err != nil {
					return err
				}
				config.Reset()
				newCfg, err := config.Load()
				if err == nil && newCfg != nil {
					*a.Config = *newCfg
				}
				return nil
			},
		}),
		views.NewHelpView(),
		views.NewNotificationsView(views.NotificationsViewConfig{
			FilePath:  shell.NotificationsFilePath(),
			ReadLastN: shell.ReadLastN,
		}),
	}

	// Inject team resolution into project mode view (must be after projectModeView is created).
	projectModeView.SetResolveTeam(makeResolveTeamFunc(a))

	// Team views — always registered; views handle "not configured" gracefully.
	takeoverView := views.NewTakeoverView(makeResolveTeamFunc(a))
	takeoverView.SetOnEnrich(func(project, ticketID string) error {
		return runTakeoverEnrich(a, project, ticketID)
	})
	allViews = append(allViews,
		views.NewPatternsView(makeResolveTeamFunc(a)),
		views.NewPoliciesView(makeResolveTeamFunc(a), a),
		views.NewWikiView(makeResolveTeamFunc(a)),
		takeoverView,
		views.NewTeamDetailView(views.TeamDetailViewConfig{
			GetMCPConfig: func() config.MCPConfig {
				return a.Config.MCP
			},
			GetTrackerLocalConfig: func() config.TrackerLocalConfig {
				return a.Config.Tracker
			},
			ResolveTeam: makeResolveTeamFunc(a),
			PromoteSolo: tuiPromoteSolo,
			SaveTeamConfig: func(ctx context.Context, cfg *teamstate.TeamConfig) error {
				project, _ := resolveActiveProject(a)
				tc := resolvedTeamConfig(a, project)
				if !tc.Enabled {
					return errors.New(i18n.T("tui.views.team_not_configured"))
				}
				repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
				if err := repo.SaveConfig(ctx, cfg); err != nil {
					return err
				}
				return nil
			},
			SaveLocalMCP:     func(key, value string) error { return nil },
			SaveLocalTracker: func(key, value string) error { return nil },
			GetSecrets: func() tracker.SecretGetter {
				return a.Secrets
			},
			GetHubConfig: func() *config.Config {
				return a.Config
			},
			ListProjects: func(ctx context.Context) []views.ProjectInfo {
				projects, err := a.Projects.List(ctx, "")
				if err != nil {
					return nil
				}
				result := make([]views.ProjectInfo, len(projects))
				for i, p := range projects {
					result[i] = views.ProjectInfo{ID: p.ID, Name: p.Name}
				}
				return result
			},
			SyncTracker: func(ctx context.Context) (*views.SyncTrackerResult, error) {
				return runSyncTrackerForTUI(a, ctx)
			},
			CheckSecret: func(ctx context.Context, key string) (bool, string) {
				if a.Secrets == nil {
					return false, ""
				}
				val, err := a.Secrets.Get(ctx, key)
				if err != nil || val == "" {
					return false, ""
				}
				masked := "****"
				if len(val) > 4 {
					masked = "****" + val[len(val)-4:]
				}
				return true, masked
			},
			TeamProviderKey: func(prov string) string {
				teamID := ""
				if tuiShell != nil && tuiShell.ActiveTeam() != nil {
					teamID = tuiShell.ActiveTeam().ID
				} else if p, err := resolveActiveProject(a); err == nil {
					if tc := resolvedTeamConfig(a, p); tc.Enabled {
						teamID = tc.TeamID
					}
				}
				return provider.TeamKeychainKey(provider.Name(prov), teamID)
			},
			SetSecret: func(ctx context.Context, key, value string) error {
				if a.Secrets == nil {
					return errors.New(i18n.T("tui.views.keychain_unavailable"))
				}
				return a.Secrets.Set(ctx, key, value)
			},
			OnDiscoverTracker: actionTrackerDiscovery,
		}),
		// Team sub-pages
		views.NewTeamMCPView(views.TeamMCPViewConfig{
			ResolveTeam:  makeResolveTeamFunc(a),
			GetMCPConfig: func() config.MCPConfig { return a.Config.MCP },
			SaveTeamConfig: func(ctx context.Context, cfg *teamstate.TeamConfig) error {
				project, _ := resolveActiveProject(a)
				tc := resolvedTeamConfig(a, project)
				if !tc.Enabled {
					return errors.New(i18n.T("tui.views.team_not_configured"))
				}
				repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
				return repo.SaveConfig(ctx, cfg)
			},
			GetHubConfig: func() *config.Config { return a.Config },
			CheckSecret: func(ctx context.Context, key string) (bool, string) {
				if a.Secrets == nil {
					return false, ""
				}
				val, err := a.Secrets.Get(ctx, key)
				if err != nil || val == "" {
					return false, ""
				}
				masked := "****"
				if len(val) > 4 {
					masked = "****" + val[len(val)-4:]
				}
				return true, masked
			},
			SetSecret: func(ctx context.Context, key, value string) error {
				if a.Secrets == nil {
					return errors.New(i18n.T("tui.views.keychain_unavailable"))
				}
				return a.Secrets.Set(ctx, key, value)
			},
		}),
		views.NewTeamModelsView(views.TeamModelsViewConfig{
			ResolveTeam: makeResolveTeamFunc(a),
			SaveTeamConfig: func(ctx context.Context, cfg *teamstate.TeamConfig) error {
				project, _ := resolveActiveProject(a)
				tc := resolvedTeamConfig(a, project)
				if !tc.Enabled {
					return errors.New(i18n.T("tui.views.team_not_configured"))
				}
				repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
				return repo.SaveConfig(ctx, cfg)
			},
		}),
		// Hub config view
		views.NewSettingsView(views.SettingsViewConfig{
			ToolVersion: v5ToolVersion,
			GetConfig: func() *config.Config {
				// Return the live config directly — no copy.
				// The SettingsView uses undo (reload from disk) instead of
				// working on a detached copy that could go stale.
				return a.Config
			},
			ReloadConfig: func() *config.Config {
				// Reload from disk (for undo/refresh operations).
				config.Reset()
				newCfg, err := config.Load()
				if err == nil && newCfg != nil {
					*a.Config = *newCfg
				}
				return a.Config
			},
			SaveConfig: func(c *config.Config) error {
				if err := config.Save(c); err != nil {
					return err
				}
				// Reload to pick up any side-effects of TOML serialization
				config.Reset()
				newCfg, err := config.Load()
				if err == nil && newCfg != nil {
					*a.Config = *newCfg
				}
				return nil
			},
			CheckSecret: func(ctx context.Context, key string) (bool, string) {
				if a.Secrets == nil {
					return false, ""
				}
				val, err := a.Secrets.Get(ctx, key)
				if err != nil || val == "" {
					return false, ""
				}
				masked := "****"
				if len(val) > 4 {
					masked = "****" + val[len(val)-4:]
				}
				return true, masked
			},
			SetSecret: func(ctx context.Context, key, value string) error {
				if a.Secrets == nil {
					return errors.New(i18n.T("tui.views.keychain_unavailable"))
				}
				return a.Secrets.Set(ctx, key, value)
			},
		}),
		// Project config view
		views.NewProjectConfigView(views.ProjectConfigViewConfig{
			GetProject: func() *domain.Project {
				p, _ := resolveActiveProject(a)
				if p == nil {
					return nil
				}
				cp := *p // copy
				return &cp
			},
			SaveProject: func(ctx context.Context, p *domain.Project) error {
				return a.Projects.Update(ctx, p)
			},
			ExecHints:   projectExecHints,
			WorkflowIDs: tuiWorkflowIDs,
		}),
		// Project sub-pages
		views.NewProjectMCPView(views.ProjectMCPViewConfig{
			GetProject: func() *domain.Project {
				p, _ := resolveActiveProject(a)
				if p == nil {
					return nil
				}
				cp := *p
				return &cp
			},
			SaveProject: func(ctx context.Context, p *domain.Project) error {
				return a.Projects.Update(ctx, p)
			},
			ResolveMCPSource: func(service, field string) (effective string, source string, locked bool) {
				project, _ := resolveActiveProject(a)
				tc := resolvedTeamConfig(a, project)

				// Load team shared MCP config
				var shared *teamstate.SharedMCPConfig
				var teamID string
				if tc.Enabled && tc.StatePath != "" {
					teamID = tc.TeamID
					repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
					if repo.IsCloned() {
						teamCfg, _ := repo.LoadConfig()
						if teamCfg != nil {
							if s, ok := teamCfg.MCP[service]; ok {
								shared = &s
							}
						}
					}
				}

				// Hub MCP config for this service
				hub := hubMCPServerConfig(a.Config, service)

				// Project MCP service
				var projectSvc *domain.ProjectMCPService
				if project != nil && project.MCPConfig != nil {
					for i, svc := range project.MCPConfig.Services {
						if svc.Name == service {
							projectSvc = &project.MCPConfig.Services[i]
							break
						}
					}
				}

				eff := mcpresolve.ResolveFull(shared, hub, projectSvc, teamID)

				switch field {
				case "enabled":
					src := ""
					if eff.EnabledEnforced {
						src = fmt.Sprintf("[%s: enforced]", teamID)
					}
					return strconv.FormatBool(eff.Enabled), src, eff.EnabledEnforced
				case "url":
					src := ""
					if eff.URLEnforced {
						src = fmt.Sprintf("[%s: enforced]", teamID)
					}
					return eff.URL, src, eff.URLEnforced
				default:
					return "", "", false
				}
			},
		}),
		newBricksView(a),
		views.NewProjectModelsView(views.ProjectModelsViewConfig{
			GetProject: func() *domain.Project {
				p, _ := resolveActiveProject(a)
				if p == nil {
					return nil
				}
				cp := *p
				return &cp
			},
			SaveProject: func(ctx context.Context, p *domain.Project) error {
				return a.Projects.Update(ctx, p)
			},
		}),
		// Secrets view
		views.NewSecretsView(views.SecretsViewConfig{
			GetStore: func() *keychain.Store {
				if a.Secrets == nil {
					return nil
				}
				if ks, ok := a.Secrets.(*keychain.Store); ok {
					return ks
				}
				return nil
			},
			GetExpectedKeys: func() []views.ExpectedSecret {
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
			},
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
		}),
	)

	return allViews
}

// hubMCPServerConfig maps a service name to the corresponding hub MCPServerConfig.
func hubMCPServerConfig(cfg *config.Config, service string) config.MCPServerConfig {
	switch service {
	case "gitlab":
		return cfg.MCP.Gitlab
	case "jira":
		return cfg.MCP.Jira
	case "figma":
		return cfg.MCP.Figma
	case "gslides":
		return cfg.MCP.Gslides
	default:
		return config.MCPServerConfig{}
	}
}

// resolveGitBranch returns the current git branch for a project path.
// Returns "" on any error (not a git repo, git not found, etc.).
func resolveGitBranch(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// tuiToolLine is the session tool line of the project mode header (cached
// version, detected when the TUI starts: no process on the event loop).
func tuiToolLine() string {
	if v, _ := v5Ver.Load().(string); v != "" {
		return "opencode " + v + " · opencode-v2"
	}
	if v5Err != nil {
		return i18n.T("cmd.v1.unsupported.doctor_name") + " ✗"
	}
	return ""
}
