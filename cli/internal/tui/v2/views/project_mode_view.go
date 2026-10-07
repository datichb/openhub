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
	// Start is the « Démarrer » section (P1-T20).
	Start StartSectionConfig
	// OnNavigate is called when the user wants to navigate to a sub-view.
	// viewID identifies the target view.
	OnNavigate func(viewID string)
	// OnExitProjectMode is called when the user toggles back to hub mode.
	OnExitProjectMode func()

	// ToolLine returns the session tool line of the header (adaptateur and
	// version, e.g. « <tool> <version> · <adapter> »). Must be cheap.
	ToolLine func() string

	// Sessions shows the "Sessions du projet" section (P3-T19).
	Sessions SessionsSectionConfig
}

// projectModeItem represents a navigable item in the project mode view.
type projectModeItem struct {
	Icon      string
	Label     string
	Desc      string
	Action    func()
	SectionID string // structural ID for split logic (headers only)
	start     *startItem
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

	mountGen uint64          // guards stale goroutines (standard pattern)
	headerTV *tview.TextView // header (badge, path, tool)
	bannerTV *tview.TextView // centered ASCII art banner
}

var _ View = (*ProjectModeView)(nil)
var _ CommandProvider = (*ProjectModeView)(nil)
var _ ModeInfoProvider = (*ProjectModeView)(nil)

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
		return i18n.Tf("tui.pm.title", v.project.Name)
	}
	return i18n.T("tui.pm.title_default")
}

// StatusHints returns keybinding hints for the omnibar (passive mode).
func (v *ProjectModeView) StatusHints() string {
	if v.project == nil {
		return fmt.Sprintf("%s · Ctrl+T %s", i18n.T("tui.pm.no_project"), i18n.T("tui.hints.hub_mode"))
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

// ModeInfo provides contextual information for the persistent mode bar.
func (v *ProjectModeView) ModeInfo() ModeBarInfo {
	info := ModeBarInfo{Icon: "💻", Label: i18n.T("tui.mode.project")}
	if v.project == nil {
		return info
	}
	info.Label = v.project.Name

	info.Right = v.project.Branch
	return info
}

// Mount builds the project mode view.
func (v *ProjectModeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++

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
			"\n  [red]%s[-]\n\n  %s",
			i18n.T("tui.pm.no_project"),
			i18n.Tf("tui.pm.hint_hub",
				theme.ColorTag(theme.ActiveMode.PrimaryHex), theme.TagColor,
				theme.ColorTag(theme.ActiveMode.PrimaryHex), theme.TagColor,
				theme.ColorTag(theme.ActiveMode.SecondaryHex), theme.TagColor,
			),
		))
		content.AddItem(tv, 0, 1, true)
		return
	}

	v.items = v.buildItems()

	// ── Banner (centered) ──────────────────────────────────────────────
	bannerView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(false)
	bannerView.SetBackgroundColor(theme.BgPanel)
	v.bannerTV = bannerView

	// ── Header (left-aligned: badge + path + session tool) ──────────
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	header.SetBackgroundColor(theme.BgPanel)
	v.headerTV = header

	v.renderHeader()
	_, bh := renderBanner(v.project.Name, 100, theme.ActiveMode.AccentHex)
	headerHeight := bh + 5

	// ── Footer ──────────────────────────────────────────────────────────
	muted := theme.ColorTag(theme.TextMutedHex)
	accent := theme.ColorTag(theme.ActiveMode.PrimaryHex)
	reset := theme.TagColor
	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(false)
	footer.SetBackgroundColor(theme.BgPanel)
	footer.SetText(fmt.Sprintf("\n%s%sCtrl+P%s %s  %s?%s %s  %sCtrl+T%s %s  %sCtrl+Q%s %s",
		muted,
		accent, reset, i18n.T("tui.home.shortcut.commands"),
		accent, reset, i18n.T("tui.home.shortcut.help"),
		accent, reset, i18n.T("tui.home.shortcut.team_mode"),
		accent, reset, i18n.T("tui.home.shortcut.quit"),
	))

	// ── Split items for dual-column: left = Sessions+Projet, right = Config+Team+Hub ──
	leftItems, rightItems := v.splitItems()

	onSelect := func(_ int, item widgets.SectionItem) {
		if idx, ok := item.Reference.(int); ok {
			v.executeItem(idx)
		}
	}

	// ── Adaptive layout with resize ─────────────────────────────────────
	// Combine banner (centered) + header (left-aligned) in a vertical flex
	// so both can be passed as a single Header to the layout builder.
	combinedHeader := tview.NewFlex().SetDirection(tview.FlexRow)
	combinedHeader.SetBackgroundColor(theme.BgPanel)
	combinedHeader.AddItem(bannerView, bh+1, 0, false) // banner + leading \n
	combinedHeader.AddItem(header, 0, 1, false)        // badge/path fills rest

	buildFn := func(width int) homeFlexResult {
		r := buildHomeLayout(width, homeFlexConfig{
			App:          app,
			Header:       combinedHeader,
			HeaderHeight: headerHeight,
			Footer:       footer,
			FooterHeight: 4,
			LeftItems:    leftItems,
			RightItems:   rightItems,
			OnSelect:     onSelect,
		})
		// Keep pointers in sync after every rebuild (including resize).
		if r.Dual != nil {
			v.dual = r.Dual
			v.list = r.Dual.left
		} else {
			v.dual = nil
			v.list = r.SingleList
		}
		return r
	}

	adaptiveHomeMount(app, content, buildFn)
}

// Unmount cleans up resources.
func (v *ProjectModeView) Unmount() {
	v.mountGen++ // invalidate in-flight async goroutine
	v.app = nil
	v.list = nil
	v.dual = nil
	v.headerTV = nil
	v.bannerTV = nil
}

// HandleKey processes view-specific key events.
func (v *ProjectModeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Rune() == '*' {
		if ref, ok := homeCurrentRef(v.list, v.dual); ok && ref < len(v.items) &&
			togglePinOf(v.cfg.Start, v.startScope(), v.items[ref].start) {
			return nil
		}
	}
	// Common home-like handling (dual-column nav, Enter)
	if result := homeHandleKey(event, v.list, v.dual, v.executeItem); result == nil {
		return nil
	}

	return event
}

// startScope is the « Démarrer » scope of the active project.
func (v *ProjectModeView) startScope() StartScope {
	if v.project == nil {
		return StartScope{}
	}
	return StartScope{ProjectID: v.project.ID}
}

// pick shows a choice through the shell (workflows of a category).
func (v *ProjectModeView) pick(title string, opts []SelectOption, onSelect func(string)) {
	if v.shell != nil {
		v.shell.ShowSelectModal(title, opts, "", onSelect)
	}
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

// ─────────────────────────────────────────────────────────────────────────────
// Header rendering
// ─────────────────────────────────────────────────────────────────────────────

// renderHeader sets (or re-sets) the header text (badge, path, tool).
// The banner is rendered separately in bannerTV (centered).
func (v *ProjectModeView) renderHeader() {
	if v.headerTV == nil || v.project == nil {
		return
	}
	secondary := theme.ColorTag(theme.ActiveMode.PrimaryHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	// Update banner (centered)
	if v.bannerTV != nil {
		banner, _ := renderBanner(v.project.Name, 100, theme.ActiveMode.AccentHex)
		v.bannerTV.SetText("\n" + banner)
	}

	// Update header (badge + path + session tool)
	tool := ""
	if v.cfg.ToolLine != nil {
		tool = muted + v.cfg.ToolLine() + reset
	}
	pathInfo := v.project.Path
	if v.project.Branch != "" {
		pathInfo += fmt.Sprintf(" · %s", v.project.Branch)
	}
	v.headerTV.SetText(fmt.Sprintf("  %s%s%s\n  %s%s%s\n  %s",
		secondary, i18n.T("tui.pm.header_badge"), reset,
		muted, pathInfo, reset,
		tool,
	))
}

// ─────────────────────────────────────────────────────────────────────────────
// Items
// ─────────────────────────────────────────────────────────────────────────────

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
	var items []projectModeItem
	scope := v.startScope()
	if header, sitems, ok := startSection(v.cfg.Start, scope, true, v.pick); ok {
		items = append(items, projectModeItem{Icon: "─", Label: header, SectionID: "sessions"})
		for i := range sitems {
			it := sitems[i]
			items = append(items, projectModeItem{Icon: it.Icon, Label: it.Label, Desc: it.Desc, Action: it.Action, start: &it})
		}
	}
	if header, sitems, ok := sessionsSection(v.cfg.Sessions, SessionsScope{ProjectID: p.ID}, "tui.sessions.project_section"); ok {
		items = append(items, projectModeItem{Icon: "─", Label: header, SectionID: "running"})
		for _, it := range sitems {
			items = append(items, projectModeItem{Icon: it.Icon, Label: it.Label, Desc: it.Desc, Action: it.Action})
		}
	}
	items = append(items, []projectModeItem{
		// ── Projet section ──
		{Icon: "─", Label: i18n.T("tui.pm.section.project"), SectionID: "project"},
		{Icon: "📋", Label: i18n.T("tui.pm.item.board"), Desc: i18n.T("tui.pm.item.board_desc"), Action: navigate("board")},
		{Icon: "📊", Label: i18n.T("tui.pm.item.metrics"), Desc: i18n.T("tui.pm.item.metrics_desc"), Action: navigate("metrics")},
		{Icon: "📡", Label: i18n.T("tui.pm.item.status"), Desc: i18n.T("tui.pm.item.status_desc"), Action: navigate("status")},
		// ── Configuration section ──
		{Icon: "─", Label: i18n.T("tui.pm.section.configuration"), SectionID: "configuration"},
		{Icon: "🔧", Label: i18n.T("tui.pm.item.config"), Desc: i18n.T("tui.pm.item.config_desc"), Action: navigate("project.config")},
		{Icon: "🌿", Label: i18n.T("tui.pm.item.worktrees"), Desc: i18n.T("tui.pm.item.worktrees_desc"), Action: navigate("worktrees")},
		{Icon: "🔗", Label: i18n.T("tui.pm.item.workflows"), Desc: i18n.T("tui.pm.item.workflows_desc"), Action: navigate("workflows")},
	}...)

	// ── Team items (conditional) ────────────────────────────────────────
	if v.resolveTeam != nil {
		if tc := v.resolveTeam(); tc.Enabled {
			items = append(items,
				projectModeItem{Icon: "─", Label: i18n.T("tui.pm.section.team"), SectionID: "team"},
				projectModeItem{Icon: "👥", Label: i18n.T("tui.pm.item.team_status"), Desc: i18n.T("tui.pm.item.team_status_desc"), Action: navigate("team.status")},
				projectModeItem{Icon: "📋", Label: i18n.T("tui.pm.item.team_board"), Desc: i18n.T("tui.pm.item.team_board_desc"), Action: navigate("team.board")},
				projectModeItem{Icon: "📈", Label: i18n.T("tui.pm.item.team_activity"), Desc: i18n.T("tui.pm.item.team_activity_desc"), Action: navigate("team.activity")},
			)
		}
	}

	// ── Mode hub (standalone — no section) ──────────────────────────────
	items = append(items, projectModeItem{
		Icon: "🏠", Label: i18n.T("tui.pm.item.hub_mode"), Desc: i18n.T("tui.pm.item.hub_mode_desc"),
		Action: func() {
			if v.cfg.OnExitProjectMode != nil {
				v.cfg.OnExitProjectMode()
			}
		},
	})

	return items
}

// splitItems distributes project mode items into left/right columns for dual mode.
// Left: Sessions + Projet. Right: Configuration + Équipe + Mode Hub.
func (v *ProjectModeView) splitItems() (left, right []widgets.SectionItem) {
	generic := make([]homeSectionItem, len(v.items))
	for i, it := range v.items {
		generic[i] = homeSectionItem{Icon: it.Icon, Label: it.Label, Desc: it.Desc, SectionID: it.SectionID}
	}
	return splitBySectionID(generic, "configuration", toSectionItem)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// ContextCommands returns contextual commands for the omnibar.
// Since ADR-032 Phase 3, commands are filtered by mode at the registry level.
// The global command registry with Modes annotations replaces the need for
// per-view contextual command duplication.
func (v *ProjectModeView) ContextCommands() []ContextCommand {
	return nil
}
