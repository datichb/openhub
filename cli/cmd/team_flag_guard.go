package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// refuseUnusedTeamFlag refuses --team where it would be ignored in silence:
// `oh team init` creates a team on a new repository (--team only names the
// team to set up again with --reconfigure), and `oh team rejoin` joins the
// team of the repository it is given. --team is the persistent flag of
// `oh team`, so cobra accepts it on both.
func refuseUnusedTeamFlag(cmd *cobra.Command) error {
	if f := cmd.Flags().Lookup("team"); f == nil || !f.Changed {
		return nil
	}
	switch cmd.Name() {
	case "init":
		if r, _ := cmd.Flags().GetBool("reconfigure"); r {
			return nil
		}
		return errors.New(i18n.Tf("v54z.team.flag_init", theme.Bold.Render("--reconfigure"), theme.Bold.Render("oh team rejoin --repo <url>")))
	case "rejoin":
		return errors.New(i18n.Tf("v54z.team.flag_rejoin", theme.Bold.Render("--repo <url>")))
	}
	return nil
}
