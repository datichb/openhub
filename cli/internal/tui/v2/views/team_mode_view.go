// Package views — TeamModeView: team-scoped landing with mini dashboard (ADR-032 Phase 3).
package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// TeamModeConfig holds the callbacks used by the team mode view.
type TeamModeConfig struct {
	// OnNavigate is called when the user wants to navigate to a sub-view.
	OnNavigate func(viewID string)
	// OnLaunchSession is called to start a coding session (may trigger project selector).
	OnLaunchSession func(agent string, extraArgs ...string)
	// OnExitTeamMode is called when the user toggles back to hub mode.
	OnExitTeamMode func()
	// TeamStats returns live summary stats for the active team (positive metrics).
	TeamStats func() TeamModeStats
	// OnSyncTracker is called when the user triggers a tracker sync from the home page.
	OnSyncTracker func()
	// OnBoardConfig is called when the user wants to configure board columns.
	OnBoardConfig func()

	// ── Session picker callbacks ────────────────────────────────────────
	// OnAuditPicker opens the audit type picker.
	OnAuditPicker func()
	// OnReviewPicker opens the review mode picker.
	OnReviewPicker func()
	// OnDebugPicker opens the debug issue input.
	OnDebugPicker func()
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
	header   *tview.TextView // header with team name + stats (updated async)
	items    []teamModeItem
	mountGen uint64 // guards stale goroutines (standard pattern)
}

var _ View = (*TeamModeView)(nil)

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
		return fmt.Sprintf("%s · Ctrl+T %s", i18n.T("tui.tm.no_team"), i18n.T("tui.hints.hub_mode"))
	}
	return fmt.Sprintf(
		"%s · j/k %s · Enter %s · Ctrl+P %s · Ctrl+T %s",
		v.team.Name,
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.open"),
		i18n.T("tui.hints.commands"),
		i18n.T("tui.hints.hub_mode"),
	)
}

// Mount builds the team mode landing view.
func (v *TeamModeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.mountGen++
	gen := v.mountGen

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
			"\n  [red]%s[-]\n\n  %s",
			i18n.T("tui.tm.no_team_active"),
			i18n.Tf("tui.tm.hint_hub",
				theme.ColorTag(theme.AccentHex), theme.TagColor,
				theme.ColorTag(theme.AccentHex), theme.TagColor,
				theme.ColorTag(theme.ActionHex), theme.TagColor,
			),
		))
		content.AddItem(tv, 0, 1, true)
		return
	}

	v.items = v.buildItems()

	// ── Header with team name (stats loaded async) ──────────────────────
	v.header = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(false)
	v.header.SetBackgroundColor(theme.BgPanel)

	secondary := theme.ColorTag(theme.TextSecondaryHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	bannerName := strings.ToUpper(v.team.Name)
	banner, bh := renderBanner(bannerName, 100)
	headerHeight := bh + 5

	// Show header immediately with a "loading" placeholder for stats
	v.header.SetText(fmt.Sprintf("\n%s\n  %s%s%s\n  %s%s%s",
		banner,
		secondary, i18n.T("tui.tm.header_badge"), reset,
		muted, i18n.T("tui.tm.loading"), reset,
	))

	// Load stats asynchronously to avoid blocking the event loop
	if v.cfg.TeamStats != nil {
		go func() {
			stats := v.cfg.TeamStats()
			if v.app != nil {
				v.app.QueueUpdateDraw(func() {
					if v.mountGen != gen {
						return // view was re-mounted, discard stale result
					}
					v.header.SetText(fmt.Sprintf("\n%s\n  %s%s%s\n  %s%s%s",
						banner,
						secondary, i18n.T("tui.tm.header_badge"), reset,
						muted, i18n.Tf("tui.tm.stats", stats.MemberCount, stats.ActiveCount), reset,
					))
				})
			}
		}()
	}

	// ── Footer ──────────────────────────────────────────────────────────
	accent := theme.ColorTag(theme.AccentHex)
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

	// ── Split items for dual-column: left = Sessions+Board, right = Config+Nav ──
	leftItems, rightItems := v.splitItems()

	onSelect := func(_ int, item widgets.SectionItem) {
		if idx, ok := item.Reference.(int); ok {
			v.executeItem(idx)
		}
	}

	// ── Adaptive layout with resize ─────────────────────────────────────
	buildFn := func(width int) homeFlexResult {
		r := buildHomeLayout(width, homeFlexConfig{
			App:          app,
			Header:       v.header,
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
func (v *TeamModeView) Unmount() {
	v.app = nil
	v.list = nil
	v.dual = nil
	v.header = nil
}

// HandleKey processes view-specific key events.
func (v *TeamModeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	return homeHandleKey(event, v.list, v.dual, v.executeItem)
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

	// ── Session launchers (with dynamic project selection) ──
	if v.cfg.OnLaunchSession != nil {
		launch := v.cfg.OnLaunchSession

		// Audit/review/debug: use pickers if available, otherwise fall back to direct launch.
		auditAction := func() { launch("auditor") }
		if v.cfg.OnAuditPicker != nil {
			auditAction = v.cfg.OnAuditPicker
		}
		reviewAction := func() { launch("reviewer") }
		if v.cfg.OnReviewPicker != nil {
			reviewAction = v.cfg.OnReviewPicker
		}
		debugAction := func() { launch("debugger") }
		if v.cfg.OnDebugPicker != nil {
			debugAction = v.cfg.OnDebugPicker
		}

		items = append(items,
			teamModeItem{Icon: "─", Label: i18n.T("tui.tm.section.sessions"), SectionID: "sessions"},
			teamModeItem{Icon: "💻", Label: i18n.T("tui.tm.item.quick"), Desc: i18n.T("tui.tm.item.quick_desc"), Action: func() { launch("") }},
			teamModeItem{Icon: "🎯", Label: i18n.T("tui.tm.item.start_dev"), Desc: i18n.T("tui.tm.item.start_dev_desc"), Action: func() { launch("", "--dev") }},
			teamModeItem{Icon: "🔍", Label: i18n.T("tui.tm.item.audit"), Desc: i18n.T("tui.tm.item.audit_desc"), Action: auditAction},
			teamModeItem{Icon: "👀", Label: i18n.T("tui.tm.item.review"), Desc: i18n.T("tui.tm.item.review_desc"), Action: reviewAction},
			teamModeItem{Icon: "🐛", Label: i18n.T("tui.tm.item.debug"), Desc: i18n.T("tui.tm.item.debug_desc"), Action: debugAction},
			teamModeItem{Icon: "🔀", Label: i18n.T("tui.pm.item.parallel"), Desc: i18n.T("tui.pm.item.parallel_desc"), Action: navigate("parallel")},
			teamModeItem{Icon: "🎓", Label: i18n.T("tui.pm.item.onboard"), Desc: i18n.T("tui.pm.item.onboard_desc"), Action: func() { launch("onboarder") }},
		)
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
		teamModeItem{Icon: "🔗", Label: i18n.T("tui.tm.item.workflow"), Desc: i18n.T("tui.tm.item.workflow_desc"), Action: navigate("workflow")},
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
