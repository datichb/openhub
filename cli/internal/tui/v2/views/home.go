package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// HomeView is the splash/landing view for the TUI shell.
// Displays an elegant welcome with quick-start hints.
type HomeView struct {
	app     *tview.Application
	content *tview.Flex
}

var _ View = (*HomeView)(nil)

// NewHomeView creates a new home splash view.
func NewHomeView() *HomeView {
	return &HomeView{}
}

// ID returns the view identifier.
func (v *HomeView) ID() string { return "home" }

// Title returns the display title.
func (v *HomeView) Title() string { return "Home" }

// Mount populates the content panel with the splash screen.
func (v *HomeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content

	// Main centered container
	center := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	center.SetBackgroundColor(theme.BgPanel)

	// Build splash content
	logo := fmt.Sprintf(`%s
    ___                   _   _       _     
   / _ \ _ __   ___ _ __ | | | |_   _| |__  
  | | | | '_ \ / _ \ '_ \| |_| | | | | '_ \ 
  | |_| | |_) |  __/ | | |  _  | |_| | |_) |
   \___/| .__/ \___|_| |_|_| |_|\__,_|_.__/ 
        |_|                                  
%s`, theme.ColorTag(theme.ActionHex), theme.TagColor)

	hints := fmt.Sprintf(`

%s─────────────────────────────────────────%s

  %sCtrl+P%s  ouvrir l'omnibar          %sEsc%s  retour
  %sstart%s   lancer une session         %squit%s quitter
  %sboard%s   kanban projet              %shelp%s raccourcis

%s─────────────────────────────────────────%s

  %sTapez n'importe quelle lettre pour chercher une commande%s
`,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor,
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor,
	)

	center.SetText(logo + hints)

	// Vertical centering with spacers
	content.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)
	content.AddItem(center, 20, 0, false)
	content.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)
}

// Unmount cleans up resources.
func (v *HomeView) Unmount() {
	v.app = nil
	v.content = nil
}

// StatusHints returns the contextual keybinding hints for the omnibar.
func (v *HomeView) StatusHints() string {
	return "Ctrl+P commande · Ctrl+Q quitter"
}

// HandleKey processes view-specific key events.
func (v *HomeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	// Home view has no contextual shortcuts — all runes go to omnibar
	return event
}
