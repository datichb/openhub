package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
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
func (v *HelpView) Title() string { return i18n.T("tui.help.title") }

// StatusHints returns keybinding hints.
func (v *HelpView) StatusHints() string {
	return fmt.Sprintf("Esc %s", i18n.T("tui.hints.back"))
}

// Mount builds the help display.
func (v *HelpView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetBorderPadding(1, 0, 2, 2)
	accent := theme.ColorTag(theme.ActiveMode.PrimaryHex)
	muted := theme.ColorTag(theme.TextSecondaryHex)
	c := theme.TagColor
	r := theme.TagReset

	var sb strings.Builder
	fmt.Fprintf(&sb, "\n  [::b]%s%s\n\n", i18n.T("tui.help.title"), r)
	fmt.Fprintf(&sb, "  %sOpenHub%s — %s\n\n", accent, c, i18n.T("tui.help.description"))
	fmt.Fprintf(&sb, "  %s%s%s\n", accent, i18n.T("tui.help.shortcuts_title"), c)
	fmt.Fprintf(&sb, "    Ctrl+P       %s\n", i18n.T("tui.help.shortcut_palette"))
	fmt.Fprintf(&sb, "    ?  / F1      %s\n", i18n.T("tui.help.shortcut_help"))
	fmt.Fprintf(&sb, "    j / k        %s\n", i18n.T("tui.help.shortcut_navigate"))
	fmt.Fprintf(&sb, "    Ctrl+T       %s\n", i18n.T("tui.help.shortcut_toggle"))
	fmt.Fprintf(&sb, "    Ctrl+Q       %s\n\n", i18n.T("tui.help.shortcut_quit"))
	fmt.Fprintf(&sb, "  %s%s%s\n", accent, i18n.T("tui.help.sessions_title"), c)
	fmt.Fprintf(&sb, "    %s\n", i18n.T("tui.help.sessions_line1"))
	fmt.Fprintf(&sb, "    %s\n\n", i18n.T("tui.help.sessions_line2"))
	fmt.Fprintf(&sb, "  %s%s%s  oh (OpenHub CLI)\n", muted, i18n.T("tui.help.version_label"), c)
	fmt.Fprintf(&sb, "  %s%s%s     https://github.com/datichb/openhub\n", muted, i18n.T("tui.help.docs_label"), c)

	tv.SetText(sb.String())

	content.AddItem(tv, 0, 1, true)
}

// Unmount cleans up resources.
func (v *HelpView) Unmount() { v.app = nil }

// HandleKey processes view-specific key events.
func (v *HelpView) HandleKey(event *tcell.EventKey) *tcell.EventKey { return event }
