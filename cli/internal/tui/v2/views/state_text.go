package views

import (
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// stateKind is what a view has to say when it has no data to show.
type stateKind int

const (
	// stateLoading: the data is being read (muted).
	stateLoading stateKind = iota
	// stateEmpty: the data was read and there is nothing in it (muted).
	stateEmpty
	// stateError: the data could not be read (✗, red).
	stateError
)

// stateText is the text, with tview tags, of a view without data: the same
// colours and the same wording on every screen (keys tui.state.*).
//
// msg is the message of the view, already translated: « Chargement des
// services MCP… », « Aucun déploiement à nettoyer ». Empty, the common text of
// the state is used. For stateError msg is the reason (err.Error()) and is put
// in « Impossible de charger : … »; the texts that come from outside are
// escaped.
func stateText(kind stateKind, msg string) string {
	switch kind {
	case stateError:
		return tagged(theme.ErrorHex, theme.SymFailure+" "+tview.Escape(i18n.Tf("tui.state.error", msg)))
	case stateEmpty:
		if msg == "" {
			msg = i18n.T("tui.state.empty")
		}
	default:
		if msg == "" {
			msg = i18n.T("tui.state.loading")
		}
	}
	return muted(tview.Escape(msg))
}

// stateView is a text view that holds stateText, indented as the first lines
// of the views are (a placeholder put in a view's content while its data is
// being read, replaced when it arrives).
func stateView(kind stateKind, msg string) *tview.TextView {
	tv := tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(theme.BgPanel)
	setState(tv, kind, msg)
	return tv
}

// setState puts a state in a text view.
func setState(tv *tview.TextView, kind stateKind, msg string) {
	tv.SetText("\n  " + stateText(kind, msg))
}
