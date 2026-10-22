package views

import (
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// newTeamBetaLine is the one-line header of the team views: team mode is a
// beta in 5.0 (limits in docs/guides/team-setup).
func newTeamBetaLine() *tview.TextView {
	tv := tview.NewTextView().SetDynamicColors(true)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetText(tagged(theme.WarningHex, i18n.T("tui.team.beta.line")))
	return tv
}
