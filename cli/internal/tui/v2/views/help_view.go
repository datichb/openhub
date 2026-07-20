package views

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// HelpView displays keyboard shortcuts and documentation links.
type HelpView struct {
	app *tview.Application
}

var _ View = (*HelpView)(nil)

// NewHelpView creates a new help view.
func NewHelpView() *HelpView { return &HelpView{} }

// ID returns the view identifier.
func (v *HelpView) ID() string { return "help" }

// Title returns the display title.
func (v *HelpView) Title() string { return "Aide" }

// StatusHints returns keybinding hints.
func (v *HelpView) StatusHints() string { return "Esc retour" }

// Mount builds the help display.
func (v *HelpView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetBorderPadding(1, 0, 2, 2)
	tv.SetText(fmt.Sprintf(`
  [::b]Raccourcis clavier%s

  %sNavigation%s
    j / ↓        Item suivant
    k / ↑        Item précédent
    Enter        Ouvrir / exécuter
    Esc          Retour
    Ctrl+N       Basculer menu ↔ contenu

  %sMenu%s
    h / ←        Fermer catégorie / remonter
    l / →        Ouvrir catégorie
    Space        Ouvrir/fermer catégorie
    g            Premier item
    G            Dernier item

  %sGlobal%s
    Ctrl+P       Palette de commandes (recherche rapide)
    Ctrl+Q       Quitter le TUI

  %sSessions%s
    Les sessions opencode s'ouvrent en plein écran.
    Le TUI se met en pause et reprend à la fin.

  %sVersion :%s  oh (OpenHub CLI)
  %sDocs :%s     https://github.com/datichb/openhub
`,
		theme.TagReset,
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor,
	))

	content.AddItem(tv, 0, 1, true)
}

// Unmount cleans up resources.
func (v *HelpView) Unmount() { v.app = nil }

// HandleKey processes view-specific key events.
func (v *HelpView) HandleKey(event *tcell.EventKey) *tcell.EventKey { return event }
