package cmd

import (
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
)


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
			Label:       i18n.T("tui.cmd.project_board"),
			Aliases:     []string{"kanban", "tasks", "project board"},
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

	// ── Team commands — only registered when a team is configured ───────
	hasTeam := a.Config.ActiveTeam().StateRepo != ""
	if hasTeam {
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
	)

	// Sync tracker (team-only action)
	commands = append(commands, shell.Command{
		ID:          "team.sync",
		Label:       "Sync Tracker",
		Aliases:     []string{"sync tracker", "sync-tracker", "synchroniser tracker"},
		Description: "Synchroniser les claims avec le tracker externe",
		Category:    i18n.T("tui.category.team"),
		Priority:    60,
		Action:      actionSyncTracker,
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

	// ── Worktrees — always visible (git feature, not team-specific) ──
	commands = append(commands, shell.Command{
		ID:          "worktrees",
		Label:       "Worktrees",
		Aliases:     []string{"wt", "git worktree"},
		Description: "Gestion des git worktrees",
		Category:    "Git",
		Priority:    100,
		ViewID:      "worktrees",
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
