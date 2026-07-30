package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// tuiShell holds a reference to the running shell for action callbacks.
var tuiShell *shell.Shell

// runTUI launches the unified TUI shell.
// If projectName is non-empty, the TUI starts directly in project mode.
func runTUI() error {
	return runTUIWithProject("")
}

// runTUIWithProject launches the TUI, optionally activating project mode
// immediately for the given project name.
func runTUIWithProject(projectName string) error {
	a := MustApp()

	homeViewID := "home"

	// Resolve initial project if requested
	var initialProject *views.ActiveProject
	if projectName != "" && a.Projects != nil {
		p, err := a.Projects.GetByName(context.Background(), projectName)
		if err != nil || p == nil {
			return fmt.Errorf("projet introuvable: %q", projectName)
		}
		initialProject = &views.ActiveProject{
			ID:   p.ID,
			Name: p.Name,
			Path: p.Path,
		}
		homeViewID = "project.mode"
	}

	// Create the notification store upfront so it can be shared between the
	// shell (which populates it via showToast) and the NotificationsView.
	notifStore := shell.NewNotificationStore(50)

	builtViews := buildViews(a, notifStore)

	cfg := shell.Config{
		ProjectName:   a.Config.Name,
		Commands:      buildCommands(a),
		Views:         builtViews,
		HomeViewID:    homeViewID,
		Notifications: notifStore,
	}

	tuiShell = shell.New(cfg)

	// Pre-set the active project before navigating home so project.mode renders it
	if initialProject != nil {
		tuiShell.SetActiveProject(initialProject)
	}

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
			Priority:    80,
			Action:      actionStartLauncher,
		},
		{
			ID:          "start.dev",
			Label:       "Start Dev",
			Aliases:     []string{"dev", "ticket"},
			Description: "Session orientée développement",
			Category:    "Sessions",
			Priority:    80,
			Action:      func() { launchOpencode("", "--dev") },
			RunsDirect:  true,
		},
		{
			ID:          "start.onboard",
			Label:       "Start Onboard",
			Aliases:     []string{"onboard", "onboarding"},
			Description: "Session d'onboarding projet",
			Category:    "Sessions",
			Priority:    60,
			Action:      func() { launchOpencode("", "--onboard") },
			RunsDirect:  true,
		},
		{
			ID:          "audit",
			Label:       "Audit",
			Aliases:     []string{"audit"},
			Description: "Lancer un audit (sécurité, perf, archi...)",
			Category:    "Sessions",
			Priority:    60,
			Action:      actionAuditLauncher,
		},
		{
			ID:          "audit.security",
			Label:       "Audit Sécurité",
			Aliases:     []string{"secu", "security", "owasp"},
			Description: "Audit de sécurité (OWASP, injections, auth)",
			Category:    "Sessions",
			Priority:    55,
			Action:      func() { launchOpencode("auditor", "--type", "security") },
			RunsDirect:  true,
		},
		{
			ID:          "audit.performance",
			Label:       "Audit Performance",
			Aliases:     []string{"perf", "performance"},
			Description: "Audit performance (N+1, mémoire, CPU)",
			Category:    "Sessions",
			Priority:    55,
			Action:      func() { launchOpencode("auditor", "--type", "performance") },
			RunsDirect:  true,
		},
		{
			ID:          "audit.architecture",
			Label:       "Audit Architecture",
			Aliases:     []string{"archi", "architecture"},
			Description: "Audit d'architecture (couplage, patterns)",
			Category:    "Sessions",
			Priority:    55,
			Action:      func() { launchOpencode("auditor", "--type", "architecture") },
			RunsDirect:  true,
		},
		{
			ID:          "audit.accessibility",
			Label:       "Audit Accessibilité",
			Aliases:     []string{"a11y", "accessibility", "wcag"},
			Description: "Audit a11y (WCAG, ARIA, contraste)",
			Category:    "Sessions",
			Priority:    55,
			Action:      func() { launchOpencode("auditor", "--type", "accessibility") },
			RunsDirect:  true,
		},
		{
			ID:          "audit.ecodesign",
			Label:       "Audit Éco-conception",
			Aliases:     []string{"eco", "ecodesign", "green"},
			Description: "Audit impact environnemental",
			Category:    "Sessions",
			Priority:    55,
			Action:      func() { launchOpencode("auditor", "--type", "ecodesign") },
			RunsDirect:  true,
		},
		{
			ID:          "audit.observability",
			Label:       "Audit Observabilité",
			Aliases:     []string{"obs", "observability", "logs"},
			Description: "Audit logs, traces, métriques",
			Category:    "Sessions",
			Priority:    55,
			Action:      func() { launchOpencode("auditor", "--type", "observability") },
			RunsDirect:  true,
		},
		{
			ID:          "review",
			Label:       "Review",
			Aliases:     []string{"rev", "cr"},
			Description: "Lancer une code review",
			Category:    "Sessions",
			Priority:    60,
			Action:      actionReviewLauncher,
		},
		{
			ID:          "review.standard",
			Label:       "Review Standard",
			Aliases:     []string{},
			Description: "Code review classique",
			Category:    "Sessions",
			Priority:    55,
			Action:      func() { launchOpencode("reviewer") },
			RunsDirect:  true,
		},
		{
			ID:          "review.adversarial",
			Label:       "Review Adversarial",
			Aliases:     []string{"adversarial"},
			Description: "Review adversariale (trouver les failles)",
			Category:    "Sessions",
			Priority:    55,
			Action:      func() { launchOpencode("reviewer", "--mode", "adversarial") },
			RunsDirect:  true,
		},
		{
			ID:          "review.edge",
			Label:       "Review Edge Cases",
			Aliases:     []string{"edge"},
			Description: "Review orientée cas limites",
			Category:    "Sessions",
			Priority:    55,
			Action:      func() { launchOpencode("reviewer", "--mode", "edge-case") },
			RunsDirect:  true,
		},
		{
			ID:          "review.complete",
			Label:       "Review Complète",
			Aliases:     []string{"complete", "all"},
			Description: "Review complète (tous les modes)",
			Category:    "Sessions",
			Priority:    55,
			Action:      func() { launchOpencode("reviewer", "--mode", "all") },
			RunsDirect:  true,
		},
		{
			ID:          "debug",
			Label:       "Debug",
			Aliases:     []string{"dbg", "debugger"},
			Description: "Session de debug (décrivez le problème)",
			Category:    "Sessions",
			Priority:    80,
			Action:      actionDebugLauncher,
		},
		{
			ID:          "quick",
			Label:       "Quick",
			Aliases:     []string{"q", "fast"},
			Description: "Lancer opencode directement",
			Category:    "Sessions",
			Priority:    60,
			Action:      actionOpencode("", ""),
			RunsDirect:  true,
		},
		{
			ID:          "parallel",
			Label:       "Parallel",
			Aliases:     []string{"par", "multi"},
			Description: "Sessions parallèles",
			Category:    "Sessions",
			Priority:    60,
			ViewID:      "parallel",
		},

		// ── Projets ──────────────────────────────────────────────────────
		{
			ID:          "board",
			Label:       "Board",
			Aliases:     []string{"kanban", "tasks"},
			Description: "Kanban du projet actif",
			Category:    "Projets",
			Priority:    100,
			ViewID:      "board",
		},
		{
			ID:          "board.init",
			Label:       "Init Board",
			Aliases:     []string{"beads init", "init board", "init tickets"},
			Description: "Initialiser le suivi des tickets (beads) pour le projet actif",
			Category:    "Projets",
			Priority:    30,
			Action: func() {
				initBeadsForActiveProject(a)
			},
		},
		{
			ID:          "projects",
			Label:       "Projets",
			Aliases:     []string{"proj", "list"},
			Description: "Liste des projets",
			Category:    "Projets",
			Priority:    100,
			ViewID:      "projects.list",
		},
		{
			ID:          "deploy",
			Label:       "Deploy",
			Aliases:     []string{"dep", "push"},
			Description: "Déployer agents/skills sur le projet actif",
			Category:    "Projets",
			Priority:    30,
			Action:      actionDeploy,
		},
		{
			ID:          "sync",
			Label:       "Sync",
			Aliases:     []string{"synchronize"},
			Description: "Synchroniser tous les projets",
			Category:    "Projets",
			Priority:    30,
			Action:      actionSync,
		},

		// ── Configuration ────────────────────────────────────────────────
		{
			ID:          "teams",
			Label:       i18n.T("tui.teams"),
			Aliases:     []string{"team", "equipe", "equipes", "teams"},
			Description: i18n.T("tui.teams.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    85,
			ViewID:      "teams",
		},
		{
			ID:          "models",
			Label:       "Models",
			Aliases:     []string{"mod", "model", "llm"},
			Description: "Configuration des modèles",
			Category:    "Configuration",
			Priority:    50,
			ViewID:      "models",
		},
		{
			ID:          "provider",
			Label:       "Provider",
			Aliases:     []string{"prov", "api"},
			Description: "Configuration du provider LLM",
			Category:    "Configuration",
			Priority:    50,
			ViewID:      "provider",
		},
		{
			ID:          "mcp",
			Label:       "MCP",
			Aliases:     []string{"servers"},
			Description: "Serveurs MCP",
			Category:    "Configuration",
			Priority:    50,
			ViewID:      "mcp",
		},
		{
			ID:          "team-detail",
			Label:       i18n.T("tui.team.detail"),
			Aliases:     []string{"tracker", "sync", "team config", "team detail"},
			Description: i18n.T("tui.team.detail.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    48,
			ViewID:      "team.detail",
		},

		// ── Système ──────────────────────────────────────────────────────
		{
			ID:          "settings",
			Label:       "Settings",
			Aliases:     []string{"config", "cfg", "settings", "hub", "hub config"},
			Description: "Configuration personnelle du hub",
			Category:    "Configuration",
			Priority:    90,
			ViewID:      "settings",
		},
		{
			ID:          "project-config",
			Label:       "Config Projet",
			Aliases:     []string{"config projet", "project config", "projet config"},
			Description: "Configuration du projet actif éditable ligne par ligne",
			Category:    "Configuration",
			Priority:    46,
			ViewID:      "project.config",
		},
		{
			ID:          "secrets",
			Label:       "Secrets & Tokens",
			Aliases:     []string{"tokens", "credentials", "keychain"},
			Description: "Gérer les secrets (tokens API) — global et par projet",
			Category:    "Configuration",
			Priority:    45,
			ViewID:      "secrets",
		},
		{
			ID:          "status",
			Label:       "Status",
			Aliases:     []string{"stat", "info"},
			Description: "État du système",
			Category:    "Système",
			Priority:    90,
			ViewID:      "status",
		},
		{
			ID:          "doctor",
			Label:       "Doctor",
			Aliases:     []string{"doc", "health", "check"},
			Description: "Diagnostic de santé",
			Category:    "Système",
			Priority:    40,
			ViewID:      "doctor",
		},
		{
			ID:          "metrics",
			Label:       "Métriques",
			Aliases:     []string{"met", "stats", "tokens"},
			Description: "Statistiques d'usage",
			Category:    "Système",
			Priority:    90,
			ViewID:      "metrics",
		},
		{
			ID:          "plugins",
			Label:       "Plugins",
			Aliases:     []string{"plug", "extensions"},
			Description: "Gestion des plugins",
			Category:    "Système",
			Priority:    40,
			ViewID:      "plugins",
		},
		{
			ID:          "upgrade",
			Label:       "Upgrade",
			Aliases:     []string{"up", "update"},
			Description: "Mettre à jour opencode",
			Category:    "Système",
			Priority:    40,
			Action:      actionUpgrade,
		},
		{
			ID:          "notifications",
			Label:       "Notifications",
			Aliases:     []string{"notif", "logs", "messages", "toasts", "erreurs"},
			Description: "Historique des notifications de cette session",
			Category:    "Système",
			Priority:    30,
			ViewID:      "notifications",
		},
		{
			ID:          "help",
			Label:       "Help",
			Aliases:     []string{"?", "aide", "shortcuts"},
			Description: "Raccourcis et aide",
			Category:    "Système",
			Priority:    90,
			ViewID:      "help",
		},

		// ── Navigation ───────────────────────────────────────────────────
		{
			ID:          "home",
			Label:       "Home",
			Aliases:     []string{"accueil", "welcome"},
			Description: "Retour à l'accueil",
			Category:    "Navigation",
			Priority:    100,
			ViewID:      "home",
		},
		{
			ID:          "project.mode",
			Label:       "Mode Projet",
			Aliases:     []string{"projet", "project", "focus"},
			Description: "Basculer vers le mode projet (Ctrl+T)",
			Category:    "Navigation",
			Priority:    100,
			Action: func() {
				if tuiShell == nil {
					return
				}
				if tuiShell.ActiveProject() != nil {
					tuiShell.NavigateTo("project.mode")
				} else {
					tuiShell.NavigateTo("projects.list")
				}
			},
		},
		{
			ID:          "hub.mode",
			Label:       "Mode Hub",
			Aliases:     []string{"hub", "complet", "retour"},
			Description: "Revenir au TUI complet (mode hub)",
			Category:    "Navigation",
			Priority:    95,
			Action: func() {
				if tuiShell != nil {
					tuiShell.SetProjectMode(nil)
				}
			},
		},
		{
			ID:          "quit",
			Label:       "Quit",
			Aliases:     []string{"exit", "q"},
			Description: "Quitter le TUI",
			Category:    "Navigation",
			Priority:    10,
			Action: func() {
				if tuiShell != nil {
					tuiShell.App().Stop()
				}
			},
		},
	}

	// ── Team commands — always registered ────────────────────────────────
	// Views handle the "team not configured" case gracefully at render time.
	// Worktrees is a git feature, not team-specific — always visible.
	commands = append(commands,
		shell.Command{
			ID:          "team.board",
			Label:       i18n.T("tui.team.board"),
			Aliases:     []string{"team board", "team kanban", "equipe board"},
			Description: i18n.T("tui.team.board.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    70,
			ViewID:      "team.board",
		},
		shell.Command{
			ID:          "team.status",
			Label:       i18n.T("tui.team.status"),
			Aliases:     []string{"team stat", "status team"},
			Description: i18n.T("tui.team.status.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    70,
			ViewID:      "team.status",
		},
		shell.Command{
			ID:          "team.activity",
			Label:       i18n.T("tui.team.activity"),
			Aliases:     []string{"activite", "feed", "activity"},
			Description: i18n.T("tui.team.activity.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    65,
			ViewID:      "team.activity",
		},
		shell.Command{
			ID:          "team.briefs",
			Label:       i18n.T("tui.team.briefs"),
			Aliases:     []string{"takeover", "briefs", "reprises"},
			Description: i18n.T("tui.team.briefs.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    60,
			ViewID:      "team.briefs",
		},
		shell.Command{
			ID:          "worktrees",
			Label:       "Worktrees",
			Aliases:     []string{"wt", "git worktree"},
			Description: "Gestion des git worktrees",
			Category:    "Git",
			Priority:    100,
			ViewID:      "worktrees",
		},
		shell.Command{
			ID:          "team.patterns",
			Label:       i18n.T("tui.team.patterns"),
			Aliases:     []string{"pat", "patterns"},
			Description: i18n.T("tui.team.patterns.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    50,
			ViewID:      "team.patterns",
		},
		shell.Command{
			ID:          "team.policies",
			Label:       i18n.T("tui.team.policies"),
			Aliases:     []string{"pol", "rules", "policies"},
			Description: i18n.T("tui.team.policies.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    50,
			ViewID:      "team.policies",
		},
		shell.Command{
			ID:          "team.init",
			Label:       i18n.T("tui.team.init"),
			Aliases:     []string{"team init", "initialiser"},
			Description: i18n.T("tui.team.init.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    40,
			Action:      actionTeamInit,
		},
	)

	// ── Sync tracker (global action) ────────────────────────────────────
	commands = append(commands, shell.Command{
		ID:          "team.sync",
		Label:       "Sync Tracker",
		Aliases:     []string{"sync tracker", "sync-tracker", "synchroniser tracker"},
		Description: "Synchroniser les claims avec le tracker externe",
		Category:    i18n.T("tui.category.team"),
		Priority:    60,
		Action:      actionSyncTracker,
	})

	if a.Config.MCP.Gitlab.WriteEnabled {
		commands = append(commands, shell.Command{
			ID:          "review.publish",
			Label:       "Review Publish",
			Aliases:     []string{"publish", "mr"},
			Description: "Publier pour review (MR + notification)",
			Category:    "Sessions",
			Action:      func() { launchOpencode("reviewer", "--publish") },
			RunsDirect:  true,
		})
	}

	// ── Team Configure (toujours disponible si un projet est actif) ──────
	commands = append(commands, shell.Command{
		ID:          "team.configure",
		Label:       i18n.T("tui.team.configure"),
		Aliases:     []string{"team config", "team projet", "configurer equipe"},
		Description: i18n.T("tui.team.configure.desc"),
		Category:    i18n.T("tui.category.team"),
		Priority:    50,
		Action:      actionTeamConfigure,
	})

	return commands
}

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
		views.NewHomeView(),
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
		views.NewMetricsView(),
		views.NewDoctorView(a),
		views.NewTeamsView(views.TeamsViewDeps{
			Config: a.Config,
			OnSync: func(teamID string) {
				// TODO: trigger team-state git pull for the specified team
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
		views.NewProviderView(a),
		views.NewMCPView(a, views.MCPViewConfig{}),
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
				if err := repo.SaveConfig(cfg); err != nil {
					return err
				}
				return repo.CommitAndPush(ctx, "config: update tracker", "config.toml")
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

// resolveActiveProject returns the currently selected project or the first one.
func resolveActiveProject(a *app.App) (*domain.Project, error) {
	ctx := context.Background()
	return resolveProject(ctx, a, "")
}

// makeResolveTeamFunc returns a ResolveTeamFunc for use in team views.
// It resolves the effective team config for the currently active project
// (project-level override → hub fallback) each time it is called.
func makeResolveTeamFunc(a *app.App) views.ResolveTeamFunc {
	return func() views.TeamResolution {
		project, _ := resolveActiveProject(a)
		tc := resolvedTeamConfig(a, project)
		return views.TeamResolution{
			Enabled:   tc.Enabled,
			StateRepo: tc.StateRepo,
			StatePath: tc.StatePath,
			MemberID:  tc.MemberID,
		}
	}
}

// findOpencodeOrToast checks that the opencode binary exists, shows a toast if not.
// Returns an error if the binary is not found.
func findOpencodeOrToast() (string, error) {
	bin, err := opencode.FindBinary()
	if err != nil && tuiShell != nil {
		tuiShell.ShowToast("opencode non trouvé", shell.ToastError)
	}
	return bin, err
}

// buildTeamBoardViewConfig builds the full TeamBoardViewConfig for the shell TUI,
// wiring the refresh, sync and action callbacks to the live team-state repo.
func buildTeamBoardViewConfig(a *app.App) views.TeamBoardViewConfig {
	ctx := context.Background()

	// resolveRepo is a helper that returns the active team repo (or nil).
	resolveRepo := func() *teamstate.Repo {
		project, _ := resolveActiveProject(a)
		tc := resolvedTeamConfig(a, project)
		if !tc.Enabled {
			return nil
		}
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		if !repo.IsCloned() {
			return nil
		}
		return repo
	}

	return views.TeamBoardViewConfig{
		RefreshRate: 5 * time.Second,
		RefreshFunc: func() []views.TeamTicket {
			repo := resolveRepo()
			if repo == nil {
				return nil
			}
			return fetchTeamTicketsV2(repo)
		},
		SyncFunc: func() error {
			repo := resolveRepo()
			if repo == nil {
				return nil
			}
			if err := repo.Pull(ctx); err != nil {
				return err
			}
			// Tracker sync is best-effort — don't block on errors.
			if engine := resolveTrackerEngine(ctx, a); engine != nil && engine.ShouldAutoSync() {
				_, _ = engine.Run(ctx)
			}
			return nil
		},
		Actions: &views.BoardActions{
			Members: func() []views.SelectOption {
				repo := resolveRepo()
				if repo == nil {
					return nil
				}
				members, err := repo.ListMembers()
				if err != nil {
					return nil
				}
				opts := make([]views.SelectOption, 0, len(members))
				for _, m := range members {
					opts = append(opts, views.SelectOption{
						Label: m.DisplayName,
						Value: m.ID,
					})
				}
				return opts
			},
			OnClaim: func(ticketID string) error {
				repo := resolveRepo()
				if repo == nil {
					return fmt.Errorf("team non configurée")
				}
				project, _ := resolveActiveProject(a)
				projectID := ""
				if project != nil {
					projectID = project.ID
				}
				memberID := a.Config.ActiveTeam().MemberID
				_, err := repo.CreateClaim(ctx, teamstate.Claim{
					TicketID:  ticketID,
					Project:   projectID,
					ClaimedBy: memberID,
					Status:    teamstate.ClaimStatusInProgress,
				})
				if err == teamstate.ErrClaimExists {
					return nil // idempotent
				}
				return err
			},
			OnRelease: func(ticketID string) error {
				repo := resolveRepo()
				if repo == nil {
					return fmt.Errorf("team non configurée")
				}
				project, _ := resolveActiveProject(a)
				projectID := ""
				if project != nil {
					projectID = project.ID
				}
				return repo.ReleaseClaim(ctx, projectID, ticketID)
			},
			OnTransfer: func(ticketID, toMember string) error {
				repo := resolveRepo()
				if repo == nil {
					return fmt.Errorf("team non configurée")
				}
				project, _ := resolveActiveProject(a)
				projectID := ""
				if project != nil {
					projectID = project.ID
				}
				return repo.TransferClaim(ctx, projectID, ticketID, toMember)
			},
			OnStatus: func(ticketID, newStatus string) error {
				repo := resolveRepo()
				if repo == nil {
					return fmt.Errorf("team non configurée")
				}
				project, _ := resolveActiveProject(a)
				projectID := ""
				if project != nil {
					projectID = project.ID
				}
				return repo.UpdateClaimStatus(ctx, projectID, ticketID, newStatus)
			},
		},
	}
}

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

	credSrc := buildCredentialSource(a, teamCfg.MCP)
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
func buildCredentialSource(a *app.App, sharedMCP map[string]teamstate.SharedMCPConfig) tracker.CredentialSource {
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

	return tracker.CredentialSource{
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
}

// initBeadsForActiveProject initialises beads for the currently active project.
// Shows a confirmation modal before proceeding.
func initBeadsForActiveProject(a *app.App) {
	if tuiShell == nil {
		return
	}
	path := resolveActiveProjectPath(a)
	if path == "" {
		tuiShell.ShowToastMsg("Aucun projet actif", false)
		return
	}
	// Resolve project ID for the prefix
	var id, name string
	if ap := tuiShell.ActiveProject(); ap != nil {
		id = ap.ID
		name = ap.Name
	} else if p, err := resolveActiveProject(a); err == nil && p != nil {
		id = p.ID
		name = p.Name
	}
	initBeadsForProject(a, id, name, path)
}

// initBeadsForProject shows a confirmation modal then initialises beads for the
// given project (path, id/name as prefix).
func initBeadsForProject(_ *app.App, id, name, path string) {
	if tuiShell == nil {
		return
	}
	if beads.IsInitialized(path) {
		tuiShell.ShowToastMsg("Board déjà initialisé pour "+name, true)
		tuiShell.NavigateTo("board")
		return
	}

	label := name
	if label == "" {
		label = path
	}

	tuiShell.ShowSelectModal(
		"Initialiser le board pour "+label+" ?",
		[]views.SelectOption{
			{Label: "Oui, initialiser beads", Value: "yes"},
			{Label: "Annuler", Value: "no"},
		}, "",
		func(value string) {
			if value != "yes" {
				return
			}
			prefix := id
			if prefix == "" {
				prefix = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
			}
			if err := beads.Init(path, prefix); err != nil {
				tuiShell.ShowToastMsg("Erreur init beads: "+err.Error(), false)
				return
			}
			tuiShell.ShowToastMsg("Board initialisé pour "+label, true)
			tuiShell.NavigateTo("board")
		},
	)
}

// fetchBoardTicketsForPath loads and normalises tickets from the beads system
// for the given project path. Returns nil (and no error) when bd is not installed,
// when the project has no tickets yet, or when beads is not initialized.
func fetchBoardTicketsForPath(projectPath string) []views.BoardTicket {
	if projectPath == "" {
		return nil
	}
	if err := beads.Available(); err != nil {
		return nil // bd not installed — board shows empty, no crash
	}
	if !beads.IsInitialized(projectPath) {
		return nil // no .beads/ — board will show the init invite screen
	}
	raw, err := beads.ListAll(projectPath)
	if err != nil {
		slog.Warn("board: failed to list beads tickets", "path", projectPath, "error", err)
		return nil
	}
	out := make([]views.BoardTicket, 0, len(raw))
	for _, t := range raw {
		// Epics are containers, not actionable items — exclude from the board.
		if strings.EqualFold(t.Type, "epic") {
			continue
		}
		out = append(out, views.BoardTicket{
			ID:       t.ID,
			Title:    t.Title,
			Status:   boardNormalizeStatus(t.Status),
			Priority: t.Priority,
			Type:     t.Type,
		})
	}
	return out
}

// boardNormalizeStatus maps beads status values to the board column statuses.
func boardNormalizeStatus(s string) string {
	switch strings.ToLower(s) {
	case "todo", "to_do", "backlog", "open":
		return "todo"
	case "in_progress", "in-progress", "doing", "wip":
		return "in_progress"
	case "review", "in_review", "in-review":
		return "review"
	case "done", "completed", "closed", "cancelled":
		return "done"
	case "blocked", "stuck":
		return "blocked"
	default:
		return "todo"
	}
}

// resolveActiveProjectPath returns the path of the active project.
// Prefers the shell's active project; falls back to the first registered project.
// Returns "" if no project is available (safe to call with a minimal App in tests).
func resolveActiveProjectPath(a *app.App) string {
	if tuiShell != nil {
		if ap := tuiShell.ActiveProject(); ap != nil {
			return ap.Path
		}
	}
	if a == nil || a.Projects == nil {
		return ""
	}
	p, err := resolveActiveProject(a)
	if err != nil || p == nil {
		return ""
	}
	return p.Path
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
