package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Gestion des plugins",
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginListCmd())
}

func pluginListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les plugins installés",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := GetApp()

			// For now, RTK is the only built-in plugin
			fmt.Fprintln(a.IO.Out, common.Bold.Render("Plugins intégrés:"))
			fmt.Fprintf(a.IO.Out, "  %s rtk — Token optimization (built-in)\n",
				common.SuccessStyle.Render(common.IconSuccess))

			fmt.Fprintln(a.IO.Out)
			fmt.Fprintln(a.IO.Out, common.Bold.Render("Plugins externes:"))
			fmt.Fprintf(a.IO.Out, "  %s\n", common.Subtitle.Render("Aucun plugin externe installé."))
			fmt.Fprintf(a.IO.Out, "  %s\n", common.Subtitle.Render("Répertoire: ~/.oh/plugins/"))

			return nil
		},
	}
}
