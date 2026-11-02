// Package views — TeamModeView: team-scoped landing with mini dashboard (ADR-032 Phase 3).
package views

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// TeamModeConfig holds the callbacks used by the team mode view.
type TeamModeConfig struct {
	// OnNavigate is called when the user wants to navigate to a sub-view.
	OnNavigate func(viewID string)
	// Start is the « Démarrer » section (P1-T20).
	Start StartSectionConfig
	// OnExitTeamMode is called when the user toggles back to hub mode.
	OnExitTeamMode func()
	// TeamStats returns live summary stats for the active team (positive metrics).
	TeamStats func() TeamModeStats
	// OnSyncTracker is called when the user triggers a tracker sync from the home page.
	OnSyncTracker func()
	// OnBoardConfig is called when the user wants to configure board columns.
	OnBoardConfig func()

	// Sessions shows the "Sessions de l'équipe" section (P3-T19).
	Sessions SessionsSectionConfig
}

// TeamModeStats holds the summary stats displayed in the team landing header.
// Only positive/neutral metrics — no blame or pressure numbers.
type TeamModeStats struct {
	MemberCount int // number of team members
	ActiveCount int // tickets in progress + review (momentum indicator)
}

// teamModeItem represents a navigable item in the team mode landing.
type teamModeItem struct {
	Icon      string
	Label     string
	Desc      string
	Action    func()
	SectionID string // structural ID for split logic (headers only)
	start     *startItem
}

// TeamModeView is the team-scoped landing view — a mini dashboard with
// quick actions for team navigation and optional session launching.
type TeamModeView struct {
	cfg      TeamModeConfig
	team     *ActiveTeam
	shell    ShellAccess
	app      *tview.Application
	list     *widgets.SectionedList
	dual     *homeDualLayout
	header   *tview.TextView // header with badge + stats (updated async)
	bannerTV *tview.TextView // centered ASCII art banner
	stats    string          // stats line of the header ("" = loading)
	summary  *TeamModeStats  // the stats once read (nil = not read yet)
	items    []teamModeItem
	loads    loadSlot // the read of the stats: only the latest is applied
}

var _ View = (*TeamModeView)(nil)
var _ ModeInfoProvider = (*TeamModeView)(nil)

// NewTeamModeView creates a new team mode view.
func NewTeamModeView(cfg TeamModeConfig) *TeamModeView {
	return &TeamModeView{cfg: cfg}
}

// SetShell provides the shell reference.
func (v *TeamModeView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *TeamModeView) ID() string { return "team.mode" }

// Title returns the display title.
func (v *TeamModeView) Title() string {
	if v.team != nil {
		return i18n.Tf("tui.tm.title", v.team.Name)
	}
	return i18n.T("tui.tm.title_default")
}

// StatusHints returns keybinding hints.
func (v *TeamModeView) StatusHints() string {
	if v.team == nil {
		return fmt.Sprintf("%s · Ctrl+T %s", i18n.T("tui.tm.no_team"), i18n.T("tui.hints.next_view"))
	}
	return keys.Hints(
		v.team.Name,
		keys.Navigate.HintAs(i18n.T("tui.hints.navigate")),
		keys.Open.HintAs(i18n.T("tui.hints.open")),
		"Ctrl+P "+i18n.T("tui.hints.commands"),
		"Ctrl+T "+i18n.T("tui.hints.next_view"),
	)
}

// ModeInfo provides contextual information for the persistent mode bar.
func (v *TeamModeView) ModeInfo() ModeBarInfo {
	info := ModeBarInfo{Icon: "👥", Label: i18n.T("tui.mode.team")}
	if v.team == nil {
		return info
	}
	info.Label = v.team.Name

	// Right-side context: member count + active tickets, once read (the
	// mount reads them off the event loop and refreshes the bar).
	if st := v.summary; st != nil {
		info.Right = fmt.Sprintf("%d %s · %d %s",
			st.MemberCount, i18n.T("tui.mode.members"),
			st.ActiveCount, i18n.T("tui.mode.active"))
	}
	return info
}

// Mount builds the team mode landing view.
func (v *TeamModeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	// Refresh team from shell on each mount
	if v.shell != nil {
		v.team = v.shell.ActiveTeam()
	}

	if v.team == nil {
		tv := tview.NewTextView().
			SetDynamicColors(true).
			SetScrollable(false)
		tv.SetBackgroundColor(theme.BgPanel)
		tv.SetBorderPadding(1, 0, 2, 2)
		tv.SetText(fmt.Sprintf(
			"\n  %s\n\n  %s",
			stateText(stateEmpty, i18n.T("tui.tm.no_team_active")),
			i18n.Tf("tui.tm.hint_hub",
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
	v.bannerTV = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(false)
	v.bannerTV.SetBackgroundColor(theme.BgPanel)

	// ── Header with badge + stats (left-aligned, loaded async) ─────────
	v.header = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	v.header.SetBackgroundColor(theme.BgPanel)

	bannerName := v.team.Name // as typed, like the name of a project
	banner, bh := renderBanner(bannerName, 100, theme.ActiveMode.AccentHex)
	compactNow := false

	// renderHeader shows the banner and the header; compact (N20): banner
	// and badge on one line, stats on the next.
	v.stats = ""
	renderHeader := func(compact bool) {
		compactNow = compact
		stats := v.stats
		if stats == "" {
			stats = stateText(stateLoading, i18n.T("tui.tm.loading"))
		} else {
			stats = theme.Paint(theme.TextMutedHex, stats)
		}
		v.header.SetWrap(!compact)
		if compact {
			v.bannerTV.SetText(compactBanner(bannerName, theme.ActiveMode.AccentHex, i18n.T("tui.tm.header_badge")))
			v.header.SetText("  " + stats)
			return
		}
		v.bannerTV.SetText("\n" + banner)
		v.header.SetText(fmt.Sprintf("  %s\n  %s",
			theme.Paint(theme.ActiveMode.PrimaryHex, i18n.T("tui.tm.header_badge")), stats))
	}

	// Read the stats off the event loop, then draw them in the header and in
	// the mode bar.
	v.summary = nil
	if stats := v.cfg.TeamStats; stats != nil {
		load(app, shellContext(v.shell), &v.loads, func(context.Context) (TeamModeStats, error) {
			return stats(), nil
		}, func(st TeamModeStats, err error) {
			if v.app == nil || err != nil {
				return // unmounted, or the read failed: the header keeps « loading »
			}
			v.summary = &st
			v.stats = i18n.Tf("tui.tm.stats", st.MemberCount, st.ActiveCount)
			renderHeader(compactNow)
			refreshModeBar(v.shell)
		})
	}

	footer := newHomeFooter()

	// ── Split items for dual-column: left = Sessions+Board, right = Config+Nav ──
	leftItems, rightItems := v.splitItems()

	onSelect := func(_ int, item widgets.SectionItem) {
		if idx, ok := item.Reference.(int); ok {
			v.executeItem(idx)
		}
	}

	// ── Adaptive layout with resize ─────────────────────────────────────
	// Combine banner (centered) + header (left-aligned) in a vertical flex
	combinedHeader := tview.NewFlex().SetDirection(tview.FlexRow)
	combinedHeader.SetBackgroundColor(theme.BgPanel)
	combinedHeader.AddItem(v.bannerTV, bh+1, 0, false) // banner + leading \n
	combinedHeader.AddItem(v.header, 0, 1, false)      // badge/stats fills rest

	buildFn := func(width int, compact bool) homeFlexResult {
		headerHeight := bh + 4
		if compact {
			headerHeight = 2
			combinedHeader.ResizeItem(v.bannerTV, 1, 0)
		} else {
			combinedHeader.ResizeItem(v.bannerTV, bh+1, 0)
		}
		renderHeader(compact)
		r := buildHomeLayout(width, homeFlexConfig{
			App:          app,
			Header:       combinedHeader,
			HeaderHeight: headerHeight,
			Compact:      compact,
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
func (v *TeamModeView) Unmount() {
	v.loads.Invalidate() // drop the read in flight
	v.app = nil
	v.list = nil
	v.dual = nil
	v.header = nil
	v.bannerTV = nil
}

// HandleKey processes view-specific key events.
func (v *TeamModeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if homePinKey.Matches(event) {
		if ref, ok := homeCurrentRef(v.list, v.dual); ok && ref < len(v.items) &&
			togglePinOf(v.cfg.Start, v.startScope(), v.items[ref].start) {
			return nil
		}
	}
	return homeHandleKey(event, v.list, v.dual, v.executeItem)
}

// startScope is the « Démarrer » scope of the active team.
func (v *TeamModeView) startScope() StartScope {
	if v.team == nil {
		return StartScope{}
	}
	return StartScope{TeamID: v.team.ID}
}

// pick shows a choice through the shell (workflows of a category).
func (v *TeamModeView) pick(title string, opts []SelectOption, onSelect func(string)) {
	if v.shell != nil {
		v.shell.ShowSelectModal(title, opts, "", onSelect)
	}
}

func (v *TeamModeView) executeItem(idx int) {
	if idx < 0 || idx >= len(v.items) {
		return
	}
	item := v.items[idx]
	if item.Action != nil {
		item.Action()
	}
}

func (v *TeamModeView) buildItems() []teamModeItem {
	if v.team == nil {
		return nil
	}

	navigate := func(viewID string) func() {
		return func() {
			if v.cfg.OnNavigate != nil {
				v.cfg.OnNavigate(viewID)
			}
		}
	}

	items := []teamModeItem{}

	// ── Démarrer (P1-T20) ──
	if header, sitems, ok := startSection(v.cfg.Start, v.startScope(), true, v.pick); ok {
		items = append(items, teamModeItem{Icon: "─", Label: header, SectionID: "sessions"})
		for i := range sitems {
			it := sitems[i]
			items = append(items, teamModeItem{Icon: it.Icon, Label: it.Label, Desc: it.Desc, Action: it.Action, start: &it})
		}
	}

	if header, sitems, ok := sessionsSection(v.cfg.Sessions, SessionsScope{TeamID: v.team.ID}, "tui.sessions.team_section"); ok {
		items = append(items, teamModeItem{Icon: "─", Label: header, SectionID: "running"})
		for _, it := range sitems {
			items = append(items, teamModeItem{Icon: it.Icon, Label: it.Label, Desc: it.Desc, Action: it.Action})
		}
	}

	// ── Board section ──
	items = append(items,
		teamModeItem{Icon: "─", Label: i18n.T("tui.tm.section.board"), SectionID: "board"},
		teamModeItem{Icon: "📋", Label: i18n.T("tui.tm.item.team_board"), Desc: i18n.T("tui.tm.item.team_board_desc"), Action: navigate("team.board")},
		teamModeItem{Icon: "👥", Label: i18n.T("tui.tm.item.team_status"), Desc: i18n.T("tui.tm.item.team_status_desc"), Action: navigate("team.status")},
		teamModeItem{Icon: "📈", Label: i18n.T("tui.tm.item.activity"), Desc: i18n.T("tui.tm.item.activity_desc"), Action: navigate("team.activity")},
		teamModeItem{Icon: "📄", Label: i18n.T("tui.tm.item.briefs"), Desc: i18n.T("tui.tm.item.briefs_desc"), Action: navigate("team.briefs")},
	)
	if v.cfg.OnBoardConfig != nil {
		boardCfgFn := v.cfg.OnBoardConfig
		items = append(items,
			teamModeItem{Icon: "🔧", Label: i18n.T("tui.tm.item.board_config"), Desc: i18n.T("tui.tm.item.board_config_desc"), Action: boardCfgFn},
		)
	}

	// ── Configuration section ──
	items = append(items,
		teamModeItem{Icon: "─", Label: i18n.T("tui.tm.section.configuration"), SectionID: "configuration"},
		teamModeItem{Icon: "🔗", Label: i18n.T("tui.tm.item.patterns"), Desc: i18n.T("tui.tm.item.patterns_desc"), Action: navigate("team.patterns")},
		teamModeItem{Icon: "📜", Label: i18n.T("tui.tm.item.policies"), Desc: i18n.T("tui.tm.item.policies_desc"), Action: navigate("team.policies")},
		teamModeItem{Icon: "🔧", Label: i18n.T("tui.tm.item.config_team"), Desc: i18n.T("tui.tm.item.config_team_desc"), Action: navigate("team.detail")},
		teamModeItem{Icon: "🔗", Label: i18n.T("tui.tm.item.workflow"), Desc: i18n.T("tui.tm.item.workflow_desc"), Action: navigate("workflows")},
	)

	// Sync Tracker (conditional — only if callback provided)
	if v.cfg.OnSyncTracker != nil {
		syncFn := v.cfg.OnSyncTracker
		items = append(items,
			teamModeItem{Icon: "🔄", Label: i18n.T("tui.tm.item.sync_tracker"), Desc: i18n.T("tui.tm.item.sync_tracker_desc"), Action: syncFn},
		)
	}

	// ── Navigation section ──
	items = append(items,
		teamModeItem{Icon: "─", Label: i18n.T("tui.tm.section.navigation"), SectionID: "navigation"},
		teamModeItem{Icon: "📂", Label: i18n.T("tui.tm.item.projects"), Desc: i18n.T("tui.tm.item.projects_desc"), Action: navigate("projects.list")},
		teamModeItem{Icon: "📊", Label: i18n.T("tui.tm.item.metrics"), Desc: i18n.T("tui.tm.item.metrics_desc"), Action: navigate("metrics")},
		teamModeItem{Icon: "🏠", Label: i18n.T("tui.tm.item.hub_mode"), Desc: i18n.T("tui.tm.item.hub_mode_desc"), Action: func() {
			if v.cfg.OnExitTeamMode != nil {
				v.cfg.OnExitTeamMode()
			}
		}},
	)

	return items
}

// splitItems distributes team mode items into left/right columns for dual mode.
// Left: Sessions + Board. Right: Configuration + Navigation.
func (v *TeamModeView) splitItems() (left, right []widgets.SectionItem) {
	generic := make([]homeSectionItem, len(v.items))
	for i, it := range v.items {
		generic[i] = homeSectionItem{Icon: it.Icon, Label: it.Label, Desc: it.Desc, SectionID: it.SectionID}
	}
	return splitBySectionID(generic, "configuration", toSectionItem)
}
