package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)


// buildViews constructs all registered views for the shell.
func buildViews(a *app.App, notifStore *shell.NotificationStore) []views.View {
	var projectItems []views.ProjectItem
	if a.Projects != nil {
		projects, _ := a.Projects.List(context.Background(), "")
		for _, p := range projects {
			projectItems = append(projectItems, views.ProjectItem{
				ID:       p.ID,
				Name:     p.Name,
				Path:     p.Path,
				Language: p.Language,
				Provider: p.Provider,
				Model:    p.Model,
				Agents:   p.Agents,
				Status:   string(p.Status),
				MCPOverrides: func() map[string]string {
					if p.MCPConfig == nil || len(p.MCPConfig.Services) == 0 {
						return nil
					}
					m := make(map[string]string, len(p.MCPConfig.Services))
					for _, svc := range p.MCPConfig.Services {
						if svc.Enabled == nil {
							m[svc.Name] = "inherit"
						} else if *svc.Enabled {
							m[svc.Name] = "enabled"
						} else {
							m[svc.Name] = "disabled"
						}
					}
					return m
				}(),
			})
		}
	}

	projectsView := views.NewProjectsView(views.ProjectsViewConfig{
		Projects:         projectItems,
		AvailableAgents:  discoverAgents(),
		KnownMCPServices: []string{"figma", "gitlab", "gslides"},
	})
	if a.Projects != nil {
		projectsView.SetOnAdd(func(name, path string) {
			p := &domain.Project{Name: name, Path: path}
			if err := a.Projects.Create(context.Background(), p); err != nil {
				slog.Warn("failed to add project", "error", err)
			}
		})
		projectsView.SetOnRemove(func(id string) {
			if err := a.Projects.Delete(context.Background(), id); err != nil {
				slog.Warn("failed to remove project", "error", err)
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
			project.Agents = cfg.Agents
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
		OnLaunchSession: func(p *views.ActiveProject, agent string, extraArgs ...string) {
			if tuiShell == nil {
				return
			}
			if _, err := findOpencodeOrToast(); err != nil {
				return
			}
			proj := &domain.Project{ID: p.ID, Path: p.Path}
			opts := opencode.StartOpts{
				ProjectPath: p.Path,
				ProjectID:   p.ID,
				Agent:       agent,
				ExtraArgs:   extraArgs,
			}
			resolveProviderCreds(a, proj, &opts)
			err := tuiShell.SuspendAndExec(func() error {
				return opencode.Run(opts)
			})
			if err != nil {
				slog.Warn("opencode session ended with error", "error", err)
				tuiShell.ShowToast(fmt.Sprintf("Session: %s", err), shell.ToastWarning)
			} else {
				tuiShell.ShowToast("Session terminée", shell.ToastSuccess)
			}
		},
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
	})

	allViews := []views.View{
		views.NewHomeView(views.HomeViewConfig{}),
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
		}),
		views.NewTeamBoardView(buildTeamBoardViewConfig(a)),
		views.NewParallelView(views.ParallelViewConfig{}),
		projectsView,
		projectModeView,
		views.NewTeamStatusView(makeResolveTeamFunc(a)),
		views.NewActivityView(makeResolveTeamFunc(a)),
		views.NewWorktreeView(a, views.WorktreeViewConfig{
			DeployProject: func(projectPath string) error {
				return runDeployForProject(a, &domain.Project{Path: projectPath})
			},
		}),
		views.NewStatusView(a),
		views.NewMetricsView(views.MetricsViewConfig{
			AgentEvents: a.AgentEvents,
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
					tuiShell.ShowToast("Sync "+tc.DisplayName()+"...", shell.ToastInfo)
				}
				go func() {
					ctx := tuiShell.Context()
					err := repo.Pull(ctx)
					select {
					case <-ctx.Done():
						return
					default:
					}
					tuiShell.App().QueueUpdateDraw(func() {
						if err != nil && !teamstate.IsPullWarning(err) {
							tuiShell.ShowToast("Sync échouée: "+err.Error(), shell.ToastError)
						} else {
							tuiShell.ShowToast("Sync "+tc.DisplayName()+" terminée", shell.ToastSuccess)
						}
					})
				}()
			},
			OnSave: func(cfg *config.Config) {
				_ = config.Save(cfg)
			},
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
					return fmt.Errorf("keychain non disponible")
				}
				return a.Secrets.Set(ctx, key, value)
			},
			DeleteSecret: func(ctx context.Context, key string) error {
				if a.Secrets == nil {
					return fmt.Errorf("keychain non disponible")
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
		views.NewPluginsView(),
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
		views.NewPoliciesView(makeResolveTeamFunc(a)),
		takeoverView,
		views.NewTeamDetailView(views.TeamDetailViewConfig{
			GetMCPConfig: func() config.MCPConfig {
				return a.Config.MCP
			},
			GetTrackerLocalConfig: func() config.TrackerLocalConfig {
				return a.Config.Tracker
			},
			ResolveTeam: makeResolveTeamFunc(a),
			SaveTeamConfig: func(ctx context.Context, cfg *teamstate.TeamConfig) error {
				project, _ := resolveActiveProject(a)
				tc := resolvedTeamConfig(a, project)
				if !tc.Enabled {
					return fmt.Errorf("équipe non configurée")
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
			SetSecret: func(ctx context.Context, key, value string) error {
				if a.Secrets == nil {
					return fmt.Errorf("keychain non disponible")
				}
				return a.Secrets.Set(ctx, key, value)
			},
		}),
		// Hub config view
		views.NewSettingsView(views.SettingsViewConfig{
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
					return fmt.Errorf("secret store non disponible")
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
			Deploy: func(ctx context.Context, p *domain.Project) error {
				return runDeployForProject(a, p)
			},
			AllAgents: func() []string {
				return []string{
					"auditor", "auditor-subagent", "debugger", "designer",
					"developer", "developer-migrator", "developer-refactor",
					"documentarian", "onboarder", "orchestrator", "orchestrator-dev",
					"pathfinder", "planner", "reviewer",
				}
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
