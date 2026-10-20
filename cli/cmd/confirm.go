package cmd

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// confirmed reads the answer to a yes/no question typed on the keyboard. It
// is the only reader of such answers in the CLI (U-11): o, oui, y and yes, in
// any case, are yes; an empty answer takes the default of the question; any
// other answer is no. (The TUI confirms with keys, not typed answers.)
func confirmed(answer string, defaultYes bool) bool {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "":
		return defaultYes
	case "o", "oui", "y", "yes":
		return true
	}
	return false
}

// yesNoHint is the hint of a yes/no question in the language of the hub:
// "[O/n]" (yes by default) or "[o/N]", "[Y/n]" or "[y/N]" in English.
func yesNoHint(defaultYes bool) string {
	if defaultYes {
		return i18n.T("cmd.confirm.hint_yes")
	}
	return i18n.T("cmd.confirm.hint_no")
}
