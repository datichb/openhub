package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// The commands deprecated in 5.0 were removed in 5.1. Their name stays as a
// hidden stub that only says what replaces it and exits with ExitUsage, as
// `oh deploy` does: without it, cobra answers « unknown command » (and
// `oh takeover-brief enrich x` would print the help of its parent with exit
// code 0). The options removed with them (-p, -m, --force…) get the plain
// « unknown option » of pflag.

// removedCommand is a command removed in 5.1 and the key of the text that
// names its replacement.
type removedCommand struct {
	path        []string
	replacement string
}

var removedCommands = []removedCommand{
	{[]string{"start"}, "cmd.removed.start"},
	{[]string{"audit"}, "cmd.removed.audit"},
	{[]string{"review"}, "cmd.removed.review"},
	{[]string{"review", "feedback"}, "cmd.removed.review_feedback"},
	{[]string{"debug"}, "cmd.removed.debug"},
	{[]string{"service"}, "cmd.removed.service"},
	{[]string{"service", "setup"}, "cmd.removed.service_setup"},
	{[]string{"service", "remove"}, "cmd.removed.service_remove"},
	{[]string{"takeover-brief", "enrich"}, "cmd.removed.takeover_brief_enrich"},
}

func init() {
	stubs := map[string]*cobra.Command{}
	for _, r := range removedCommands {
		stub := newRemovedCommand(r)
		stubs[strings.Join(r.path, " ")] = stub
		parentPath := strings.Join(r.path[:len(r.path)-1], " ")
		switch parentPath {
		case "":
			rootCmd.AddCommand(stub)
		case "takeover-brief":
			takeoverBriefCmd.AddCommand(stub)
		default:
			stubs[parentPath].AddCommand(stub)
		}
	}
}

func newRemovedCommand(r removedCommand) *cobra.Command {
	old := "oh " + strings.Join(r.path, " ")
	c := &cobra.Command{
		Use:                r.path[len(r.path)-1],
		Short:              i18n.T("cmd.removed.short"),
		Hidden:             true, // removed: not in the help, still names its replacement
		DisableFlagParsing: true, // former options accepted
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := cmd.ErrOrStderr()
			fmt.Fprintf(w, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.removed.message", old))
			fmt.Fprintf(w, "  %s\n", i18n.T(r.replacement))
			return &ExitError{Code: ExitUsage} // the message is printed: the script must notice
		},
	}
	return withInitLevel(c, levelNone)
}
