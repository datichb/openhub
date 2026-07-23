package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// tuiShell holds a reference to the running shell for action callbacks.
var tuiShell *shell.Shell

// runTUI launches the unified TUI shell.
func runTUI() error {
	a := MustApp()

	cfg := shell.Config{
		ProjectName: a.Config.Name,
		Commands:    buildCommands(a),
		Views:       buildViews(a),
		HomeViewID:  "home",
	}

	tuiShell = shell.New(cfg)
	tuiShell.NavigateHome(cfg.HomeViewID)
	return tuiShell.Run()
}

// buildCommands constructs the flat command registry for the omnibar.
func buildCommands(a *app.App) []shell.Command {
	commands := []shell.Command{
		// ── Sessions ─────────────────────────────────────────────────────
		{
			ID:          "start",
			Label:       "Start",
			Aliases:     []string{"session", "code", "launch"},
			Description: "Lancer une session opencode",
			Category:    "Sessions",
			Action:      actionStartLauncher,
		},
		{
			ID:          "start.dev",
			Label:       "Start Dev",
			Aliases:     []string{"dev", "ticket"},
			Description: "Session orientée développement",
			Category:    "Sessions",
			Action:      func() { launchOpencode("", "--dev") },
		},
		{
			ID:          "start.onboard",
			Label:       "Start Onboard",
			Aliases:     []string{"onboard", "onboarding"},
			Description: "Session d'onboarding projet",
			Category:    "Sessions",
			Action:      func() { launchOpencode("", "--onboard") },
		},
		{
			ID:          "audit",
			Label:       "Audit",
			Aliases:     []string{"audit"},
			Description: "Lancer un audit (sécurité, perf, archi...)",
			Category:    "Sessions",
			Action:      actionAuditLauncher,
		},
		{
			ID:          "audit.security",
			Label:       "Audit Sécurité",
			Aliases:     []string{"secu", "security", "owasp"},
			Description: "Audit de sécurité (OWASP, injections, auth)",
			Category:    "Sessions",
			Action:      func() { launchOpencode("auditor", "--type", "security") },
		},
		{
			ID:          "audit.performance",
			Label:       "Audit Performance",
			Aliases:     []string{"perf", "performance"},
			Description: "Audit performance (N+1, mémoire, CPU)",
			Category:    "Sessions",
			Action:      func() { launchOpencode("auditor", "--type", "performance") },
		},
		{
			ID:          "audit.architecture",
			Label:       "Audit Architecture",
			Aliases:     []string{"archi", "architecture"},
			Description: "Audit d'architecture (couplage, patterns)",
			Category:    "Sessions",
			Action:      func() { launchOpencode("auditor", "--type", "architecture") },
		},
		{
			ID:          "audit.accessibility",
			Label:       "Audit Accessibilité",
			Aliases:     []string{"a11y", "accessibility", "wcag"},
			Description: "Audit a11y (WCAG, ARIA, contraste)",
			Category:    "Sessions",
			Action:      func() { launchOpencode("auditor", "--type", "accessibility") },
		},
		{
			ID:          "audit.ecodesign",
			Label:       "Audit Éco-conception",
			Aliases:     []string{"eco", "ecodesign", "green"},
			Description: "Audit impact environnemental",
			Category:    "Sessions",
			Action:      func() { launchOpencode("auditor", "--type", "ecodesign") },
		},
		{
			ID:          "audit.observability",
			Label:       "Audit Observabilité",
			Aliases:     []string{"obs", "observability", "logs"},
			Description: "Audit logs, traces, métriques",
			Category:    "Sessions",
			Action:      func() { launchOpencode("auditor", "--type", "observability") },
		},
		{
			ID:          "review",
			Label:       "Review",
			Aliases:     []string{"rev", "cr"},
			Description: "Lancer une code review",
			Category:    "Sessions",
			Action:      actionReviewLauncher,
		},
		{
			ID:          "review.standard",
			Label:       "Review Standard",
			Aliases:     []string{},
			Description: "Code review classique",
			Category:    "Sessions",
			Action:      func() { launchOpencode("reviewer") },
		},
		{
			ID:          "review.adversarial",
			Label:       "Review Adversarial",
			Aliases:     []string{"adversarial"},
			Description: "Review adversariale (trouver les failles)",
			Category:    "Sessions",
			Action:      func() { launchOpencode("reviewer", "--mode", "adversarial") },
		},
		{
			ID:          "review.edge",
			Label:       "Review Edge Cases",
			Aliases:     []string{"edge"},
			Description: "Review orientée cas limites",
			Category:    "Sessions",
			Action:      func() { launchOpencode("reviewer", "--mode", "edge-case") },
		},
		{
			ID:          "review.complete",
			Label:       "Review Complète",
			Aliases:     []string{"complete", "all"},
			Description: "Review complète (tous les modes)",
			Category:    "Sessions",
			Action:      func() { launchOpencode("reviewer", "--mode", "all") },
		},
		{
			ID:          "debug",
			Label:       "Debug",
			Aliases:     []string{"dbg", "debugger"},
			Description: "Session de debug (décrivez le problème)",
			Category:    "Sessions",
			Action:      actionDebugLauncher,
		},
		{
			ID:          "quick",
			Label:       "Quick",
			Aliases:     []string{"q", "fast"},
			Description: "Lancer opencode directement",
			Category:    "Sessions",
			Action:      actionOpencode("", ""),
		},
		{
			ID:          "parallel",
			Label:       "Parallel",
			Aliases:     []string{"par", "multi"},
			Description: "Sessions parallèles",
			Category:    "Sessions",
			ViewID:      "parallel",
		},

		// ── Projets ──────────────────────────────────────────────────────
		{
			ID:          "board",
			Label:       "Board",
			Aliases:     []string{"kanban", "tasks"},
			Description: "Kanban du projet actif",
			Category:    "Projets",
			ViewID:      "board",
		},
		{
			ID:          "projects",
			Label:       "Projets",
			Aliases:     []string{"proj", "list"},
			Description: "Liste des projets",
			Category:    "Projets",
			ViewID:      "projects.list",
		},
		{
			ID:          "deploy",
			Label:       "Deploy",
			Aliases:     []string{"dep", "push"},
			Description: "Déployer agents/skills sur le projet actif",
			Category:    "Projets",
			Action:      actionDeploy,
		},
		{
			ID:          "sync",
			Label:       "Sync",
			Aliases:     []string{"synchronize"},
			Description: "Synchroniser tous les projets",
			Category:    "Projets",
			Action:      actionSync,
		},

		// ── Configuration ────────────────────────────────────────────────
		{
			ID:          "config",
			Label:       "Config",
			Aliases:     []string{"cfg", "settings", "hub"},
			Description: "Configuration du hub",
			Category:    "Configuration",
			ViewID:      "config",
		},
		{
			ID:          "models",
			Label:       "Models",
			Aliases:     []string{"mod", "model", "llm"},
			Description: "Configuration des modèles",
			Category:    "Configuration",
			ViewID:      "models",
		},
		{
			ID:          "provider",
			Label:       "Provider",
			Aliases:     []string{"prov", "api"},
			Description: "Configuration du provider LLM",
			Category:    "Configuration",
			ViewID:      "provider",
		},
		{
			ID:          "mcp",
			Label:       "MCP",
			Aliases:     []string{"servers"},
			Description: "Serveurs MCP",
			Category:    "Configuration",
			ViewID:      "mcp",
		},

		// ── Système ──────────────────────────────────────────────────────
		{
			ID:          "status",
			Label:       "Status",
			Aliases:     []string{"stat", "info"},
			Description: "État du système",
			Category:    "Système",
			ViewID:      "status",
		},
		{
			ID:          "doctor",
			Label:       "Doctor",
			Aliases:     []string{"doc", "health", "check"},
			Description: "Diagnostic de santé",
			Category:    "Système",
			ViewID:      "doctor",
		},
		{
			ID:          "metrics",
			Label:       "Métriques",
			Aliases:     []string{"met", "stats", "tokens"},
			Description: "Statistiques d'usage",
			Category:    "Système",
			ViewID:      "metrics",
		},
		{
			ID:          "plugins",
			Label:       "Plugins",
			Aliases:     []string{"plug", "extensions"},
			Description: "Gestion des plugins",
			Category:    "Système",
			ViewID:      "plugins",
		},
		{
			ID:          "upgrade",
			Label:       "Upgrade",
			Aliases:     []string{"up", "update"},
			Description: "Mettre à jour opencode",
			Category:    "Système",
			Action:      actionUpgrade,
		},
		{
			ID:          "help",
			Label:       "Help",
			Aliases:     []string{"?", "aide", "shortcuts"},
			Description: "Raccourcis et aide",
			Category:    "Système",
			ViewID:      "help",
		},

		// ── Navigation ───────────────────────────────────────────────────
		{
			ID:          "home",
			Label:       "Home",
			Aliases:     []string{"accueil", "welcome"},
			Description: "Retour à l'accueil",
			Category:    "Navigation",
			ViewID:      "home",
		},
		{
			ID:          "quit",
			Label:       "Quit",
			Aliases:     []string{"exit", "q"},
			Description: "Quitter le TUI",
			Category:    "Navigation",
			Action: func() {
				if tuiShell != nil {
					tuiShell.App().Stop()
				}
			},
		},
	}

	// ── Team commands (conditional) ──────────────────────────────────────
	if a.Config.Team.Enabled {
		teamCommands := []shell.Command{
			{
				ID:          "team.board",
				Label:       "Team Board",
				Aliases:     []string{"team kanban", "equipe"},
				Description: "Kanban d'équipe",
				Category:    "Team",
				ViewID:      "team.board",
			},
			{
				ID:          "team.status",
				Label:       "Team Status",
				Aliases:     []string{"team stat"},
				Description: "Statut de l'équipe",
				Category:    "Team",
				ViewID:      "team.status",
			},
			{
				ID:          "team.activity",
				Label:       "Team Activity",
				Aliases:     []string{"activite", "feed"},
				Description: "Activité récente de l'équipe",
				Category:    "Team",
				ViewID:      "team.activity",
			},
			{
				ID:          "team.briefs",
				Label:       "Takeover Briefs",
				Aliases:     []string{"takeover", "briefs"},
				Description: "Briefs de reprise de contexte",
				Category:    "Team",
				ViewID:      "takeover-briefs",
			},
			{
				ID:          "worktrees",
				Label:       "Worktrees",
				Aliases:     []string{"wt", "git worktree"},
				Description: "Gestion des git worktrees",
				Category:    "Team",
				ViewID:      "worktrees",
			},
			{
				ID:          "patterns",
				Label:       "Patterns",
				Aliases:     []string{"pat"},
				Description: "Patterns d'équipe",
				Category:    "Team",
				ViewID:      "patterns",
			},
			{
				ID:          "policies",
				Label:       "Policies",
				Aliases:     []string{"pol", "rules"},
				Description: "Politiques d'équipe",
				Category:    "Team",
				ViewID:      "policies",
			},
		}
		commands = append(commands, teamCommands...)
	} else {
		commands = append(commands, shell.Command{
			ID:          "team.init",
			Label:       "Team Init",
			Aliases:     []string{"team init", "initialiser"},
			Description: "Initialiser le mode équipe",
			Category:    "Team",
			Action:      actionTeamInit,
		})
		commands = append(commands, shell.Command{
			ID:          "worktrees",
			Label:       "Worktrees",
			Aliases:     []string{"wt", "git worktree"},
			Description: "Gestion des git worktrees",
			Category:    "Team",
			ViewID:      "worktrees",
		})
	}

	if a.Config.MCP.Gitlab.WriteEnabled {
		commands = append(commands, shell.Command{
			ID:          "review.publish",
			Label:       "Review Publish",
			Aliases:     []string{"publish", "mr"},
			Description: "Publier pour review (MR + notification)",
			Category:    "Sessions",
			Action:      func() { launchOpencode("reviewer", "--publish") },
		})
	}

	return commands
}

// buildViews constructs all registered views for the shell.
func buildViews(a *app.App) []views.View {
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
	}

	allViews := []views.View{
		views.NewHomeView(),
		views.NewBoardView(views.BoardViewConfig{}),
		views.NewTeamBoardView(views.TeamBoardViewConfig{}),
		views.NewParallelView(views.ParallelViewConfig{}),
		projectsView,
		views.NewTeamStatusView(a),
		views.NewActivityView(a),
		views.NewWorktreeView(a, views.WorktreeViewConfig{
			DeployProject: func(projectPath string) error {
				return runDeployForProject(a, &domain.Project{Path: projectPath})
			},
		}),
		views.NewStatusView(a),
		views.NewMetricsView(),
		views.NewDoctorView(a),
		views.NewConfigView(),
		views.NewModelsView(a),
		views.NewProviderView(a),
		views.NewMCPView(a, views.MCPViewConfig{
			OnUpdateProjectMCP: func(projectID, service, state string) {
				ctx := context.Background()
				project, err := a.Projects.Get(ctx, projectID)
				if err != nil {
					slog.Warn("failed to get project for MCP override", "id", projectID, "error", err)
					return
				}
				if project.MCPConfig == nil {
					project.MCPConfig = &domain.ProjectMCPConfig{}
				}
				// Update or insert the service override
				found := false
				for i, svc := range project.MCPConfig.Services {
					if svc.Name == service {
						if state == "inherit" {
							project.MCPConfig.Services[i].Enabled = nil
						} else {
							t := state == "enabled"
							project.MCPConfig.Services[i].Enabled = &t
						}
						found = true
						break
					}
				}
				if !found {
					svc := domain.ProjectMCPService{Name: service}
					if state != "inherit" {
						t := state == "enabled"
						svc.Enabled = &t
					}
					project.MCPConfig.Services = append(project.MCPConfig.Services, svc)
				}
				project.UpdatedAt = time.Now()
				if err := a.Projects.Update(ctx, project); err != nil {
					slog.Warn("failed to update project MCP override", "id", projectID, "error", err)
				}
			},
		}),
		views.NewPluginsView(),
		views.NewHelpView(),
	}

	if a.Config.Team.Enabled {
		takeoverView := views.NewTakeoverView(a)
		takeoverView.SetOnEnrich(func(project, ticketID string) error {
			return runTakeoverEnrich(a, project, ticketID)
		})
		allViews = append(allViews,
			views.NewPatternsView(a),
			views.NewPoliciesView(a),
			takeoverView,
		)
	}

	return allViews
}

// resolveActiveProject returns the currently selected project or the first one.
func resolveActiveProject(a *app.App) (*domain.Project, error) {
	ctx := context.Background()
	return resolveProject(ctx, a, "")
}

// resolveProviderCreds populates provider credentials into opts.
func resolveProviderCreds(a *app.App, project *domain.Project, opts *opencode.StartOpts) {
	provider := project.Provider
	if provider == "" {
		provider = a.Config.Opencode.DefaultProvider
	}
	opts.Provider = provider

	if a.Secrets == nil {
		return
	}

	ctx := context.Background()
	switch provider {
	case "bedrock":
		token, _ := a.Secrets.Get(ctx, fmt.Sprintf("bedrock-token-%s", project.ID))
		if token == "" {
			token, _ = a.Secrets.Get(ctx, "bedrock-token-default")
		}
		opts.BearerToken = token
		opts.AWSProfile = a.Config.Provider.Bedrock.AWSProfile
		opts.AWSRegion = a.Config.Provider.Bedrock.AWSRegion
	case "anthropic":
		key, _ := a.Secrets.Get(ctx, fmt.Sprintf("anthropic-api-key-%s", project.ID))
		if key == "" {
			key, _ = a.Secrets.Get(ctx, "anthropic-api-key-default")
		}
		opts.APIKey = key
	case "openrouter":
		key, _ := a.Secrets.Get(ctx, fmt.Sprintf("openrouter-api-key-%s", project.ID))
		if key == "" {
			key, _ = a.Secrets.Get(ctx, "openrouter-api-key-default")
		}
		opts.APIKey = key
	}
}
