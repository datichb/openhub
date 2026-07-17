package shell

import (
	"fmt"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Header is the 3-section header bar for the TUI shell.
type Header struct {
	root   *tview.Flex
	left   *tview.TextView
	center *tview.TextView
	right  *tview.TextView
}

// NewHeader creates a header with the project name.
func NewHeader(projectName string) *Header {
	h := &Header{
		left:   tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft),
		center: tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter),
		right:  tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignRight),
	}

	h.left.SetBackgroundColor(theme.BgElement)
	h.center.SetBackgroundColor(theme.BgElement)
	h.right.SetBackgroundColor(theme.BgElement)

	// Set logo
	h.left.SetText(fmt.Sprintf(" %s%s%s [::b]%s%s",
		theme.ColorTag(theme.ActionHex), theme.IconActive, theme.TagColor,
		projectName, theme.TagReset))

	h.root = tview.NewFlex()
	h.root.AddItem(h.left, 0, 1, false)
	h.root.AddItem(h.center, 0, 2, false)
	h.root.AddItem(h.right, 0, 1, false)
	h.root.SetBackgroundColor(theme.BgElement)

	return h
}

// Primitive returns the header flex for layout integration.
func (h *Header) Primitive() tview.Primitive {
	return h.root
}

// SetBreadcrumb updates the center section with navigation path.
func (h *Header) SetBreadcrumb(path string) {
	h.center.SetText(fmt.Sprintf("%s%s%s",
		theme.ColorTag(theme.AccentHex), path, theme.TagColor))
}

// SetMeta updates the right section with meta information.
func (h *Header) SetMeta(text string) {
	h.right.SetText(fmt.Sprintf("%s%s%s ",
		theme.ColorTag(theme.TextSecondaryHex), text, theme.TagColor))
}
