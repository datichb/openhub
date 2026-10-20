package views

import (
	"github.com/datichb/openhub/cli/internal/i18n"
)

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
