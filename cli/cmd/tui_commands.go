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

// The modes in which a command shows (ADR-032 Phase 3); no mode = global.
var (
	// modeSession: sessions need a project, available from team via selection.
	modeSession = []views.Mode{views.ModeProject, views.ModeTeam}
	// modeProject: project-scoped only.
	modeProject = []views.Mode{views.ModeProject}
	// modeTeam: team-scoped only.
	modeTeam = []views.Mode{views.ModeTeam}
)

// buildCommands constructs the flat command registry for the omnibar: the
// fixed groups, then those that depend on a project or a team, then the
// commands that are always there.
func buildCommands(a *app.App) []shell.Command {
	commands := sessionCommands()
	commands = append(commands, hubCommands()...)
	commands = append(commands, systemCommands()...)
	commands = append(commands, navigationCommands()...)
	commands = append(commands, projectCommands(a)...)
	commands = append(commands, teamCommands(a)...)
	commands = append(commands, alwaysVisibleCommands()...)
	if a.Config.MCP.Gitlab.WriteEnabled {
		commands = append(commands, reviewPublishCommand())
	}
	return append(commands, hubSetupCommands()...)
}

// sessionCommands are the sessions, the catalogue and the cleanup.
func sessionCommands() []shell.Command {
	return []shell.Command{
		{
			ID:          "libre",
			Label:       i18n.T("tui.start.free"),
			Aliases:     []string{"coder", "session", "code", "free"},
			Description: i18n.T("tui.start.free_desc"),
			Category:    "Sessions",
			Priority:    80,
			Action:      actionFreeSession,
			RunsDirect:  true,
			Modes:       modeSession,
		},
		{
			ID:          "libre.agent",
			Label:       i18n.T("tui.launch.agent_cmd"),
			Aliases:     []string{"coder.agent", "coder agent", "libre agent", "free agent", "session agent"},
			Description: i18n.T("tui.launch.agent_cmd_desc"),
			Category:    "Sessions",
			Priority:    78,
			Action:      actionFreeSessionWithAgent,
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
			ID:          "cleanup",
			Label:       i18n.T("tui.cleanup.cmd"),
			Aliases:     []string{"nettoyage", "gc", "oh gc", "deploy-cleanup", "migrate", "garbage", "espace disque", "disk"},
			Description: i18n.T("tui.cleanup.cmd_desc"),
			Category:    i18n.T("tui.category.system"),
			Priority:    30,
			Action:      actionCleanup,
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
	}
}

// hubCommands are the projects and the configuration of the hub.
func hubCommands() []shell.Command {
	return []shell.Command{
		{
			ID:          "projects",
			Label:       i18n.T("tui.cmd.projects"),
			Aliases:     []string{"proj", "list"},
			Description: i18n.T("tui.cmd.projects.desc"),
			Category:    i18n.T("tui.category.projects"),
			Priority:    100,
			ViewID:      "projects.list",
		},

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
			ID:          "settings",
			Label:       i18n.T("tui.cmd.settings"),
			Aliases:     []string{"config", "cfg", "settings", "hub config"},
			Description: i18n.T("tui.cmd.settings.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    90,
			ViewID:      "settings",
		},
		{
			ID:          "secrets",
			Label:       i18n.T("tui.cmd.secrets"),
			Aliases:     []string{"tokens", "credentials", "keychain"},
			Description: i18n.T("tui.cmd.secrets.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    45,
			ViewID:      "secrets",
		},
	}
}

// systemCommands are the views of the system and the history import/export.
func systemCommands() []shell.Command {
	return []shell.Command{
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
			Aliases:     []string{"met", "stats", "usage"},
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
	}
}

// navigationCommands move between the modes and quit.
func navigationCommands() []shell.Command {
	return []shell.Command{
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
					tuiShell.RequestQuit() // through the v5 quit dialog
				}
			},
		},
	}
}

// projectCommands are the commands of a project, only registered when at
// least one project exists (ADR-032).
func projectCommands(a *app.App) []shell.Command {
	if a.Projects == nil {
		return nil
	}
	projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
	if len(projects) == 0 {
		return nil
	}
	return []shell.Command{
		{
			ID:          "board",
			Label:       i18n.T("tui.cmd.project_board"),
			Aliases:     []string{"kanban", "tasks", "project board"},
			Description: i18n.T("tui.cmd.board.desc"),
			Category:    i18n.T("tui.category.projects"),
			Priority:    100,
			ViewID:      "board",
			Modes:       modeProject,
		},
		{
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
		{
			ID:          "project-wiki",
			Label:       i18n.T("tui.cmd.project_wiki"),
			Aliases:     []string{"wiki", "wiki projet", "project wiki", "doc metrics"},
			Description: i18n.T("tui.cmd.project_wiki.desc"),
			Category:    i18n.T("tui.category.projects"),
			Priority:    40,
			ViewID:      "project.wiki",
			Modes:       modeProject,
		},
		{
			ID:          "project-config",
			Label:       i18n.T("tui.cmd.project_config"),
			Aliases:     []string{"config projet", "project config", "projet config"},
			Description: i18n.T("tui.cmd.project_config.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    46,
			ViewID:      "project.config",
			Modes:       modeProject,
		},
	}
}

// teamCommands are the commands of the team, only registered when a team is
// configured (ADR-032).
func teamCommands(a *app.App) []shell.Command {
	if a.Config.ActiveTeam().StateRepo == "" {
		return nil
	}
	commands := teamConfigCommands()
	commands = append(commands, teamPageCommands()...)
	return append(commands, teamActionCommands()...)
}

// teamConfigCommands are the configuration of the team and its tracker.
func teamConfigCommands() []shell.Command {
	return []shell.Command{
		{
			ID:          "team-detail",
			Label:       i18n.T("tui.team.detail"),
			Aliases:     []string{"tracker", "sync", "team detail", "profile", "profil"},
			Description: i18n.T("tui.team.detail.desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    48,
			ViewID:      "team.detail",
			Modes:       modeTeam,
		},
		{
			ID:          "team.discover",
			Label:       i18n.T("cmd.discovery.omnibar_label"),
			Aliases:     []string{"tracker discovery", "discover", "configurer tracker", "discovery"},
			Description: i18n.T("cmd.discovery.omnibar_desc"),
			Category:    i18n.T("tui.category.configuration"),
			Priority:    47,
			Action:      actionTrackerDiscovery,
			Modes:       modeTeam,
		},
	}
}

// teamPageCommands open the views of the team.
func teamPageCommands() []shell.Command {
	return []shell.Command{
		{
			ID:          "team.board",
			Label:       i18n.T("tui.team.board"),
			Aliases:     []string{"team board", "team kanban", "equipe board"},
			Description: i18n.T("tui.team.board.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    70,
			ViewID:      "team.board",
			Modes:       modeTeam,
		},
		{
			ID:          "team.status",
			Label:       i18n.T("tui.team.status"),
			Aliases:     []string{"team stat", "status team"},
			Description: i18n.T("tui.team.status.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    70,
			ViewID:      "team.status",
			Modes:       modeTeam,
		},
		{
			ID:          "team.activity",
			Label:       i18n.T("tui.team.activity"),
			Aliases:     []string{"activite", "feed", "activity"}, //nolint:misspell // French search alias
			Description: i18n.T("tui.team.activity.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    65,
			ViewID:      "team.activity",
			Modes:       modeTeam,
		},
		{
			ID:          "team.briefs",
			Label:       i18n.T("tui.team.briefs"),
			Aliases:     []string{"takeover", "briefs", "reprises"},
			Description: i18n.T("tui.team.briefs.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    60,
			ViewID:      "team.briefs",
			Modes:       modeTeam,
		},
		{
			ID:          "team.patterns",
			Label:       i18n.T("tui.team.patterns"),
			Aliases:     []string{"pat", "patterns"},
			Description: i18n.T("tui.team.patterns.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    50,
			ViewID:      "team.patterns",
			Modes:       modeTeam,
		},
		{
			ID:          "team.policies",
			Label:       i18n.T("tui.team.policies"),
			Aliases:     []string{"pol", "rules", "policies"},
			Description: i18n.T("tui.team.policies.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    50,
			ViewID:      "team.policies",
			Modes:       modeTeam,
		},
		{
			ID:          "team.wiki",
			Label:       i18n.T("tui.teamwiki.title"),
			Aliases:     []string{"wiki", "proposals", "pending", "team-state"},
			Description: i18n.T("tui.teamwiki.cmd_desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    55,
			ViewID:      "team.wiki",
			Modes:       modeTeam,
		},
	}
}

// teamActionCommands are the actions of the team: sync, configure, history.
func teamActionCommands() []shell.Command {
	return []shell.Command{
		{
			ID:          "team.sync",
			Label:       i18n.T("tui.cmd.sync_tracker"),
			Aliases:     []string{"sync tracker", "sync-tracker", "synchroniser tracker"},
			Description: i18n.T("tui.cmd.sync_tracker.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    60,
			Action:      actionSyncTracker,
			Modes:       modeTeam,
		},
		{
			ID:          "team.configure",
			Label:       i18n.T("tui.team.configure"),
			Aliases:     []string{"team config", "team projet", "configurer equipe"},
			Description: i18n.T("tui.team.configure.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    50,
			Action:      actionTeamConfigure,
			Modes:       modeTeam,
		},
		{
			ID:          "history.team",
			Label:       i18n.T("tui.cmd.history_team"),
			Aliases:     []string{"team history", "team sessions", "historique equipe"},
			Description: i18n.T("tui.cmd.history_team.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    55,
			ViewID:      "team.activity",
			Modes:       modeTeam,
		},
	}
}

// alwaysVisibleCommands are there with or without a team: creating one,
// rejoining one after a reinstall, and the worktrees (a git feature, not
// team-specific).
func alwaysVisibleCommands() []shell.Command {
	return []shell.Command{
		{
			ID:          "team.init",
			Label:       i18n.T("tui.team.init"),
			Aliases:     []string{"team init", "initialiser"},
			Description: i18n.T("tui.team.init.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    40,
			Action:      actionTeamInit,
		},
		{
			ID:          "team.rejoin",
			Label:       i18n.T("tui.team.rejoin"),
			Aliases:     []string{"team rejoin", "rejoindre", "rejoin"},
			Description: i18n.T("tui.team.rejoin.desc"),
			Category:    i18n.T("tui.category.team"),
			Priority:    41,
			Action:      actionTeamRejoin,
		},
		{
			ID:          "worktrees",
			Label:       i18n.T("tui.cmd.worktrees"),
			Aliases:     []string{"wt", "git worktree"},
			Description: i18n.T("tui.cmd.worktrees.desc"),
			Category:    i18n.T("tui.category.git"),
			Priority:    100,
			ViewID:      "worktrees",
		},
	}
}

// reviewPublishCommand is review --publish: merge request through the GitLab
// API (no LLM), in the suspended terminal. Registered when the GitLab write
// mode is on.
func reviewPublishCommand() shell.Command {
	return shell.Command{
		ID:          "review.publish",
		Label:       i18n.T("tui.cmd.review_publish"),
		Aliases:     []string{"publish", "mr"},
		Description: i18n.T("tui.cmd.review_publish.desc"),
		Category:    i18n.T("tui.category.workflows"),
		Action:      actionReviewPublish,
		RunsDirect:  true,
		Modes:       modeSession,
	}
}

// hubSetupCommands are the hub init (reconfigure the hub) and the project
// add, always visible.
func hubSetupCommands() []shell.Command {
	return []shell.Command{
		{
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
		},
		{
			ID:          "project.add",
			Label:       i18n.T("tui.cmd.project_add"),
			Aliases:     []string{"project add", "add project", "nouveau projet"},
			Description: i18n.T("tui.cmd.project_add.desc"),
			Category:    i18n.T("tui.category.projects"),
			Priority:    45,
			Action:      actionProjectAdd,
		},
	}
}

// workflowCommandPrefix prefixes the generated `run <workflow>` commands.
const workflowCommandPrefix = "run."

// registerWorkflowCommands replaces the `run <workflow>` commands of the
// omnibar by those of the catalogue. The terms of a command are
// the id of the workflow and its `aliases:` read in the YAML (resolved by
// the workflow service); an alias routed `with` an input (feature with
// tickets → implement-spec) gets its own command, which opens the form of
// that route.
func registerWorkflowCommands(a *app.App, list []workflowsvc.Summary) {
	sh := tuiShell
	if sh == nil {
		return
	}
	sh.Commands().ReplaceGroup(workflowCommandPrefix, workflowCommands(list, func(req tuiLaunchRequest) {
		req.ProjectID = activeScope().ProjectID
		openLaunchForm(a, req)
	}))
}

// workflowCommands are the omnibar commands of the catalogue; open opens a
// launch form.
func workflowCommands(list []workflowsvc.Summary, open func(tuiLaunchRequest)) []shell.Command {
	ids := map[string]bool{}
	for _, s := range list {
		ids[s.ID] = true
	}
	cmds := make([]shell.Command, 0, len(list))
	var routes []shell.Command
	for _, s := range list {
		id := s.ID
		desc := s.Description
		if desc == "" {
			desc = s.Label
		}
		// The « libre » command of the Sessions category owns the terms of the
		// free session (libre, coder, free…): a term has a single meaning.
		own := id != freeWorkflow
		terms := []string{}
		if own {
			terms = append(terms, id)
		}
		for _, al := range s.Aliases {
			if ids[al.ID] {
				continue // a workflow of that id wins (alias_shadowed)
			}
			if al.With == "" {
				if own {
					terms = append(terms, al.ID)
				}
				continue
			}
			alias, with := al.ID, al.With
			routes = append(routes, shell.Command{
				ID:          workflowCommandPrefix + alias + "." + id,
				Label:       i18n.Tf("tui.launch.alias_command", alias, id, with),
				Aliases:     []string{alias + " " + with},
				Description: desc,
				Category:    i18n.T("tui.category.workflows"),
				Priority:    84,
				Action:      func() { open(tuiLaunchRequest{WorkflowID: alias, AliasTarget: id}) },
			})
		}
		cmds = append(cmds, shell.Command{
			ID:          workflowCommandPrefix + id,
			Label:       "run " + id,
			Aliases:     terms,
			Description: desc,
			Category:    i18n.T("tui.category.workflows"),
			Priority:    85,
			Action:      func() { open(tuiLaunchRequest{WorkflowID: id}) },
		})
	}
	return append(cmds, routes...)
}

// actionReviewPublish runs `oh mr create` on the active project with
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
