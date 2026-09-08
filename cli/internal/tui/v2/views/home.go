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

	// ── Interactive list (middle) — SectionedList with auto-skip headers ─
	v.list = widgets.NewSectionedList()
	v.list.SetApp(app)
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetBorderPadding(0, 0, 3, 3)

	var sectionItems []widgets.SectionItem
	for idx, item := range v.items {
		if item.Icon == "─" {
			sectionItems = append(sectionItems, widgets.SectionItem{
				MainText: item.Label,
				IsHeader: true,
			})
		} else {
			sectionItems = append(sectionItems, widgets.SectionItem{
				MainText:      fmt.Sprintf("%s  %s", item.Icon, item.Label),
				SecondaryText: item.Desc,
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

	// ── Shortcuts footer (bottom) ───────────────────────────────────────
	footer := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetScrollable(false)
	footer.SetBackgroundColor(theme.BgPanel)
	footer.SetText(buildShortcutsFooter())

	// ── Layout: centered column ─────────────────────────────────────────
	innerFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(theme.BgPanel)
	innerFlex.AddItem(logo, 9, 0, false)
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

func (v *HomeView) Unmount() {
	v.app = nil
	v.content = nil
	v.list = nil
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

	switch event.Key() {
	case tcell.KeyEnter:
		if idx, item, ok := v.list.CurrentItem(); ok && idx >= 0 {
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
