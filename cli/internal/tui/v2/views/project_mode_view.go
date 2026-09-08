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
	headerHeight := bannerHeight(v.project.Name, 100) + 5 // banner + subtitle(1) + spacing(2) + metadata(1) + padding(1)

	// ── Interactive list — SectionedList with auto-skip headers ──────────
	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorderPadding(0, 0, 3, 3)

	var sectionItems []widgets.SectionItem
	for idx, it := range v.items {
		if it.Icon == "─" {
			sectionItems = append(sectionItems, widgets.SectionItem{
				MainText: it.Label,
				IsHeader: true,
			})
		} else {
			sectionItems = append(sectionItems, widgets.SectionItem{
				MainText:      fmt.Sprintf("%s  %s", it.Icon, it.Label),
				SecondaryText: it.Desc,
				Reference:     idx,
			})
		}
	}
	v.list.SetItems(sectionItems)

	v.list.SetItemSelectedFunc(func(index int, item widgets.SectionItem) {
		if idx, ok := item.Reference.(int); ok {
			v.executeItem(idx)
		}
	})

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

	// ── Layout: centered column ─────────────────────────────────────────
	innerFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(theme.BgPanel)
	innerFlex.AddItem(header, headerHeight, 0, false)
	innerFlex.AddItem(v.list, 0, 1, true)
	innerFlex.AddItem(footer, 4, 0, false)

	// Horizontal centering
	hCenter := tview.NewFlex()
	hCenter.SetBackgroundColor(theme.BgPanel)
	hCenter.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)
	hCenter.AddItem(innerFlex, 72, 0, true)
	hCenter.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)

	// Vertical centering: equal top/bottom spacers
	content.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)
	content.AddItem(hCenter, 0, 3, true)
	content.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)
}

// Unmount cleans up resources.
func (v *ProjectModeView) Unmount() {
	v.app = nil
	v.list = nil
}

// HandleKey processes view-specific key events.
func (v *ProjectModeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if v.list == nil {
		return event
	}

	switch event.Key() {
	case tcell.KeyEnter:
		if _, item, ok := v.list.CurrentItem(); ok {
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
		// ── Projet section ──
		{Icon: "─", Label: "Projet"},
		{Icon: "⊞", Label: "Board", Desc: "Kanban du projet", Action: navigate("board")},
		{Icon: "⊟", Label: "Métriques", Desc: "Statistiques d'utilisation", Action: navigate("metrics")},
		{Icon: "⊛", Label: "Config Projet", Desc: "Modifier la configuration", Action: navigate("project.config")},
		{Icon: "⊜", Label: "Worktrees", Desc: "Gérer les git worktrees", Action: navigate("worktrees")},
		{Icon: "⊝", Label: "Statut", Desc: "Santé et informations du projet", Action: navigate("status")},
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

// ContextCommands returns contextual commands for the omnibar.
// Since ADR-032 Phase 3, commands are filtered by mode at the registry level.
// The global command registry with Modes annotations replaces the need for
// per-view contextual command duplication.
func (v *ProjectModeView) ContextCommands() []ContextCommand {
	return nil
}
