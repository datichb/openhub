package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
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
}

// HomeView is the splash/landing view for the TUI shell.
type HomeView struct {
	app     *tview.Application
	content *tview.Flex
	list    *tview.List
	shell   ShellAccess
	cfg     HomeViewConfig
	items   []homeItem
}

var _ View = (*HomeView)(nil)
var _ CommandProvider = (*HomeView)(nil)

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

	// ── Interactive list (middle) ────────────────────────────────────────
	v.list = tview.NewList()
	v.list.SetBackgroundColor(theme.BgPanel)
	v.list.SetMainTextColor(theme.FgPrimary)
	v.list.SetSecondaryTextColor(theme.FgSecondary)
	v.list.SetSelectedBackgroundColor(theme.BgElement)
	v.list.SetSelectedTextColor(theme.Action)
	v.list.SetHighlightFullLine(true)
	v.list.SetWrapAround(true)
	v.list.ShowSecondaryText(true)
	v.list.SetBorderPadding(0, 0, 4, 4)

	for _, item := range v.items {
		v.list.AddItem(
			fmt.Sprintf("%s  %s", item.Icon, item.Label),
			fmt.Sprintf("     %s", item.Desc),
			0, nil,
		)
	}

	v.list.SetSelectedFunc(func(idx int, _, _ string, _ rune) {
		v.executeItem(idx)
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
	hCenter.AddItem(innerFlex, 68, 0, true)
	hCenter.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)

	// Vertical centering: small top spacer + content + small bottom spacer
	content.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 1, 0, false)
	content.AddItem(hCenter, 0, 1, true)
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
		idx := v.list.GetCurrentItem()
		v.executeItem(idx)
		return nil
	}

	// Let tview.List handle j/k/arrows natively
	return event
}

// ContextCommands exposes home items to the omnibar.
func (v *HomeView) ContextCommands() []ContextCommand {
	var cmds []ContextCommand
	for _, it := range v.items {
		cmd := ContextCommand{
			ID:          "home." + strings.ToLower(strings.ReplaceAll(it.Label, " ", "-")),
			Label:       it.Label,
			Description: it.Desc,
			Category:    "Navigation",
		}
		if it.ViewID != "" {
			viewID := it.ViewID
			cmd.Action = func() {
				if v.shell != nil {
					v.shell.NavigateTo(viewID)
				}
			}
		} else if it.Action != nil {
			action := it.Action
			cmd.Action = action
			cmd.RunsDirect = true
		}
		cmds = append(cmds, cmd)
	}
	return cmds
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
	items := []homeItem{
		// ── Navigation ──
		{Icon: "⊞", Label: "Board", Desc: "Kanban du projet actif", ViewID: "board"},
		{Icon: "◈", Label: "Projets", Desc: "Gérer les projets", ViewID: "projects.list"},
		{Icon: "⊛", Label: "Worktrees", Desc: "Git worktrees", ViewID: "worktrees"},
		{Icon: "◎", Label: "Métriques", Desc: "Statistiques d'usage", ViewID: "metrics"},
		{Icon: "⊟", Label: "Config", Desc: "Configuration du hub", ViewID: "settings"},
	}

	// ── Actions rapides ──
	if v.cfg.OnLaunchSession != nil {
		launch := v.cfg.OnLaunchSession
		items = append(items,
			homeItem{Icon: "▶", Label: "Start", Desc: "Lancer une session", Action: func() { launch("developer") }},
			homeItem{Icon: "◉", Label: "Audit", Desc: "Audit multi-domaine", Action: func() { launch("auditor") }},
			homeItem{Icon: "◈", Label: "Review", Desc: "Code review", Action: func() { launch("reviewer") }},
			homeItem{Icon: "◆", Label: "Debug", Desc: "Session de debug", Action: func() { launch("developer", "--debug") }},
		)
	}

	return items
}

// ─────────────────────────────────────────────────────────────────────────────
// Static rendering helpers
// ─────────────────────────────────────────────────────────────────────────────

// visibleWidth returns the terminal display width of s (in cells), ignoring
// tview colour tags of the form [#xxxxxx], [-], [::b], etc.
func visibleWidth(s string) int {
	inTag := false
	width := 0
	for _, r := range s {
		switch {
		case r == '[' && !inTag:
			inTag = true
		case r == ']' && inTag:
			inTag = false
		case !inTag:
			width += runewidth.RuneWidth(r)
		}
	}
	return width
}

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

	return fmt.Sprintf("\n%s%sCtrl+P%s commandes  %s?%s aide  %sCtrl+T%s mode projet  %sCtrl+Q%s quitter",
		muted,
		accent, reset,
		accent, reset,
		accent, reset,
		accent, reset,
	)
}
