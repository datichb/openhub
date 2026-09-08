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
}

// TeamModeStats holds the summary stats displayed in the team landing header.
// Only positive/neutral metrics — no blame or pressure numbers.
type TeamModeStats struct {
	MemberCount int // number of team members
	ActiveCount int // tickets in progress + review (momentum indicator)
}

// teamModeItem represents a navigable item in the team mode landing.
type teamModeItem struct {
	Icon   string
	Label  string
	Desc   string
	Action func()
}

// TeamModeView is the team-scoped landing view — a mini dashboard with
// quick actions for team navigation and optional session launching.
type TeamModeView struct {
	cfg    TeamModeConfig
	team   *ActiveTeam
	shell  ShellAccess
	app    *tview.Application
	list   *widgets.SectionedList
	header *tview.TextView // header with team name + stats (updated async)
	items  []teamModeItem
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
		return fmt.Sprintf("Équipe · %s", v.team.Name)
	}
	return "Mode Équipe"
}

// StatusHints returns keybinding hints.
func (v *TeamModeView) StatusHints() string {
	if v.team == nil {
		return fmt.Sprintf("Aucune équipe · Ctrl+T %s", i18n.T("tui.hints.hub_mode"))
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
			"\n  [red]Aucune équipe active[-]\n\n  Utilisez %sCtrl+T%s pour revenir au mode hub.",
			theme.ColorTag(theme.AccentHex), theme.TagColor,
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

	banner := renderBanner(strings.ToUpper(v.team.ID), 68)
	headerHeight := bannerHeight(strings.ToUpper(v.team.ID), 68) + 5

	// Show header immediately with a "loading" placeholder for stats
	v.header.SetText(fmt.Sprintf("\n  %s◆ Mode Équipe%s\n\n%s\n  %schargement...%s",
		secondary, reset,
		banner,
		muted, reset,
	))

	// Load stats asynchronously to avoid blocking the event loop
	if v.cfg.TeamStats != nil {
		go func() {
			stats := v.cfg.TeamStats()
			if v.app != nil {
				v.app.QueueUpdateDraw(func() {
					v.header.SetText(fmt.Sprintf("\n  %s◆ Mode Équipe%s\n\n%s\n  %s%d membres · %d tickets actifs%s",
						secondary, reset,
						banner,
						muted, stats.MemberCount, stats.ActiveCount, reset,
					))
				})
			}
		}()
	}

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
	innerFlex.AddItem(v.header, headerHeight, 0, false)
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
func (v *TeamModeView) Unmount() {
	v.app = nil
	v.list = nil
	v.header = nil
}

// HandleKey processes view-specific key events.
func (v *TeamModeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
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

	items := []teamModeItem{
		// ── Équipe section ──
		{Icon: "─", Label: "Équipe"},
		{Icon: "◫", Label: "Team Board", Desc: "Kanban d'équipe", Action: navigate("team.board")},
		{Icon: "◫", Label: "Team Status", Desc: "Dashboard membres", Action: navigate("team.status")},
		{Icon: "◫", Label: "Activité", Desc: "Flux d'activité récent", Action: navigate("team.activity")},
		{Icon: "◫", Label: "Reprises", Desc: "Briefs de reprise de tickets", Action: navigate("team.briefs")},
		{Icon: "◫", Label: "Patterns", Desc: "Patterns d'équipe", Action: navigate("team.patterns")},
		{Icon: "◫", Label: "Policies", Desc: "Règles d'équipe", Action: navigate("team.policies")},
	}

	// ── Session launchers (with dynamic project selection) ──
	if v.cfg.OnLaunchSession != nil {
		launch := v.cfg.OnLaunchSession
		items = append(items,
			teamModeItem{Icon: "─", Label: "Sessions"},
			teamModeItem{Icon: "▶", Label: "Start Dev", Desc: "Lancer une session de développement", Action: func() { launch("", "--dev") }},
			teamModeItem{Icon: "◉", Label: "Audit", Desc: "Lancer un audit", Action: func() { launch("auditor") }},
			teamModeItem{Icon: "◈", Label: "Review", Desc: "Code review", Action: func() { launch("reviewer") }},
		)
	}

	// ── General navigation + exit (standalone — no section) ──
	items = append(items,
		teamModeItem{Icon: "◈", Label: "Projets", Desc: "Voir les projets", Action: navigate("projects.list")},
	)
	items = append(items,
		teamModeItem{Icon: "↩", Label: "Mode Hub", Desc: "Revenir au hub", Action: func() {
			if v.cfg.OnExitTeamMode != nil {
				v.cfg.OnExitTeamMode()
			}
		}},
	)

	return items
}
