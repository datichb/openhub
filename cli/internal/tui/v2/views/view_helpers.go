package views

import (
	"fmt"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// tagged colours s with a theme colour (hex), then resets the foreground
// (theme.Paint: nothing under NO_COLOR).
func tagged(hex, s string) string {
	return theme.Paint(hex, s)
}

// bracketed is s between square brackets as plain text: « [Détail] », « [f] ».
// The brackets are escaped so that tview does not take them for a tag.
func bracketed(s string) string {
	return tview.Escape(fmt.Sprintf("%c%s%c", '[', s, ']'))
}

// errorText is the toast of a failed operation (« Erreur : … »).
func errorText(err error) string {
	return i18n.Tf("tui.settings.error_detail", err.Error())
}

// confirmDiscard runs reload at once when nothing is unsaved, and after a
// confirmation otherwise (r drops the unsaved changes of a view).
func confirmDiscard(sh ShellAccess, dirty bool, reload func()) {
	if !dirty || sh == nil {
		reload()
		return
	}
	sh.Confirm(i18n.T("tui.config.discard_title"), i18n.T("tui.config.discard_detail"), true, reload)
}
