package cmd

import (
	"fmt"
	"io"
	"sync/atomic"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Team mode is a beta in 5.0: claims are released when a session stops, never
// expire, and a release lost offline stays in place (docs/guides/team-setup).
// The warning is shown once per process, on stderr by the team commands and
// among the notes of the first claim taken at launch (a TUI owns the screen).

// teamBetaShown is set once the warning was taken in this process.
var teamBetaShown atomic.Bool

// takeTeamBetaNotice reports whether the warning is still to be shown, and
// marks it as shown.
func takeTeamBetaNotice() bool {
	return teamBetaShown.CompareAndSwap(false, true)
}

// printTeamBetaWarning prints the beta warning of the team mode on w, once
// per process.
func printTeamBetaWarning(w io.Writer) {
	if !takeTeamBetaNotice() {
		return
	}
	fmt.Fprintf(w, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.T("cmd.team.beta.warning"))
}
