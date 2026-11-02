package cmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Commands deprecated in 5.1 and removed in v6 (`oh dashboard`, `oh metrics`)
// carry the Deprecated field of cobra, which hides them from the help
// (helpVisible), and warn in the language of the hub when they run, naming
// the view of the TUI that replaces them.

// deprecatedCommandMessage is the sentence of the warning: replacementKey is
// the key of the text of the Deprecated field of the command, which says what
// to use instead.
func deprecatedCommandMessage(old, replacementKey string) string {
	return i18n.Tf("cmd.deprecated_command.warn", old, i18n.T(replacementKey))
}

// warnDeprecatedCommand tells on w that the command old is deprecated.
func warnDeprecatedCommand(w io.Writer, old, replacementKey string) {
	cliout.Warn(w, deprecatedCommandMessage(old, replacementKey))
}

// silenceCobraDeprecation stops cobra from printing its own English sentence
// (« Command "x" is deprecated, … ») for the deprecated command about to
// run: the command warns in the language of the hub (warnDeprecatedCommand).
// The command stays marked deprecated for the help; the returned function
// restores the message.
func silenceCobraDeprecation(root *cobra.Command, args []string) (restore func()) {
	c, _, err := root.Find(args)
	if err != nil || c == nil || c.Deprecated == "" {
		return func() {}
	}
	msg := c.Deprecated
	c.Deprecated = ""
	return func() { c.Deprecated = msg }
}
