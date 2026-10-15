package views

import (
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// keyPairs are the key/label pairs of actions of the convention, for the
// « Touches » block of a detail pane (detailText.keys).
func keyPairs(actions ...keys.Action) []string {
	out := make([]string, 0, 2*len(actions))
	for _, a := range actions {
		out = append(out, a.Keys(), a.Label())
	}
	return out
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
