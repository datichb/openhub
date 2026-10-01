package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// buildCommands constructs the flat command registry for the omnibar.
func buildCommands(a *app.App) []shell.Command {
	// Mode shortcuts for readability (ADR-032 Phase 3)
	modeSession := []views.Mode{views.ModeProject, views.ModeTeam} // sessions need a project, available from team via selection
	modeProject := []views.Mode{views.ModeProject}                 // project-scoped only
	modeTeam := []views.Mode{views.ModeTeam}                       // team-scoped only
	// nil = global (visible in all modes)

	commands := []shell.Command{
		// ── Sessions ─────────────────────────────────────────────────────
		{
			ID:          "coder",
			Label:       i18n.T("tui.pm.item.quick"),
			Aliases:     []string{"quick", "q", "session", "code", "launch", "start", "fast"},
			Description: i18n.T("tui.pm.item.quick_desc"),
			Category:    "Sessions",
			Priority:    80,
			Action:      actionOpencode("orchestrator", ""),
			RunsDirect:  true,
			Modes:       modeSession,
		},
		{
			ID:          "dev",
			Label:       i18n.T("tui.pm.item.start_dev"),
			Aliases:     []string{"start.dev", "ticket"},
			Description: i18n.T("tui.pm.item.start_dev_desc"),
			Category:    "Sessions",
			Priority:    80,
			Action:      func() { launchDevSession() },
			RunsDirect:  true,
			Modes:       modeSession,
		},
		{
			ID:          "audit",
			Label:       i18n.T("tui.pm.item.audit"),
			Aliases:     []string{"audit.security", "audit.performance", "audit.architecture", "audit.accessibility", "audit.ecodesign", "audit.observability", "secu", "security", "perf", "archi", "a11y", "eco", "obs"},
			Description: i18n.T("tui.pm.item.audit_desc"),
			Category:    "Sessions",
			Priority:    60,
			Action:      actionAuditLauncher,
			Modes:       modeSession,
		},
		{
			ID:          "review",
			Label:       i18n.T("tui.pm.item.review"),
			Aliases:     []string{"rev", "cr", "review.standard", "review.adversarial", "review.edge", "review.complete", "adversarial", "edge"},
			Description: i18n.T("tui.pm.item.review_desc"),
			Category:    "Sessions",
			Priority:    60,
			Action:      actionReviewLauncher,
			Modes:       modeSession,
		},
		{
			ID:          "debug",
			Label:       i18n.T("tui.pm.item.debug"),
			Aliases:     []string{"dbg", "debugger", "diag"},
			Description: i18n.T("tui.pm.item.debug_desc"),
			Category:    "Sessions",
			Priority:    60,
			Action:      actionDebugLauncher,
			Modes:       modeSession,
		},
		{
			ID:          "parallel",
			Label:       i18n.T("tui.pm.item.parallel"),
			Aliases:     []string{"par", "multi"},
			Description: i18n.T("tui.pm.item.parallel_desc"),
			Category:    "Sessions",
			Priority:    60,
			ViewID:      "parallel",
			Modes:       modeProject,
		},
		{
			ID:          "onboard",
			Label:       i18n.T("tui.pm.item.onboard"),
			Aliases:     []string{"start.onboard", "onboarding"},
			Description: i18n.T("tui.pm.item.onboard_desc"),
			Category:    "Sessions",
			Priority:    50,
			Action:      func() { launchSessionWithPrompt("onboarder", buildOnboardPromptForTUI()) },
			RunsDirect:  true,
			Modes:       modeSession,
		},

		// ── Projets ──────────────────────────────────────────────────────
		{
			ID:          "projects",
			Label:       i18n.T("tui.cmd.projects"),
			Aliases:     []string{"proj", "list"},
			Description: i18n.T("tui.cmd.projects.desc"),
			Category:    i18n.T("tui.category.projects"),
			Priority:    100,
			ViewID:      "projects.list",
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
			Label:       i18n.T("tui.cmd.models"),
			Aliases:     []string{"mod", "model", "llm"},
			Description: i18n.T("tui.cmd.models.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    50,
			ViewID:      "models",
		},
		{
			ID:          "provider",
			Label:       i18n.T("tui.cmd.provider"),
			Aliases:     []string{"prov", "api"},
			Description: i18n.T("tui.cmd.provider.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    50,
			ViewID:      "provider",
		},
		{
			ID:          "mcp",
			Label:       i18n.T("tui.cmd.mcp"),
			Aliases:     []string{"servers"},
			Description: i18n.T("tui.cmd.mcp.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    50,
			ViewID:      "mcp",
		},
		{
			ID:          "workflow",
			Label:       i18n.T("tui.cmd.workflow"),
			Aliases:     []string{"wf", "pipeline", "steps"},
			Description: i18n.T("tui.cmd.workflow.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    50,
			ViewID:      "workflow",
		},
		// team-detail moved into hasTeam block (ADR-032)

		// ── Système ──────────────────────────────────────────────────────
		{
			ID:          "settings",
			Label:       i18n.T("tui.cmd.settings"),
			Aliases:     []string{"config", "cfg", "settings", "hub", "hub config"},
			Description: i18n.T("tui.cmd.settings.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    90,
			ViewID:      "settings",
		},
		// project-config moved into hasProject block (ADR-032)
		{
			ID:          "secrets",
			Label:       i18n.T("tui.cmd.secrets"),
			Aliases:     []string{"tokens", "credentials", "keychain"},
			Description: i18n.T("tui.cmd.secrets.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    45,
			ViewID:      "secrets",
		},
		{
			ID:          "status",
			Label:       i18n.T("tui.cmd.status"),
			Aliases:     []string{"stat", "info"},
			Description: i18n.T("tui.cmd.status.desc"),
			Category:    i18n.T("tui.category.system"),
			Priority:    90,
			ViewID:      "status",
		},
		{
			ID:          "doctor",
			Label:       i18n.T("tui.cmd.doctor"),
			Aliases:     []string{"doc", "health", "check"},
			Description: i18n.T("tui.cmd.doctor.desc"),
			Category:    i18n.T("tui.category.system"),
			Priority:    40,
			ViewID:      "doctor",
		},
		{
			ID:          "metrics",
			Label:       i18n.T("tui.cmd.metrics"),
			Aliases:     []string{"met", "stats", "tokens"},
			Description: i18n.T("tui.cmd.metrics.desc"),
			Category:    i18n.T("tui.category.system"),
			Priority:    90,
			ViewID:      "metrics",
		},
		{
			ID:          "plugins",
			Label:       i18n.T("tui.cmd.plugins"),
			Aliases:     []string{"plug", "extensions"},
			Description: i18n.T("tui.cmd.plugins.desc"),
			Category:    i18n.T("tui.category.system"),
			Priority:    40,
			ViewID:      "plugins",
		},
		{
			ID:          "upgrade",
			Label:       i18n.T("tui.cmd.upgrade"),
			Aliases:     []string{"up", "update"},
			Description: i18n.T("tui.cmd.upgrade.desc"),
			Category:    i18n.T("tui.category.system"),
			Priority:    40,
			Action:      actionUpgrade,
		},
		{
			ID:          "notifications",
			Label:       i18n.T("tui.cmd.notifications"),
			Aliases:     []string{"notif", "logs", "messages", "toasts", "erreurs"},
			Description: i18n.T("tui.cmd.notifications.desc"),
			Category:    i18n.T("tui.category.system"),
			Priority:    30,
			ViewID:      "notifications",
		},
		{
			ID:          "help",
			Label:       i18n.T("tui.cmd.help"),
			Aliases:     []string{"?", "aide", "shortcuts"},
			Description: i18n.T("tui.cmd.help.desc"),
			Category:    i18n.T("tui.category.system"),
			Priority:    90,
			ViewID:      "help",
		},
		{
			ID:          "history.export",
			Label:       i18n.T("tui.cmd.history_export"),
			Aliases:     []string{"export history", "export sessions", "exporter"},
			Description: i18n.T("tui.cmd.history_export.desc"),
			Category:    i18n.T("tui.category.system"),
			Priority:    35,
			Action:      actionHistoryExport,
		},
		{
			ID:          "history.import",
			Label:       i18n.T("tui.cmd.history_import"),
			Aliases:     []string{"import history", "import sessions", "importer"},
			Description: i18n.T("tui.cmd.history_import.desc"),
			Category:    i18n.T("tui.category.system"),
			Priority:    35,
			Action:      actionHistoryImport,
		},

		// ── Navigation ───────────────────────────────────────────────────
		{
			ID:          "home",
			Label:       i18n.T("tui.cmd.home"),
			Aliases:     []string{"accueil", "welcome"},
			Description: i18n.T("tui.cmd.home.desc"),
			Category:    i18n.T("tui.category.navigation"),
			Priority:    100,
			ViewID:      "home",
		},
		{
			ID:          "project.mode",
			Label:       i18n.T("tui.cmd.project_mode"),
			Aliases:     []string{"projet", "project", "focus"},
			Description: i18n.T("tui.cmd.project_mode.desc"),
			Category:    i18n.T("tui.category.navigation"),
			Priority:    100,
			Modes:       []views.Mode{views.ModeHub, views.ModeTeam},
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
			Label:       i18n.T("tui.cmd.hub_mode"),
			Aliases:     []string{"hub", "complet", "retour"},
			Description: i18n.T("tui.cmd.hub_mode.desc"),
			Category:    i18n.T("tui.category.navigation"),
			Priority:    95,
			Modes:       []views.Mode{views.ModeTeam, views.ModeProject},
			Action: func() {
				if tuiShell != nil {
					tuiShell.SetMode(views.ModeHub)
				}
			},
		},
		{
			ID:          "quit",
			Label:       i18n.T("tui.cmd.quit"),
			Aliases:     []string{"exit", "q"},
			Description: i18n.T("tui.cmd.quit.desc"),
			Category:    i18n.T("tui.category.navigation"),
			Priority:    10,
			Action: func() {
				if tuiShell != nil {
					tuiShell.App().Stop()
				}
			},
		},
	}

	// ── Project commands — only registered when at least one project exists (ADR-032) ──
	hasProject := false
	if a.Projects != nil {
		projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
		hasProject = len(projects) > 0
	}
	if hasProject {
		commands = append(commands,
			shell.Command{
				ID:          "board",
				Label:       i18n.T("tui.cmd.project_board"),
				Aliases:     []string{"kanban", "tasks", "project board"},
				Description: i18n.T("tui.cmd.board.desc"),
				Category:    i18n.T("tui.category.projects"),
				Priority:    100,
				ViewID:      "board",
				Modes:       modeProject,
			},
			shell.Command{
				ID:          "board.init",
				Label:       i18n.T("tui.cmd.board_init"),
				Aliases:     []string{"beads init", "init board", "init tickets"},
				Description: i18n.T("tui.cmd.board_init.desc"),
				Category:    i18n.T("tui.category.projects"),
				Priority:    30,
				Action: func() {
					initBeadsForActiveProject(a)
				},
				Modes: modeProject,
			},
			shell.Command{
				ID:          "deploy",
				Label:       i18n.T("tui.cmd.deploy"),
				Aliases:     []string{"dep", "push"},
				Description: i18n.T("tui.cmd.deploy.desc"),
				Category:    i18n.T("tui.category.projects"),
				Priority:    30,
				Action:      actionDeploy,
				Modes:       modeProject,
			},
			shell.Command{
				ID:          "sync",
				Label:       i18n.T("tui.cmd.sync"),
				Aliases:     []string{"synchronize"},
				Description: i18n.T("tui.cmd.sync.desc"),
				Category:    i18n.T("tui.category.projects"),
				Priority:    30,
				Action:      actionSync,
				Modes:       modeProject,
			},
			shell.Command{
				ID:          "project-config",
				Label:       i18n.T("tui.cmd.project_config"),
				Aliases:     []string{"config projet", "project config", "projet config"},
				Description: i18n.T("tui.cmd.project_config.desc"),
				Category:    i18n.T("tui.category.configuration"),
				Priority:    46,
				ViewID:      "project.config",
				Modes:       modeProject,
			},
		)
	}

	// ── Team commands — only registered when a team is configured (ADR-032) ──
	hasTeam := a.Config.ActiveTeam().StateRepo != ""
	if hasTeam {
		commands = append(commands,
			shell.Command{
				ID:          "team-detail",
				Label:       i18n.T("tui.team.detail"),
				Aliases:     []string{"tracker", "sync", "team config", "team detail"},
				Description: i18n.T("tui.team.detail.desc"),
				Category:    i18n.T("tui.category.configuration"),
				Priority:    48,
				ViewID:      "team.detail",
				Modes:       modeTeam,
			},
			shell.Command{
				ID:          "team.discover",
				Label:       i18n.T("cmd.discovery.omnibar_label"),
				Aliases:     []string{"tracker discovery", "discover", "configurer tracker", "discovery"},
				Description: i18n.T("cmd.discovery.omnibar_desc"),
				Category:    i18n.T("tui.category.configuration"),
				Priority:    47,
				Action:      actionTrackerDiscovery,
				Modes:       modeTeam,
			},
			shell.Command{
				ID:          "team.board",
				Label:       i18n.T("tui.team.board"),
				Aliases:     []string{"team board", "team kanban", "equipe board"},
				Description: i18n.T("tui.team.board.desc"),
				Category:    i18n.T("tui.category.team"),
				Priority:    70,
				ViewID:      "team.board",
				Modes:       modeTeam,
			},
			shell.Command{
				ID:          "team.status",
				Label:       i18n.T("tui.team.status"),
				Aliases:     []string{"team stat", "status team"},
				Description: i18n.T("tui.team.status.desc"),
				Category:    i18n.T("tui.category.team"),
				Priority:    70,
				ViewID:      "team.status",
				Modes:       modeTeam,
			},
			shell.Command{
				ID:          "team.activity",
				Label:       i18n.T("tui.team.activity"),
				Aliases:     []string{"activite", "feed", "activity"}, //nolint:misspell // French search alias
				Description: i18n.T("tui.team.activity.desc"),
				Category:    i18n.T("tui.category.team"),
				Priority:    65,
				ViewID:      "team.activity",
				Modes:       modeTeam,
			},
			shell.Command{
				ID:          "team.briefs",
				Label:       i18n.T("tui.team.briefs"),
				Aliases:     []string{"takeover", "briefs", "reprises"},
				Description: i18n.T("tui.team.briefs.desc"),
				Category:    i18n.T("tui.category.team"),
				Priority:    60,
				ViewID:      "team.briefs",
				Modes:       modeTeam,
			},
			shell.Command{
				ID:          "team.patterns",
				Label:       i18n.T("tui.team.patterns"),
				Aliases:     []string{"pat", "patterns"},
				Description: i18n.T("tui.team.patterns.desc"),
				Category:    i18n.T("tui.category.team"),
				Priority:    50,
				ViewID:      "team.patterns",
				Modes:       modeTeam,
			},
			shell.Command{
				ID:          "team.policies",
				Label:       i18n.T("tui.team.policies"),
				Aliases:     []string{"pol", "rules", "policies"},
				Description: i18n.T("tui.team.policies.desc"),
				Category:    i18n.T("tui.category.team"),
				Priority:    50,
				ViewID:      "team.policies",
				Modes:       modeTeam,
			},
		)

		// Sync tracker (team-only action)
		commands = append(commands, shell.Command{
			ID:          "team.sync",
			Label:       i18n.T("tui.cmd.sync_tracker"),
			Aliases:     []string{"sync tracker", "sync-tracker", "synchroniser tracker"},
			Description: i18n.T("tui.cmd.sync_tracker.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    60,
			Action:      actionSyncTracker,
			Modes:       modeTeam,
		})

		// team.configure — only useful with a team configured (ADR-032)
		commands = append(commands, shell.Command{
			ID:          "team.configure",
			Label:       i18n.T("tui.team.configure"),
			Aliases:     []string{"team config", "team projet", "configurer equipe"},
			Description: i18n.T("tui.team.configure.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    50,
			Action:      actionTeamConfigure,
			Modes:       modeTeam,
		})

		commands = append(commands, shell.Command{
			ID:          "history.team",
			Label:       i18n.T("tui.cmd.history_team"),
			Aliases:     []string{"team history", "team sessions", "historique equipe"},
			Description: i18n.T("tui.cmd.history_team.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    55,
			ViewID:      "team.activity",
			Modes:       modeTeam,
		})
	} // end hasTeam

	// ── Team init — always visible (needed to create a team) ─────────
	commands = append(commands, shell.Command{
		ID:          "team.init",
		Label:       i18n.T("tui.team.init"),
		Aliases:     []string{"team init", "initialiser"},
		Description: i18n.T("tui.team.init.desc"),
		Category:    i18n.T("tui.category.team"),
		Priority:    40,
		Action:      actionTeamInit,
	})

	// ── Team rejoin — always visible (for reinstall recovery) ────────
	commands = append(commands, shell.Command{
		ID:          "team.rejoin",
		Label:       i18n.T("tui.team.rejoin"),
		Aliases:     []string{"team rejoin", "rejoindre", "rejoin"},
		Description: i18n.T("tui.team.rejoin.desc"),
		Category:    i18n.T("tui.category.team"),
		Priority:    41,
		Action:      actionTeamRejoin,
	})

	// ── Worktrees — always visible (git feature, not team-specific) ──
	commands = append(commands, shell.Command{
		ID:          "worktrees",
		Label:       i18n.T("tui.cmd.worktrees"),
		Aliases:     []string{"wt", "git worktree"},
		Description: i18n.T("tui.cmd.worktrees.desc"),
		Category:    i18n.T("tui.category.git"),
		Priority:    100,
		ViewID:      "worktrees",
	})

	if a.Config.MCP.Gitlab.WriteEnabled {
		commands = append(commands, shell.Command{
			ID:          "review.publish",
			Label:       i18n.T("tui.cmd.review_publish"),
			Aliases:     []string{"publish", "mr"},
			Description: i18n.T("tui.cmd.review_publish.desc"),
			Category:    "Sessions",
			Action:      func() { launchSessionWithPrompt("reviewer", "[PUBLISH] "+buildReviewPrompt("")) },
			RunsDirect:  true,
			Modes:       modeSession,
		})

		commands = append(commands, shell.Command{
			ID:          "review.feedback",
			Label:       "Review Feedback",
			Aliases:     []string{"feedback", "rf", "retours"},
			Description: "Intégrer les retours reviewer d'une MR GitLab",
			Category:    "Sessions",
			Action:      func() { actionReviewFeedback() },
			RunsDirect:  true,
			Modes:       modeSession,
		})
	}

	// ── Hub init — always visible (reconfigure hub) ─────────────────
	commands = append(commands, shell.Command{
		ID:          "init",
		Label:       i18n.T("tui.cmd.init"),
		Aliases:     []string{"init", "setup", "reconfigure", "configurer"},
		Description: i18n.T("tui.cmd.init.desc"),
		Category:    i18n.T("tui.category.hub"),
		Priority:    30,
		Action: func() {
			if tuiShell == nil {
				return
			}
			wizard := buildFirstRunInlineWizard(MustApp())
			tuiShell.PushView(wizard)
		},
	})

	// ── Project add — always visible ────────────────────────────────
	commands = append(commands, shell.Command{
		ID:          "project.add",
		Label:       i18n.T("tui.cmd.project_add"),
		Aliases:     []string{"project add", "add project", "nouveau projet"},
		Description: i18n.T("tui.cmd.project_add.desc"),
		Category:    i18n.T("tui.category.projects"),
		Priority:    45,
		Action:      actionProjectAdd,
	})

	return commands
}
