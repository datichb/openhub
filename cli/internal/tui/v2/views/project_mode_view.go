package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ProjectModeConfig holds the callbacks used by the project mode view.
type ProjectModeConfig struct {
	// OnLaunchSession is called when the user triggers a session action.
	// agent is the opencode agent name, extraArgs are additional CLI flags.
	OnLaunchSession func(project *ActiveProject, agent string, extraArgs ...string)
	// OnNavigate is called when the user wants to navigate to a sub-view.
	// viewID identifies the target view.
	OnNavigate func(viewID string)
	// OnExitProjectMode is called when the user toggles back to hub mode.
	OnExitProjectMode func()
}

// ProjectModeView is the simplified project-scoped TUI view.
// It occupies the full content area and shows the active project's context.
// All actions are accessible via the omnibar (filtered to project scope).
type ProjectModeView struct {
	cfg         ProjectModeConfig
	project     *ActiveProject
	shell       ShellAccess
	app         *tview.Application
	tv          *tview.TextView
	commands    []ContextCommand // cached contextual commands
	resolveTeam ResolveTeamFunc  // resolves effective team config for the active project
}

var _ View = (*ProjectModeView)(nil)
var _ CommandProvider = (*ProjectModeView)(nil)

// NewProjectModeView creates a new project mode view.
func NewProjectModeView(cfg ProjectModeConfig) *ProjectModeView {
	return &ProjectModeView{cfg: cfg}
}

// SetShell provides the shell reference.
func (v *ProjectModeView) SetShell(s ShellAccess) { v.shell = s }

// SetResolveTeam injects the team resolution function used to conditionally
// show team-related context commands when the active project has team enabled.
func (v *ProjectModeView) SetResolveTeam(fn ResolveTeamFunc) { v.resolveTeam = fn }

// ID returns the view identifier.
func (v *ProjectModeView) ID() string { return "project.mode" }

// Title returns the display title.
func (v *ProjectModeView) Title() string {
	if v.project != nil {
		return fmt.Sprintf("Projet · %s", v.project.Name)
	}
	return "Mode Projet"
}

// StatusHints returns keybinding hints for the omnibar (passive mode).
func (v *ProjectModeView) StatusHints() string {
	if v.project == nil {
		return "Aucun projet actif · Ctrl+T retour au hub"
	}
	return fmt.Sprintf(
		"%s  · Ctrl+P commandes · Ctrl+T mode hub",
		v.project.Name,
	)
}

// Mount builds the project mode view.
func (v *ProjectModeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	// Refresh project from shell each time the view is mounted
	if v.shell != nil {
		v.project = v.shell.ActiveProject()
	}
	v.commands = nil // invalidate cache

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	v.render()
	content.AddItem(v.tv, 0, 1, true)
}

// Unmount cleans up resources.
func (v *ProjectModeView) Unmount() {
	v.app = nil
	v.tv = nil
	v.commands = nil
}

// HandleKey processes view-specific key events.
func (v *ProjectModeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	return event
}

// ContextCommands returns contextual commands for the omnibar.
// These replace the global command registry when project mode is active.
func (v *ProjectModeView) ContextCommands() []ContextCommand {
	if v.commands != nil {
		return v.commands
	}
	if v.project == nil {
		return nil
	}

	p := v.project
	navigate := func(id string) func() {
		return func() {
			if v.cfg.OnNavigate != nil {
				v.cfg.OnNavigate(id)
			}
		}
	}
	launch := func(agent string, args ...string) func() {
		return func() {
			if v.cfg.OnLaunchSession != nil {
				v.cfg.OnLaunchSession(p, agent, args...)
			}
		}
	}
	// launchCmd builds a ContextCommand for session launches.
	// RunsDirect=true because OnLaunchSession calls SuspendAndExec, which
	// deadlocks when invoked from inside QueueUpdateDraw.
	launchCmd := func(id, label string, aliases []string, description, agent string, args ...string) ContextCommand {
		return ContextCommand{
			ID:          id,
			Label:       label,
			Aliases:     aliases,
			Description: description,
			Category:    "Sessions",
			Action:      launch(agent, args...),
			RunsDirect:  true,
		}
	}

	v.commands = []ContextCommand{
		// ── Sessions ────────────────────────────────────────────────────
		launchCmd("project.start", "Start Dev",
			[]string{"dev", "session", "code"},
			fmt.Sprintf("Session dev sur %s", p.Name),
			"", "--dev"),
		launchCmd("project.audit", "Audit",
			[]string{"audit", "secu", "perf"},
			fmt.Sprintf("Audit sur %s", p.Name),
			"auditor"),
		launchCmd("project.audit.security", "Audit Sécurité",
			[]string{"secu", "owasp"},
			"Audit sécurité (OWASP)",
			"auditor", "--type", "security"),
		launchCmd("project.audit.performance", "Audit Performance",
			[]string{"perf"},
			"Audit performance",
			"auditor", "--type", "performance"),
		launchCmd("project.review", "Review",
			[]string{"rev", "cr"},
			fmt.Sprintf("Code review sur %s", p.Name),
			"reviewer"),
		launchCmd("project.debug", "Debug",
			[]string{"dbg"},
			"Session de debug",
			""),
		// ── Vues du projet ──────────────────────────────────────────────
		{
			ID:          "project.board",
			Label:       "Board",
			Aliases:     []string{"kanban", "tasks", "tickets"},
			Description: fmt.Sprintf("Kanban de %s", p.Name),
			Category:    "Projet",
			Action:      navigate("board"),
		},
		{
			ID:          "project.metrics",
			Label:       "Métriques",
			Aliases:     []string{"stats", "tokens", "usage"},
			Description: fmt.Sprintf("Métriques de %s", p.Name),
			Category:    "Projet",
			Action:      navigate("metrics"),
		},
		{
			ID:          "project.config",
			Label:       "Config Projet",
			Aliases:     []string{"cfg", "settings", "config"},
			Description: fmt.Sprintf("Configuration de %s", p.Name),
			Category:    "Projet",
			Action:      navigate("projects.list"),
		},
		{
			ID:          "project.worktrees",
			Label:       "Worktrees",
			Aliases:     []string{"wt", "git worktree"},
			Description: fmt.Sprintf("Worktrees de %s", p.Name),
			Category:    "Projet",
			Action:      navigate("worktrees"),
		},
		{
			ID:          "project.status",
			Label:       "Statut",
			Aliases:     []string{"stat", "info", "health"},
			Description: fmt.Sprintf("Statut de %s", p.Name),
			Category:    "Projet",
			Action:      navigate("status"),
		},
		// ── Navigation ──────────────────────────────────────────────────
		{
			ID:          "project.hub",
			Label:       "Mode Hub",
			Aliases:     []string{"hub", "retour", "complet"},
			Description: "Revenir au TUI complet (mode hub)",
			Category:    "Navigation",
			Action: func() {
				if v.cfg.OnExitProjectMode != nil {
					v.cfg.OnExitProjectMode()
				}
			},
		},
	}

	// ── Team commands (conditional) ──────────────────────────────────────
	// Shown only when the active project has team features enabled.
	if v.resolveTeam != nil {
		if tc := v.resolveTeam(); tc.Enabled {
			navigate := func(viewID string) func() {
				return func() {
					if v.cfg.OnNavigate != nil {
						v.cfg.OnNavigate(viewID)
					}
				}
			}
			v.commands = append(v.commands,
				ContextCommand{
					ID:          "project.team.status",
					Label:       "Team Status",
					Aliases:     []string{"team stat", "equipe"},
					Description: "Statut de l'équipe pour ce projet",
					Category:    "Team",
					Action:      navigate("team.status"),
				},
				ContextCommand{
					ID:          "project.team.board",
					Label:       "Team Board",
					Aliases:     []string{"board", "kanban"},
					Description: "Kanban d'équipe",
					Category:    "Team",
					Action:      navigate("team.board"),
				},
				ContextCommand{
					ID:          "project.team.activity",
					Label:       "Team Activity",
					Aliases:     []string{"activite", "feed"},
					Description: "Activité récente de l'équipe",
					Category:    "Team",
					Action:      navigate("team.activity"),
				},
				ContextCommand{
					ID:          "project.team.briefs",
					Label:       "Takeover Briefs",
					Aliases:     []string{"takeover", "briefs"},
					Description: "Briefs de reprise de contexte",
					Category:    "Team",
					Action:      navigate("team.briefs"),
				},
				ContextCommand{
					ID:          "project.patterns",
					Label:       "Patterns",
					Aliases:     []string{"pat"},
					Description: "Patterns d'équipe",
					Category:    "Team",
					Action:      navigate("team.patterns"),
				},
				ContextCommand{
					ID:          "project.policies",
					Label:       "Policies",
					Aliases:     []string{"pol", "rules"},
					Description: "Politiques d'équipe",
					Category:    "Team",
					Action:      navigate("team.policies"),
				},
			)
		}
	}

	return v.commands
}

// render draws the project mode screen content.
func (v *ProjectModeView) render() {
	if v.tv == nil {
		return
	}

	if v.project == nil {
		v.tv.SetText(fmt.Sprintf(
			"\n  [red]Aucun projet actif[-]\n\n  Utilisez %sCtrl+T%s pour revenir au mode hub.",
			theme.ColorTag(theme.AccentHex), theme.TagColor,
		))
		return
	}

	p := v.project
	accent := theme.ColorTag(theme.AccentHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	var b strings.Builder

	// ── Header ──────────────────────────────────────────────────────────
	fmt.Fprintf(&b, "\n  %s◆ Mode Projet%s\n\n", accent, reset)
	fmt.Fprintf(&b, "  %s%-12s%s %s\n", accent, "Projet", reset, p.Name)
	fmt.Fprintf(&b, "  %s%-12s%s %s\n\n", muted, "Chemin", reset, p.Path)

	// ── Actions disponibles ─────────────────────────────────────────────
	fmt.Fprintf(&b, "  %sActions disponibles%s  (Ctrl+P pour rechercher)\n\n", accent, reset)

	type entry struct{ icon, label, desc string }
	entries := []entry{
		{"▶", "Start Dev", "Lancer une session de développement"},
		{"◉", "Audit", "Analyser le code (sécurité, perf, archi)"},
		{"◎", "Review", "Code review du projet"},
		{"◈", "Debug", "Session de debug"},
		{"⊞", "Board", "Kanban du projet"},
		{"⊟", "Métriques", "Statistiques d'utilisation"},
		{"⊛", "Config Projet", "Modifier la configuration du projet"},
		{"⊜", "Worktrees", "Gérer les git worktrees"},
		{"⊝", "Statut", "Santé et informations du projet"},
	}
	for _, e := range entries {
		fmt.Fprintf(&b, "  %s%s%s %-20s %s%s%s\n",
			accent, e.icon, reset,
			e.label,
			muted, e.desc, reset,
		)
	}

	fmt.Fprintf(&b, "\n  %sCtrl+T%s  basculer vers le mode hub  ·  %sCtrl+P%s  commandes\n",
		accent, reset, accent, reset)

	v.tv.SetText(b.String())
}
