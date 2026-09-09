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
}

var _ View = (*HomeView)(nil)

func NewHomeView(cfg HomeViewConfig) *HomeView {
	return &HomeView{cfg: cfg}
}

// SetShell injects the shell for navigation and toasts.
func (v *HomeView) SetShell(s ShellAccess) { v.shell = s }

func (v *HomeView) ID() string    { return "home" }
func (v *HomeView) Title() string { return "Home" }

func (v *HomeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content

	v.items = v.buildItems()

	// ── Logo (top) ──────────────────────────────────────────────────────
	logo := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetScrollable(false)
	logo.SetBackgroundColor(theme.BgPanel)
	logo.SetText(buildLogo())

	// ── Footer ──────────────────────────────────────────────────────────
	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(false)
	footer.SetBackgroundColor(theme.BgPanel)
	footer.SetText(buildShortcutsFooter())

	// ── Split items for dual-column: left = Teams+System, right = Projects ──
	leftItems, rightItems := v.splitItems()

	onSelect := func(_ int, item widgets.SectionItem) {
		if ref, ok := item.Reference.(int); ok {
			v.executeItem(ref)
		}
	}

	// ── Adaptive layout with resize ─────────────────────────────────────
	var currentResult *homeFlexResult
	buildFn := func(width int) homeFlexResult {
		r := buildHomeLayout(width, homeFlexConfig{
			App:          app,
			Header:       logo,
			HeaderHeight: 9,
			Footer:       footer,
			FooterHeight: 4,
			LeftItems:    leftItems,
			RightItems:   rightItems,
			OnSelect:     onSelect,
		})
		currentResult = &r
		return r
	}

	initial := adaptiveHomeMount(app, content, buildFn)
	_ = currentResult // keep in scope for HandleKey closure

	if initial.Dual != nil {
		v.dual = initial.Dual
		v.list = initial.Dual.left
	} else {
		v.dual = nil
		v.list = initial.SingleList
	}
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
			if ref, refOk := item.Reference.(int); refOk {
				v.executeItem(ref)
			}
		}
		return nil
	}

	// Let SectionedList handle j/k/arrows via its InputCapture
	return event
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

func (v *HomeView) buildItems() []homeItem {
	var items []homeItem

	// ── Teams section (ADR-032 Phase 3) ──
	if v.cfg.ListTeams != nil {
		teams := v.cfg.ListTeams()
		if len(teams) > 0 {
			items = append(items, homeItem{Icon: "─", Label: "Équipes", Desc: ""})
			for _, t := range teams {
				team := t // capture
				desc := fmt.Sprintf("%d membres · %d en cours", team.MemberCount, team.ActiveCount)
				items = append(items, homeItem{
					Icon:  "◫",
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
			items = append(items, homeItem{Icon: "─", Label: "Projets", Desc: ""})
			for _, p := range projects {
				proj := p // capture
				desc := proj.Path
				if proj.ActiveCount > 0 {
					desc = fmt.Sprintf("%d en cours · %s", proj.ActiveCount, proj.Path)
				}
				if proj.ActiveBranch != "" {
					desc += fmt.Sprintf(" · %s", proj.ActiveBranch)
				}
				items = append(items, homeItem{
					Icon:  "◈",
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

	// ── System / navigation ──
	items = append(items, homeItem{Icon: "─", Label: "Système", Desc: ""})
	items = append(items,
		homeItem{Icon: "⊟", Label: "Settings", Desc: "Configuration du hub", ViewID: "settings"},
		homeItem{Icon: "◎", Label: "Métriques", Desc: "Statistiques d'usage", ViewID: "metrics"},
		homeItem{Icon: "⊛", Label: "Worktrees", Desc: "Git worktrees", ViewID: "worktrees"},
	)

	return items
}

// splitItems distributes home items into left/right columns for dual mode.
// Left: Équipes + Système. Right: Projets.
func (v *HomeView) splitItems() (left, right []widgets.SectionItem) {
	// Find the "Projets" section boundary
	projIdx := -1
	sysIdx := -1
	for i, it := range v.items {
		if it.Icon == "─" && it.Label == "Projets" {
			projIdx = i
		}
		if it.Icon == "─" && it.Label == "Système" {
			sysIdx = i
		}
	}

	// Left: everything except Projets section
	// Right: Projets section
	for i, it := range v.items {
		si := homeItemToSectionItem(it, i)
		if projIdx >= 0 && i >= projIdx && (sysIdx < 0 || i < sysIdx) {
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
	return widgets.SectionItem{
		MainText:      fmt.Sprintf("%s  %s", it.Icon, it.Label),
		SecondaryText: it.Desc,
		Reference:     idx,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Static rendering helpers
// ─────────────────────────────────────────────────────────────────────────────

func buildLogo() string {
	action := theme.ColorTag(theme.ActionHex)
	reset := theme.TagColor

	var b strings.Builder
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "%s ██████╗ ██████╗ ███████╗███╗   ██╗██╗  ██╗██╗   ██╗██████╗%s\n", action, reset)
	fmt.Fprintf(&b, "%s██╔═══██╗██╔══██╗██╔════╝████╗  ██║██║  ██║██║   ██║██╔══██╗%s\n", action, reset)
	fmt.Fprintf(&b, "%s██║   ██║██████╔╝█████╗  ██╔██╗ ██║███████║██║   ██║██████╔╝%s\n", action, reset)
	fmt.Fprintf(&b, "%s██║   ██║██╔═══╝ ██╔══╝  ██║╚██╗██║██╔══██║██║   ██║██╔══██╗%s\n", action, reset)
	fmt.Fprintf(&b, "%s╚██████╔╝██║     ███████╗██║ ╚████║██║  ██║╚██████╔╝██████╔╝%s\n", action, reset)
	fmt.Fprintf(&b, "%s ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚═════╝%s\n", action, reset)
	return b.String()
}

func buildShortcutsFooter() string {
	accent := theme.ColorTag(theme.AccentHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	return fmt.Sprintf("\n%s%sCtrl+P%s commandes  %s?%s aide  %sCtrl+T%s mode équipe  %sCtrl+Q%s quitter",
		muted,
		accent, reset,
		accent, reset,
		accent, reset,
		accent, reset,
	)
}
