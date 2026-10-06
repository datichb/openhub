package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
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
			Label:       i18n.T("tui.start.free"),
			Aliases:     []string{"session", "code", "free", "libre"},
			Description: i18n.T("tui.start.free_desc"),
			Category:    "Sessions",
			Priority:    80,
			Action:      actionFreeSession,
			RunsDirect:  true,
			Modes:       modeSession,
		},
		{
			ID:          "sessions",
			Label:       i18n.T("tui.sessions.title"),
			Aliases:     []string{"parallel", "par", "multi", "inbox", "à traiter", "decisions"},
			Description: i18n.T("tui.sessions.desc"),
			Category:    "Sessions",
			Priority:    70,
			ViewID:      "sessions",
		},
		{
			ID:          "bricks",
			Label:       i18n.T("tui.bricks.cmd"),
			Aliases:     []string{"briques", "agents", "skills", "catalogue des briques"},
			Description: i18n.T("tui.bricks.cmd_desc"),
			Category:    i18n.T("tui.category.workflows"),
			Priority:    60,
			ViewID:      "project.agents",
		},
		{
			ID:          "workflows",
			Label:       i18n.T("tui.catalog.title"),
			Aliases:     []string{"catalogue", "catalog", "wf", "workflow"},
			Description: i18n.T("tui.start.all_desc"),
			Category:    i18n.T("tui.category.workflows"),
			Priority:    75,
			ViewID:      "workflows",
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
					tuiShell.RequestQuit() // through the v5 quit dialog (I7)
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
			shell.Command{
				ID:          "team.wiki",
				Label:       "Wiki",
				Aliases:     []string{"wiki", "proposals", "pending"},
				Description: "Browse wiki pages and review pending proposals",
				Category:    i18n.T("tui.category.team"),
				Priority:    55,
				ViewID:      "team.wiki",
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
		// review --publish: merge request through the GitLab API (no LLM),
		// in the suspended terminal.
		commands = append(commands, shell.Command{
			ID:          "review.publish",
			Label:       i18n.T("tui.cmd.review_publish"),
			Aliases:     []string{"publish", "mr"},
			Description: i18n.T("tui.cmd.review_publish.desc"),
			Category:    i18n.T("tui.category.workflows"),
			Action:      actionReviewPublish,
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

// workflowCommandPrefix prefixes the generated `run <workflow>` commands.
const workflowCommandPrefix = "run."

// workflowCommandAliases are the former session commands, now aliases of
// their workflow (O15).
var workflowCommandAliases = map[string][]string{
	"ticket":          {"dev", "start.dev"},
	"feature":         {"start", "orchestrator"},
	"quick":           {"q", "fast"},
	"audit":           {"secu", "security", "perf", "archi", "a11y"},
	"review":          {"rev", "cr"},
	"debug":           {"dbg", "debugger", "diag"},
	"onboarding":      {"onboard", "start.onboard"},
	"review-feedback": {"feedback", "rf", "retours"},
}

// registerWorkflowCommands replaces the `run <workflow>` commands of the
// omnibar by those of the catalogue (P1-T24).
func registerWorkflowCommands(a *app.App, list []workflowsvc.Summary) {
	sh := tuiShell
	if sh == nil {
		return
	}
	cmds := make([]shell.Command, 0, len(list))
	for _, s := range list {
		id := s.ID
		desc := s.Description
		if desc == "" {
			desc = s.Label
		}
		cmds = append(cmds, shell.Command{
			ID:          workflowCommandPrefix + id,
			Label:       "run " + id,
			Aliases:     append([]string{id}, workflowCommandAliases[id]...),
			Description: desc,
			Category:    i18n.T("tui.category.workflows"),
			Priority:    85,
			Action: func() {
				openLaunchForm(a, tuiLaunchRequest{WorkflowID: id, ProjectID: activeScope().ProjectID})
			},
		})
	}
	sh.Commands().ReplaceGroup(workflowCommandPrefix, cmds)
}

// actionReviewPublish runs `oh review --publish` on the active project with
// the TUI suspended (questions and output in the terminal).
func actionReviewPublish() {
	if tuiShell == nil {
		return
	}
	c := &cobra.Command{Use: "publish"}
	c.Flags().String("project", "", "")
	c.Flags().String("reviewer", "", "")
	if p := tuiShell.ActiveProject(); p != nil {
		_ = c.Flags().Set("project", p.ID)
	}
	c.SetContext(tuiShell.Context())
	err := tuiShell.SuspendAndExec(func() error { return runReviewPublish(c) })
	if err != nil {
		tuiShell.ShowToast(err.Error(), shell.ToastError)
	}
}
