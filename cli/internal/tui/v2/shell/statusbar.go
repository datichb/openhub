package shell

import (
	"fmt"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// StatusBar is the 3-section status bar at the bottom of the shell.
type StatusBar struct {
	root   *tview.Flex
	left   *tview.TextView
	center *tview.TextView
	right  *tview.TextView
}

// NewStatusBar creates an empty status bar.
func NewStatusBar() *StatusBar {
	sb := &StatusBar{
		left:   tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft),
		center: tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter),
		right:  tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignRight),
	}

	sb.left.SetBackgroundColor(theme.BgPanel)
	sb.center.SetBackgroundColor(theme.BgPanel)
	sb.right.SetBackgroundColor(theme.BgPanel)

	sb.root = tview.NewFlex()
	sb.root.AddItem(sb.left, 0, 1, false)
	sb.root.AddItem(sb.center, 0, 3, false)
	sb.root.AddItem(sb.right, 0, 1, false)
	sb.root.SetBackgroundColor(theme.BgPanel)

	return sb
}

// Primitive returns the status bar flex for layout integration.
func (sb *StatusBar) Primitive() tview.Primitive {
	return sb.root
}

// SetView updates the left section with the current view indicator.
func (sb *StatusBar) SetView(name string) {
	sb.left.SetText(fmt.Sprintf(" %s%s%s %s%s%s",
		theme.ColorTag(theme.AccentHex), theme.IconActive, theme.TagColor,
		theme.ColorTag(theme.TextSecondaryHex), name, theme.TagColor))
}

// SetHints updates the center section with contextual keybinding hints.
func (sb *StatusBar) SetHints(hints string) {
	sb.center.SetText(fmt.Sprintf("%s%s%s",
		theme.ColorTag(theme.TextPrimaryHex), hints, theme.TagColor))
}

// SetInfo updates the right section with global information.
func (sb *StatusBar) SetInfo(text string) {
	sb.right.SetText(fmt.Sprintf("%s%s%s ",
		theme.ColorTag(theme.TextSecondaryHex), text, theme.TagColor))
}
