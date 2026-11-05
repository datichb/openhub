package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ProjectModeConfig holds the callbacks used by the project mode view.
type ProjectModeConfig struct {
	// Start is the « Démarrer » section.
	Start StartSectionConfig
	// OnNavigate is called when the user wants to navigate to a sub-view.
	// viewID identifies the target view.
	OnNavigate func(viewID string)
	// OnExitProjectMode is called when the user toggles back to hub mode.
	OnExitProjectMode func()

	// ToolLine returns the session tool line of the header (adaptateur and
	// version, e.g. « <tool> <version> · <adapter> »). Must be cheap.
	ToolLine func() string

	// Sessions shows the "Sessions du projet" section.
	Sessions SessionsSectionConfig

	// Team describes the team the active project belongs to, for the « Équipe »
	// line of the landing. Must be cheap. Nil: the line says there is none.
	Team func(project *ActiveProject) ProjectTeamInfo
	// OnAttachTeam opens the choice of the team of the project (« Rattacher le
	// projet à une équipe »). Nil: the line does nothing.
	OnAttachTeam func()

	// Runtimes returns the execution environments of the project with their
	// availability (⌂ local, ▣ container, ☁ remote: 10 §2). It may probe the
	// container engine or the keychain: the view calls it off the event loop.
	// Nil hides the line.
	Runtimes func(ctx context.Context, project *ActiveProject) []LaunchRuntime
}

// ProjectTeamInfo is the team of a project as the landing of the project
// shows it. A zero ID is « no team ».
type ProjectTeamInfo struct {
	ID       string
	Name     string
	Solo     bool // a solo space: no team features
	Archived bool
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

	headerTV *tview.TextView // header (badge, path, tool, runtimes)
	bannerTV *tview.TextView // centered ASCII art banner
	compact  bool            // the layout in use is the compact one
	runtimes []LaunchRuntime // the environments, once read
	loads    loadSlot        // the read of the environments: only the latest is applied
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
		return keys.Hints(i18n.T("tui.pm.no_project"), keys.Back.HintAs(i18n.T("tui.hints.hub_mode")), "Ctrl+T "+i18n.T("tui.hints.next_view"))
	}
	// Ctrl+T goes to the next view of the cycle; Esc goes back to the hub.
	return keys.Hints(
		v.project.Name,
		keys.Navigate.HintAs(i18n.T("tui.hints.navigate")),
		keys.Open.HintAs(i18n.T("tui.hints.open")),
		"Ctrl+P "+i18n.T("tui.hints.commands"),
		keys.Back.HintAs(i18n.T("tui.hints.hub_mode")),
		"Ctrl+T "+i18n.T("tui.hints.next_view"),
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
			"\n  %s\n\n  %s",
			stateText(stateEmpty, i18n.T("tui.pm.no_project")),
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

	_, bh := renderBanner(v.project.Name, 100, theme.ActiveMode.AccentHex)

	footer := newHomeFooter()

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

	buildFn := func(width int, compact bool) homeFlexResult {
		// Compact: banner and badge on one line, path and tool on
		// the next, no footer.
		v.compact = compact
		headerHeight := bh + 6
		if compact {
			headerHeight = 2
			combinedHeader.ResizeItem(bannerView, 1, 0)
		} else {
			combinedHeader.ResizeItem(bannerView, bh+1, 0)
		}
		v.renderHeader(compact)
		r := buildHomeLayout(width, homeFlexConfig{
			App:          app,
			Header:       combinedHeader,
			HeaderHeight: headerHeight,
			Footer:       footer,
			FooterHeight: 4,
			LeftItems:    leftItems,
			RightItems:   rightItems,
			OnSelect:     onSelect,
			Compact:      compact,
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

	// ── Execution environments, read off the event loop ────────────────
	v.runtimes = nil
	if read, project := v.cfg.Runtimes, v.project; read != nil {
		load(app, shellContext(v.shell), &v.loads, func(ctx context.Context) ([]LaunchRuntime, error) {
			return read(ctx, project), nil
		}, func(rt []LaunchRuntime, err error) {
			if v.app == nil || err != nil {
				return // unmounted, or the read failed: the line stays out
			}
			v.runtimes = rt
			v.renderHeader(v.compact)
		})
	}
}

// Unmount cleans up resources.
func (v *ProjectModeView) Unmount() {
	v.loads.Invalidate() // drop the read in flight
	v.app = nil
	v.list = nil
	v.dual = nil
	v.headerTV = nil
	v.bannerTV = nil
}

// HandleKey processes view-specific key events.
func (v *ProjectModeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if homePinKey.Matches(event) {
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
// The banner is rendered separately in bannerTV (centered); compact: one
// line each.
func (v *ProjectModeView) renderHeader(compact bool) {
	if v.headerTV == nil || v.project == nil {
		return
	}
	// Update banner (centered)
	if v.bannerTV != nil {
		if compact {
			v.bannerTV.SetText(compactBanner(v.project.Name, theme.ActiveMode.AccentHex, i18n.T("tui.pm.header_badge")))
		} else {
			banner, _ := renderBanner(v.project.Name, 100, theme.ActiveMode.AccentHex)
			v.bannerTV.SetText("\n" + banner)
		}
	}

	// Update header (badge + path + session tool)
	tool := ""
	if v.cfg.ToolLine != nil {
		tool = theme.Paint(theme.TextMutedHex, tview.Escape(v.cfg.ToolLine()))
	}
	pathInfo := v.project.Path
	if v.project.Branch != "" {
		pathInfo += fmt.Sprintf(" · %s", v.project.Branch)
	}
	v.headerTV.SetWrap(!compact) // compact: one line, cut on the right
	if compact {
		line := "  " + theme.Paint(theme.TextMutedHex, tview.Escape(pathInfo))
		if tool != "" {
			line += theme.Paint(theme.TextMutedHex, " · ") + tool
		}
		v.headerTV.SetText(line)
		return
	}
	lines := []string{
		theme.Paint(theme.ActiveMode.PrimaryHex, i18n.T("tui.pm.header_badge")),
		theme.Paint(theme.TextMutedHex, tview.Escape(pathInfo)),
		tool,
	}
	if rt := runtimesLine(v.runtimes); rt != "" {
		lines = append(lines, rt)
	}
	v.headerTV.SetText("  " + strings.Join(lines, "\n  "))
}

// runtimesLine is the line of the execution environments of the header
// (« exécution : ⌂ local ✔  ▣ conteneur · colima ✔  ☁ distant ✗ »): each
// environment with its icon, its label and its availability. "" without any.
func runtimesLine(runtimes []LaunchRuntime) string {
	if len(runtimes) == 0 {
		return ""
	}
	parts := make([]string, len(runtimes))
	for i, r := range runtimes {
		state := theme.Paint(theme.SuccessHex, theme.SymSuccess)
		if !r.Available {
			state = theme.Paint(theme.ErrorHex, theme.SymFailure)
		}
		parts[i] = theme.Paint(theme.TextMutedHex, tview.Escape(r.Label)) + " " + state
	}
	return theme.Paint(theme.TextMutedHex, i18n.T("tui.pm.runtimes")+" ") + strings.Join(parts, "  ")
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
		{Icon: "📚", Label: i18n.T("tui.pm.item.wiki"), Desc: i18n.T("tui.pm.item.wiki_desc"), Action: navigate("project.wiki")},
		// ── Configuration section ──
		{Icon: "─", Label: i18n.T("tui.pm.section.configuration"), SectionID: "configuration"},
		{Icon: "🔧", Label: i18n.T("tui.pm.item.config"), Desc: i18n.T("tui.pm.item.config_desc"), Action: navigate("project.config")},
		{Icon: "🌿", Label: i18n.T("tui.pm.item.worktrees"), Desc: i18n.T("tui.pm.item.worktrees_desc"), Action: navigate("worktrees")},
		{Icon: "🔗", Label: i18n.T("tui.pm.item.workflows"), Desc: i18n.T("tui.pm.item.workflows_desc"), Action: navigate("workflows")},
	}...)

	// ── Team section: always there, with the team of the project ──
	items = append(items, projectModeItem{Icon: "─", Label: i18n.T("tui.pm.section.team"), SectionID: "team"})
	items = append(items, v.teamLine())
	if v.resolveTeam != nil {
		if tc := v.resolveTeam(); tc.Enabled {
			items = append(items,
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

// teamLine is the first row of the « Équipe » section: the team of the
// project, or « aucune », and the way to change it.
func (v *ProjectModeView) teamLine() projectModeItem {
	var info ProjectTeamInfo
	if v.cfg.Team != nil {
		info = v.cfg.Team(v.project)
	}
	label, desc := "", i18n.T("v54u.pm.team_change")
	switch {
	case info.ID == "":
		label, desc = i18n.Tf("v54u.pm.team_line", i18n.T("v54u.pm.team_none")), i18n.T("v54u.pm.team_attach")
	case info.Solo:
		label = i18n.Tf("v54u.pm.team_line", i18n.Tf("v54u.pm.team_solo", info.Name))
	case info.Archived:
		label = i18n.Tf("v54u.pm.team_line", i18n.Tf("v54u.pm.team_archived", info.Name))
	default:
		label = i18n.Tf("v54u.pm.team_line", info.Name)
	}
	return projectModeItem{Icon: "👥", Label: label, Desc: desc, Action: func() {
		if v.cfg.OnAttachTeam != nil {
			v.cfg.OnAttachTeam()
		}
	}}
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
