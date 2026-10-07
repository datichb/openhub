package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// `oh deploy` and `oh sync` were removed in v5 (D14, P3-T23): sessions run
// from a bundle built at launch, outside the project. The commands stay
// during v5.x as aliases that explain the migration.

func init() {
	for _, name := range []string{"deploy", "sync"} {
		name := name
		rootCmd.AddCommand(&cobra.Command{
			Use:                name,
			Short:              i18n.T("cmd.deploy.removed.short"),
			Hidden:             true, // removed: not in the help, still explains the migration
			DisableFlagParsing: true, // former flags (--check, --diff, --all…) accepted
			RunE: func(cmd *cobra.Command, _ []string) error {
				deployRemoved(cmd, "oh "+name)
				return nil
			},
		})
	}
}

// deployRemoved prints what replaces the former deployment commands.
func deployRemoved(cmd *cobra.Command, old string) {
	w := cmd.ErrOrStderr()
	fmt.Fprintf(w, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.deploy.removed.message", old))
	fmt.Fprintf(w, "  %s\n", i18n.T("cmd.deploy.removed.bundle"))
	fmt.Fprintf(w, "  %s\n", i18n.T("cmd.deploy.removed.cleanup"))
}
