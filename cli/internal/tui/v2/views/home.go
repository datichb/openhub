package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// homeItem represents a navigable item on the home screen.
type homeItem struct {
	Icon   string
	Label  string
	Desc   string
	ViewID string // navigate to this view (empty = action)
	Action func() // direct action (if ViewID is empty)
}

// HomeViewConfig holds external dependencies for the home view.
type HomeViewConfig struct {
	OnLaunchSession func(agent string, args ...string)
	// HasProject returns true when at least one active project exists.
	// Used to conditionally show project-specific items (ADR-032).
	HasProject func() bool
	// ListTeams returns configured teams with positive stats for the hub dashboard (ADR-032 Phase 3).
	ListTeams func() []TeamEntry
	// ListProjects returns active projects with context info (ADR-032 Phase 3).
	ListProjects func() []ProjectEntry
	// OnSelectTeam is called when the user selects a team — enters team mode.
	OnSelectTeam func(teamID, teamName string)
	// OnSelectProject is called when the user selects a project — enters project mode.
	OnSelectProject func(projectID, projectName, projectPath string)
	// OnAddProject is called when the user triggers the "add project" quick action.
	OnAddProject func()
}

// TeamEntry represents a team with positive stats for the hub home.
type TeamEntry struct {
	ID          string
	Name        string
	MemberCount int
	ActiveCount int // in_progress + review
}

// ProjectEntry represents a project with context info for the hub home.
type ProjectEntry struct {
	ID           string
	Name         string
	Path         string
	ActiveCount  int    // tasks in progress
	ActiveBranch string // current git branch
	DeployAge    string // human-readable deploy age ("il y a 2h", "jamais"), empty if unknown
}

// HomeView is the splash/landing view for the TUI shell.
type HomeView struct {
	app      *tview.Application
	content  *tview.Flex
	list     *widgets.SectionedList
	dual     *homeDualLayout
	shell    ShellAccess
	cfg      HomeViewConfig
	items    []homeItem
	mountGen uint64 // guards stale goroutines (standard pattern)
}

var _ View = (*HomeView)(nil)

func NewHomeView(cfg HomeViewConfig) *HomeView {
	return &HomeView{cfg: cfg}
}

// SetShell injects the shell for navigation and toasts.
func (v *HomeView) SetShell(s ShellAccess) { v.shell = s }

func (v *HomeView) ID() string    { return "home" }
func (v *HomeView) Title() string { return i18n.T("tui.home.title") }

func (v *HomeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content
	v.mountGen++
	gen := v.mountGen

	// Build static items immediately (System + Actions rapides).
	// Dynamic items (Teams, Projects) are loaded asynchronously below.
	v.items = v.buildStaticItems()

	// ── Logo (top) ──────────────────────────────────────────────────────
	logo := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(false)
	logo.SetBackgroundColor(theme.BgPanel)
	bannerStr, bh := renderBanner("OPENHUB", 100)
	logo.SetText(fmt.Sprintf("\n%s", bannerStr))

	// ── Footer ──────────────────────────────────────────────────────────
	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(false)
	footer.SetBackgroundColor(theme.BgPanel)
	footer.SetText(buildShortcutsFooter())

	onSelect := func(_ int, item widgets.SectionItem) {
		if ref, ok := item.Reference.(int); ok {
			v.executeItem(ref)
		}
	}

	headerHeight := bh + 3

	// buildFn reads v.items at call time (not captured slices) so that
	// both initial mount and async data arrival produce correct layouts.
	buildFn := func(width int) homeFlexResult {
		leftItems, rightItems := v.splitItems()
		r := buildHomeLayout(width, homeFlexConfig{
			App:          app,
			Header:       logo,
			HeaderHeight: headerHeight,
			Footer:       footer,
			FooterHeight: 4,
			LeftItems:    leftItems,
			RightItems:   rightItems,
			OnSelect:     onSelect,
		})
		// Keep HomeView pointers in sync after every rebuild (including resize).
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

	// ── Async: load Teams + Projects in background ─────────────────────
	go func() {
		items := v.buildItems()
		if app != nil {
			app.QueueUpdateDraw(func() {
				if v.mountGen != gen {
					return // view was re-mounted, discard stale result
				}
				v.items = items
				// Full layout rebuild to update both columns with fresh data
				content.Clear()
				adaptiveHomeMount(app, content, buildFn)
				// Restore focus to the new widget (same pattern as resize handler)
				if v.dual != nil {
					v.dual.focusLeft()
				} else if v.list != nil {
					app.SetFocus(v.list)
				}
			})
		}
	}()
}

func (v *HomeView) Unmount() {
	v.app = nil
	v.content = nil
	v.list = nil
	v.dual = nil
}

func (v *HomeView) StatusHints() string {
	return fmt.Sprintf("j/k %s · Enter %s · Ctrl+P %s · ? %s · Ctrl+Q %s",
		i18n.T("tui.hints.navigate"),
		i18n.T("tui.hints.open"),
		i18n.T("tui.hints.commands"),
		i18n.T("tui.hints.help"),
		i18n.T("tui.hints.quit"),
	)
}

func (v *HomeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	return homeHandleKey(event, v.list, v.dual, v.executeItem)
}

func (v *HomeView) executeItem(idx int) {
	if idx < 0 || idx >= len(v.items) {
		return
	}
	item := v.items[idx]
	if item.ViewID != "" && v.shell != nil {
		v.shell.NavigateTo(item.ViewID)
	} else if item.Action != nil {
		item.Action()
	}
}

// buildStaticItems returns items that don't require I/O (Sessions + System + Projets).
// Used for the initial synchronous frame before async data arrives.
func (v *HomeView) buildStaticItems() []homeItem {
	var items []homeItem

	// ── Sessions section (4 links — each asks for project first) ──
	if v.cfg.OnLaunchSession != nil {
		launch := v.cfg.OnLaunchSession
		items = append(items, homeItem{Icon: "─", Label: i18n.T("tui.pm.section.sessions"), Desc: ""})
		items = append(items,
			homeItem{Icon: "⚡", Label: i18n.T("tui.pm.item.quick"), Desc: i18n.T("tui.pm.item.quick_desc"), Action: func() { launch("") }},
			homeItem{Icon: "🎯", Label: i18n.T("tui.pm.item.start_dev"), Desc: i18n.T("tui.pm.item.start_dev_desc"), Action: func() { launch("", "--dev") }},
			homeItem{Icon: "🔍", Label: i18n.T("tui.pm.item.audit"), Desc: i18n.T("tui.pm.item.audit_desc"), Action: func() { launch("auditor") }},
			homeItem{Icon: "👀", Label: i18n.T("tui.pm.item.review"), Desc: i18n.T("tui.pm.item.review_desc"), Action: func() { launch("reviewer") }},
		)
	}

	// ── System / navigation ──
	items = append(items, homeItem{Icon: "─", Label: i18n.T("tui.home.section.system"), Desc: ""})
	items = append(items,
		homeItem{Icon: "🔧", Label: i18n.T("tui.settings.title"), Desc: i18n.T("tui.settings.desc"), ViewID: "settings"},
		homeItem{Icon: "📊", Label: i18n.T("tui.home.metrics"), Desc: i18n.T("tui.home.metrics_desc"), ViewID: "metrics"},
		homeItem{Icon: "🔬", Label: i18n.T("tui.home.doctor"), Desc: i18n.T("tui.home.doctor_desc"), ViewID: "doctor"},
		homeItem{Icon: "🔑", Label: i18n.T("tui.home.secrets"), Desc: i18n.T("tui.home.secrets_desc"), ViewID: "secrets"},
	)

	// ── Projets (enriched — replaces old "Quick Actions") ──
	items = append(items, homeItem{Icon: "─", Label: i18n.T("tui.home.section.projects"), Desc: ""})
	items = append(items,
		homeItem{Icon: "➕", Label: i18n.T("tui.home.project_add"), Desc: i18n.T("tui.home.project_add_desc"), Action: func() {
			if v.cfg.OnAddProject != nil {
				v.cfg.OnAddProject()
			}
		}},
	)

	return items
}

func (v *HomeView) buildItems() []homeItem {
	var items []homeItem

	// ── Teams section (ADR-032 Phase 3) ──
	if v.cfg.ListTeams != nil {
		teams := v.cfg.ListTeams()
		if len(teams) > 0 {
			items = append(items, homeItem{Icon: "─", Label: i18n.T("tui.home.section.teams"), Desc: ""})
			for _, t := range teams {
				team := t // capture
				desc := i18n.Tf("tui.home.team_desc", team.MemberCount, team.ActiveCount)
				items = append(items, homeItem{
					Icon:  "👥",
					Label: team.Name,
					Desc:  desc,
					Action: func() {
						if v.cfg.OnSelectTeam != nil {
							v.cfg.OnSelectTeam(team.ID, team.Name)
						}
					},
				})
			}
		}
	}

	// ── Projects section (ADR-032 Phase 3) ──
	if v.cfg.ListProjects != nil {
		projects := v.cfg.ListProjects()
		if len(projects) > 0 {
			items = append(items, homeItem{Icon: "─", Label: i18n.T("tui.home.section.projects"), Desc: ""})
			for _, p := range projects {
				proj := p // capture
				desc := proj.Path
				if proj.ActiveCount > 0 {
					desc = i18n.Tf("tui.home.project_active", proj.ActiveCount, proj.Path)
				}
				if proj.ActiveBranch != "" {
					desc += fmt.Sprintf(" · %s", proj.ActiveBranch)
				}
				if proj.DeployAge != "" {
					desc += fmt.Sprintf(" · %s", proj.DeployAge)
				}
				items = append(items, homeItem{
					Icon:  "📁",
					Label: proj.Name,
					Desc:  desc,
					Action: func() {
						if v.cfg.OnSelectProject != nil {
							v.cfg.OnSelectProject(proj.ID, proj.Name, proj.Path)
						}
					},
				})
			}
		}
	}

	// Append static items (System + Actions rapides) — shared with buildStaticItems
	items = append(items, v.buildStaticItems()...)

	return items
}

// splitItems distributes home items into left/right columns for dual mode.
// Left: Sessions + Teams + System. Right: Projects.
func (v *HomeView) splitItems() (left, right []widgets.SectionItem) {
	// Identify which sections go right
	rightSections := map[string]bool{
		i18n.T("tui.home.section.projects"): true,
	}

	inRight := false
	for i, it := range v.items {
		si := homeItemToSectionItem(it, i)
		if it.Icon == "─" {
			// Section header — check if this section goes right
			inRight = rightSections[it.Label]
		}
		if inRight {
			right = append(right, si)
		} else {
			left = append(left, si)
		}
	}
	return
}

func homeItemToSectionItem(it homeItem, idx int) widgets.SectionItem {
	if it.Icon == "─" {
		return widgets.SectionItem{
			MainText: it.Label,
			IsHeader: true,
		}
	}
	iconWidth := runewidth.StringWidth(it.Icon)
	pad := 3 - iconWidth
	if pad < 1 {
		pad = 1
	}
	return widgets.SectionItem{
		MainText:      it.Icon + strings.Repeat(" ", pad) + it.Label,
		SecondaryText: it.Desc,
		Reference:     idx,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Static rendering helpers
// ─────────────────────────────────────────────────────────────────────────────

func buildShortcutsFooter() string {
	accent := theme.ColorTag(theme.AccentHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	return fmt.Sprintf("\n%s%sCtrl+P%s %s  %s?%s %s  %sCtrl+T%s %s  %sCtrl+Q%s %s",
		muted,
		accent, reset, i18n.T("tui.home.shortcut.commands"),
		accent, reset, i18n.T("tui.home.shortcut.help"),
		accent, reset, i18n.T("tui.home.shortcut.team_mode"),
		accent, reset, i18n.T("tui.home.shortcut.quit"),
	)
}
