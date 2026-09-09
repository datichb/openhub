package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
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

// projectModeItem represents a navigable item in the project mode view.
type projectModeItem struct {
	Icon   string
	Label  string
	Desc   string
	Action func()
}

// ProjectModeView is the simplified project-scoped TUI view.
// It occupies the full content area and shows the active project's context.
// All actions are accessible via the omnibar (filtered to project scope).
type ProjectModeView struct {
	cfg         ProjectModeConfig
	project     *ActiveProject
	shell       ShellAccess
	app         *tview.Application
	list        *widgets.SectionedList
	dual        *homeDualLayout
	items       []projectModeItem
	resolveTeam ResolveTeamFunc // resolves effective team config for the active project
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
		return fmt.Sprintf("Aucun projet actif · Ctrl+T %s", i18n.T("tui.hints.hub_mode"))
	}
	return fmt.Sprintf(
		"%s · j/k %s · Enter %s · Ctrl+P %s · Ctrl+T %s",
		v.project.Name,
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.open"),
		i18n.T("tui.hints.commands"),
		i18n.T("tui.hints.hub_mode"),
	)
}

// Mount builds the project mode view.
func (v *ProjectModeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	// Refresh project from shell each time the view is mounted
	if v.shell != nil {
		v.project = v.shell.ActiveProject()
	}

	if v.project == nil {
		// No project active — show empty state
		tv := tview.NewTextView().
			SetDynamicColors(true).
			SetScrollable(false)
		tv.SetBackgroundColor(theme.BgPanel)
		tv.SetBorderPadding(1, 0, 2, 2)
		tv.SetText(fmt.Sprintf(
			"\n  [red]Aucun projet actif[-]\n\n  Utilisez %sCtrl+T%s pour revenir au mode hub.",
			theme.ColorTag(theme.AccentHex), theme.TagColor,
		))
		content.AddItem(tv, 0, 1, true)
		return
	}

	v.items = v.buildItems()

	// ── Header ──────────────────────────────────────────────────────────
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	header.SetBackgroundColor(theme.BgPanel)
	secondary := theme.ColorTag(theme.TextSecondaryHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	banner := renderBanner(v.project.Name, 100)
	header.SetText(fmt.Sprintf("\n%s\n  %s◆ Mode Projet%s\n  %s%s%s",
		banner,
		secondary, reset,
		muted, v.project.Path, reset,
	))
	headerHeight := bannerHeight(v.project.Name, 100) + 5

	// ── Footer ──────────────────────────────────────────────────────────
	accent := theme.ColorTag(theme.AccentHex)
	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(false)
	footer.SetBackgroundColor(theme.BgPanel)
	footer.SetText(fmt.Sprintf("\n%s%sCtrl+T%s mode hub  %sCtrl+P%s commandes  %s?%s aide",
		muted, accent, reset, accent, reset, accent, reset,
	))

	// ── Split items for dual-column: left = Sessions+Board, right = Config+Team+Hub ──
	leftItems, rightItems := v.splitItems()

	onSelect := func(_ int, item widgets.SectionItem) {
		if idx, ok := item.Reference.(int); ok {
			v.executeItem(idx)
		}
	}

	// ── Adaptive layout with resize ─────────────────────────────────────
	buildFn := func(width int) homeFlexResult {
		return buildHomeLayout(width, homeFlexConfig{
			App:          app,
			Header:       header,
			HeaderHeight: headerHeight,
			Footer:       footer,
			FooterHeight: 4,
			LeftItems:    leftItems,
			RightItems:   rightItems,
			OnSelect:     onSelect,
		})
	}

	initial := adaptiveHomeMount(app, content, buildFn)

	if initial.Dual != nil {
		v.dual = initial.Dual
		v.list = initial.Dual.left
	} else {
		v.dual = nil
		v.list = initial.SingleList
	}
}

// Unmount cleans up resources.
func (v *ProjectModeView) Unmount() {
	v.app = nil
	v.list = nil
	v.dual = nil
}

// HandleKey processes view-specific key events.
func (v *ProjectModeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list == nil {
		return event
	}

	// Dual-column navigation (h/l/Tab)
	if v.dual != nil {
		if consumed := v.dual.HandleKey(event); consumed == nil {
			return nil
		}
	}

	switch event.Key() {
	case tcell.KeyEnter:
		active := v.list
		if v.dual != nil {
			active = v.dual.activeList()
		}
		if _, item, ok := active.CurrentItem(); ok {
			if idx, refOk := item.Reference.(int); refOk {
				v.executeItem(idx)
			}
		}
		return nil
	}

	return event
}

func (v *ProjectModeView) executeItem(idx int) {
	if idx < 0 || idx >= len(v.items) {
		return
	}
	item := v.items[idx]
	if item.Action != nil {
		item.Action()
	}
}

func (v *ProjectModeView) buildItems() []projectModeItem {
	if v.project == nil {
		return nil
	}

	p := v.project
	navigate := func(viewID string) func() {
		return func() {
			if v.cfg.OnNavigate != nil {
				v.cfg.OnNavigate(viewID)
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

	items := []projectModeItem{
		// ── Sessions section ──
		{Icon: "─", Label: "Sessions"},
		{Icon: "▶", Label: "Start Dev", Desc: "Lancer une session de développement", Action: launch("", "--dev")},
		{Icon: "◉", Label: "Audit", Desc: "Analyser le code (sécurité, perf, archi)", Action: launch("auditor")},
		{Icon: "◎", Label: "Review", Desc: "Code review du projet", Action: launch("reviewer")},
		{Icon: "◈", Label: "Debug", Desc: "Session de debug", Action: launch("")},
		// ── Board section ──
		{Icon: "─", Label: "Board"},
		{Icon: "⊞", Label: "Board", Desc: "Kanban du projet", Action: navigate("board")},
		{Icon: "⊟", Label: "Métriques", Desc: "Statistiques d'utilisation", Action: navigate("metrics")},
		{Icon: "⊝", Label: "Statut", Desc: "Santé et informations du projet", Action: navigate("status")},
		// ── Configuration section ──
		{Icon: "─", Label: "Configuration"},
		{Icon: "⊛", Label: "Config Projet", Desc: "Modifier la configuration", Action: navigate("project.config")},
		{Icon: "⊜", Label: "Worktrees", Desc: "Gérer les git worktrees", Action: navigate("worktrees")},
	}

	// ── Team items (conditional) ────────────────────────────────────────
	if v.resolveTeam != nil {
		if tc := v.resolveTeam(); tc.Enabled {
			items = append(items,
				projectModeItem{Icon: "─", Label: "Équipe"},
				projectModeItem{Icon: "◫", Label: "Team Status", Desc: "Statut de l'équipe", Action: navigate("team.status")},
				projectModeItem{Icon: "◫", Label: "Team Board", Desc: "Kanban d'équipe", Action: navigate("team.board")},
				projectModeItem{Icon: "◫", Label: "Team Activity", Desc: "Activité récente", Action: navigate("team.activity")},
			)
		}
	}

	// ── Mode hub (standalone — no section) ──────────────────────────────
	items = append(items, projectModeItem{
		Icon: "↩", Label: "Mode Hub", Desc: "Revenir au TUI complet",
		Action: func() {
			if v.cfg.OnExitProjectMode != nil {
				v.cfg.OnExitProjectMode()
			}
		},
	})

	return items
}

// splitItems distributes project mode items into left/right columns for dual mode.
// Left: Sessions + Board. Right: Configuration + Équipe + Mode Hub.
func (v *ProjectModeView) splitItems() (left, right []widgets.SectionItem) {
	// Find the "Configuration" section boundary
	configIdx := -1
	for i, it := range v.items {
		if it.Icon == "─" && it.Label == "Configuration" {
			configIdx = i
			break
		}
	}

	for i, it := range v.items {
		si := projectItemToSectionItem(it, i)
		if configIdx >= 0 && i >= configIdx {
			right = append(right, si)
		} else {
			left = append(left, si)
		}
	}
	return
}

func projectItemToSectionItem(it projectModeItem, idx int) widgets.SectionItem {
	if it.Icon == "─" {
		return widgets.SectionItem{
			MainText: it.Label,
			IsHeader: true,
		}
	}
	return widgets.SectionItem{
		MainText:      fmt.Sprintf("%s  %s", it.Icon, it.Label),
		SecondaryText: it.Desc,
		Reference:     idx,
	}
}

// ContextCommands returns contextual commands for the omnibar.
// Since ADR-032 Phase 3, commands are filtered by mode at the registry level.
// The global command registry with Modes annotations replaces the need for
// per-view contextual command duplication.
func (v *ProjectModeView) ContextCommands() []ContextCommand {
	return nil
}
