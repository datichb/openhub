package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// homeItem represents a navigable item on the home screen.
type homeItem struct {
	Icon   string
	Label  string
	Desc   string
	ViewID string // navigate to this view (empty = action)
	Action func() // direct action (if ViewID is empty)
	// start is the « Démarrer » item (target of `*`).
	start *startItem
}

// HomeViewConfig holds external dependencies for the home view.
type HomeViewConfig struct {
	// Start is the « Démarrer » section (P1-T20).
	Start StartSectionConfig
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
	// Sessions shows the "Sessions" section (P3-T19).
	Sessions SessionsSectionConfig
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
}

// HomeView is the splash/landing view for the TUI shell.
type HomeView struct {
	app     *tview.Application
	content *tview.Flex
	list    *widgets.SectionedList
	dual    *homeDualLayout
	shell   ShellAccess
	cfg     HomeViewConfig
	items   []homeItem
	loads   loadSlot // the read of the teams and projects: only the latest is applied
}

// homeData is what the landing reads off the event loop: the teams and the
// projects.
type homeData struct {
	teams    []TeamEntry
	projects []ProjectEntry
}

var _ View = (*HomeView)(nil)
var _ ModeInfoProvider = (*HomeView)(nil)

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

	// Build static items immediately (System + Actions rapides).
	// Dynamic items (Teams, Projects) are loaded asynchronously below.
	v.items = v.buildStaticItems()

	// ── Logo (top) ──────────────────────────────────────────────────────
	logo := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(false)
	logo.SetBackgroundColor(theme.BgPanel)
	bannerStr, bh := renderBanner("OPENHUB", 100, theme.ActiveMode.AccentHex)

	footer := newHomeFooter(footerHub)

	onSelect := func(_ int, item widgets.SectionItem) {
		if ref, ok := item.Reference.(int); ok {
			v.executeItem(ref)
		}
	}

	// buildFn reads v.items at call time (not captured slices) so that
	// both initial mount and async data arrival produce correct layouts.
	// Compact (N20): the banner on one line, no footer.
	buildFn := func(width int, compact bool) homeFlexResult {
		headerHeight := bh + 3
		if compact {
			headerHeight = 1
			logo.SetText(compactBanner("OPENHUB", theme.ActiveMode.AccentHex, ""))
		} else {
			logo.SetText(fmt.Sprintf("\n%s", bannerStr))
		}
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
			Compact:      compact,
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

	// ── Teams + Projects, read off the event loop ──────────────────────
	cfg := v.cfg
	load(app, shellContext(v.shell), &v.loads, func(context.Context) (homeData, error) {
		return readHomeData(cfg), nil
	}, func(d homeData, err error) {
		if v.app == nil || err != nil {
			return // unmounted, or the read failed: the static items stay
		}
		v.items = v.buildItems(d)
		// Keep the focus where it is when a modal (e.g. the quit
		// dialog) holds it: only the home lists give it back.
		owned := homeOwnsFocus(app, v.list, v.dual)
		// Full layout rebuild to update both columns with fresh data
		content.Clear()
		adaptiveHomeMount(app, content, buildFn)
		// Restore focus to the new widget (same pattern as resize handler)
		if !owned {
			return
		}
		if v.dual != nil {
			v.dual.focusLeft()
		} else if v.list != nil {
			app.SetFocus(v.list)
		}
	})
}

// readHomeData reads the teams and the projects of the landing. It does I/O:
// call it off the event loop.
func readHomeData(cfg HomeViewConfig) homeData {
	var d homeData
	if cfg.ListTeams != nil {
		d.teams = cfg.ListTeams()
	}
	if cfg.ListProjects != nil {
		d.projects = cfg.ListProjects()
	}
	return d
}

func (v *HomeView) Unmount() {
	v.loads.Invalidate() // drop the read in flight
	v.app = nil
	v.content = nil
	v.list = nil
	v.dual = nil
}

func (v *HomeView) StatusHints() string {
	return keys.Hints(
		keys.Navigate.HintAs(i18n.T("tui.hints.navigate")),
		keys.Open.HintAs(i18n.T("tui.hints.open")),
		"Ctrl+P "+i18n.T("tui.hints.commands"),
		keys.Help.HintAs(i18n.T("tui.hints.help")),
		"Ctrl+Q "+i18n.T("tui.hints.quit"),
	)
}

// ModeInfo provides contextual information for the persistent mode bar.
func (v *HomeView) ModeInfo() ModeBarInfo {
	return ModeBarInfo{
		Icon:  "🏠",
		Label: i18n.T("tui.mode.hub"),
		Right: "openhub",
	}
}

func (v *HomeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if homePinKey.Matches(event) {
		if ref, ok := homeCurrentRef(v.list, v.dual); ok && ref < len(v.items) &&
			togglePinOf(v.cfg.Start, StartScope{}, v.items[ref].start) {
			return nil
		}
	}
	return homeHandleKey(event, v.list, v.dual, v.executeItem)
}

// pick shows a choice through the shell (workflows of a category).
func (v *HomeView) pick(title string, opts []SelectOption, onSelect func(string)) {
	if v.shell != nil {
		v.shell.ShowSelectModal(title, opts, "", onSelect)
	}
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
	return append(v.buildLeftStaticItems(), v.buildProjectsSection(nil)...)
}

// buildLeftStaticItems returns the I/O-free sections other than « Projets »
// (Démarrer, Sessions, Système).
func (v *HomeView) buildLeftStaticItems() []homeItem {
	var items []homeItem

	// ── Démarrer (P1-T20) ──
	if header, sitems, ok := startSection(v.cfg.Start, StartScope{}, false, v.pick); ok {
		items = append(items, homeItem{Icon: "─", Label: header})
		for i := range sitems {
			it := sitems[i]
			items = append(items, homeItem{Icon: it.Icon, Label: it.Label, Desc: it.Desc, Action: it.Action, start: &it})
		}
	}

	// ── Running sessions (v5) ──
	if header, sitems, ok := sessionsSection(v.cfg.Sessions, SessionsScope{}, "tui.sessions.home_section"); ok {
		items = append(items, homeItem{Icon: "─", Label: header})
		for _, it := range sitems {
			items = append(items, homeItem{Icon: it.Icon, Label: it.Label, Desc: it.Desc, Action: it.Action})
		}
	}

	// ── System / navigation ──
	items = append(items, homeItem{Icon: "─", Label: i18n.T("tui.home.section.system"), Desc: ""})
	items = append(items,
		homeItem{Icon: "🔧", Label: i18n.T("tui.settings.title"), Desc: i18n.T("tui.settings.desc"), ViewID: "settings"},
		homeItem{Icon: "📊", Label: i18n.T("tui.home.metrics"), Desc: i18n.T("tui.home.metrics_desc"), ViewID: "metrics"},
		homeItem{Icon: "🔬", Label: i18n.T("tui.home.doctor"), Desc: i18n.T("tui.home.doctor_desc"), ViewID: "doctor"},
		homeItem{Icon: "🔑", Label: i18n.T("tui.home.secrets"), Desc: i18n.T("tui.home.secrets_desc"), ViewID: "secrets"},
	)

	return items
}

// buildProjectsSection returns the single « Projets » section: the known
// projects followed by the « Ajouter un projet » row (always present, so the
// section exists even without any project).
func (v *HomeView) buildProjectsSection(projects []ProjectEntry) []homeItem {
	items := []homeItem{{Icon: "─", Label: i18n.T("tui.home.section.projects"), Desc: ""}}
	for _, p := range projects {
		proj := p // capture
		desc := proj.Path
		if proj.ActiveCount > 0 {
			desc = i18n.Tf("tui.home.project_active", proj.ActiveCount, proj.Path)
		}
		if proj.ActiveBranch != "" {
			desc += fmt.Sprintf(" · %s", proj.ActiveBranch)
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
	items = append(items, homeItem{Icon: "➕", Label: i18n.T("tui.home.project_add"), Desc: i18n.T("tui.home.project_add_desc"), Action: func() {
		if v.cfg.OnAddProject != nil {
			v.cfg.OnAddProject()
		}
	}})
	return items
}

// buildItems is the list of the landing once its teams and projects are read.
func (v *HomeView) buildItems(d homeData) []homeItem {
	var items []homeItem

	// ── Teams section (ADR-032 Phase 3) ──
	if len(d.teams) > 0 {
		items = append(items, homeItem{Icon: "─", Label: i18n.T("tui.home.section.teams"), Desc: ""})
		for _, t := range d.teams {
			team := t // capture
			items = append(items, homeItem{
				Icon:  "👥",
				Label: team.Name,
				Desc:  i18n.Tf("tui.home.team_desc", team.MemberCount, team.ActiveCount),
				Action: func() {
					if v.cfg.OnSelectTeam != nil {
						v.cfg.OnSelectTeam(team.ID, team.Name)
					}
				},
			})
		}
	}

	// ── Projects section (ADR-032 Phase 3) ──
	// One « Projets » section; « Ajouter un projet » is its last row.
	items = append(items, v.buildProjectsSection(d.projects)...)

	// Append the other static sections (Démarrer, Sessions, Système).
	items = append(items, v.buildLeftStaticItems()...)

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
	pad := 4 - iconWidth
	if pad < 1 {
		pad = 1
	}
	return widgets.SectionItem{
		MainText:      it.Icon + strings.Repeat(" ", pad) + it.Label,
		SecondaryText: it.Desc,
		Reference:     idx,
	}
}
