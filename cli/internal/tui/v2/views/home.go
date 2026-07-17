package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// HomeView is the default landing view (dashboard) for the TUI shell.
type HomeView struct {
	app     *tview.Application
	content *tview.Flex
}

var _ View = (*HomeView)(nil)

// NewHomeView creates a new home dashboard view.
func NewHomeView() *HomeView {
	return &HomeView{}
}

// ID returns the view identifier.
func (v *HomeView) ID() string { return "home" }

// Title returns the display title.
func (v *HomeView) Title() string { return "Home" }

// Mount populates the content panel with the dashboard widgets.
func (v *HomeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content

	// Welcome section
	welcome := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	welcome.SetBackgroundColor(theme.BgPanel)
	welcome.SetText(fmt.Sprintf("\n  %s%s%s  [::b]Bienvenue sur OpenHub%s\n",
		theme.ColorTag(theme.ActionHex), theme.IconActive, theme.TagColor,
		theme.TagReset))

	// Activity panel
	activity := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	activity.SetBackgroundColor(theme.BgPanel)
	activity.SetBorder(true)
	activity.SetBorderColor(theme.BorderNormal)
	activity.SetTitle(fmt.Sprintf(" %sActivité récente%s ",
		theme.ColorTag(theme.AccentHex), theme.TagColor))
	activity.SetTitleColor(theme.Accent)
	activity.SetText(fmt.Sprintf("\n  %s%s%s Aucune activité récente\n",
		theme.ColorTag(theme.TextMutedHex), theme.IconDot, theme.TagColor))

	// Quick actions hint
	quickHint := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	quickHint.SetBackgroundColor(theme.BgPanel)
	quickHint.SetText(fmt.Sprintf("\n%sUtilisez le menu à gauche ou Ctrl+N pour naviguer%s",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor))

	content.AddItem(welcome, 3, 0, false)
	content.AddItem(activity, 0, 1, false)
	content.AddItem(quickHint, 3, 0, false)
}

// Unmount cleans up resources.
func (v *HomeView) Unmount() {
	v.app = nil
	v.content = nil
}

// StatusHints returns the contextual keybinding hints.
func (v *HomeView) StatusHints() string {
	return "Ctrl+N menu · Ctrl+Q quitter"
}

// HandleKey processes view-specific key events.
func (v *HomeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	return event
}
