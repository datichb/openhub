package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/plugin"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Gestion des plugins",
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginListCmd())
	pluginCmd.AddCommand(pluginInstallCmd())
	pluginCmd.AddCommand(pluginRemoveCmd())
	pluginCmd.AddCommand(pluginStatusCmd())
}

func pluginListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les plugins disponibles",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()

			status := plugin.RTKStatus()

			fmt.Fprintln(a.IO.Out, common.Bold.Render("Plugins:"))
			icon := common.ErrorStyle.Render(common.IconError)
			state := "non installé"
			if status.Installed {
				icon = common.SuccessStyle.Render(common.IconSuccess)
				state = "installé"
			}
			fmt.Fprintf(a.IO.Out, "  %s rtk — Token optimization (%s)\n", icon, state)

			if status.BinaryFound {
				fmt.Fprintf(a.IO.Out, "    binaire rtk: v%s\n", status.BinaryVer)
			} else {
				fmt.Fprintf(a.IO.Out, "    binaire rtk: %s\n",
					common.WarningStyle.Render("non trouvé"))
			}

			return nil
		},
	}
}

func pluginInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <name>",
		Short: "Installe un plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			name := args[0]

			switch name {
			case "rtk":
				fmt.Fprintf(a.IO.Out, "%s Installation du plugin RTK...\n",
					common.SuccessStyle.Render(common.IconArrow))

				if err := plugin.RTKInstall(); err != nil {
					return err
				}

				fmt.Fprintf(a.IO.Out, "%s Plugin RTK installé.\n",
					common.SuccessStyle.Render(common.IconSuccess))
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Redémarrez opencode pour activer le plugin."))
				return nil

			default:
				return fmt.Errorf("plugin inconnu: %s (disponible: rtk)", name)
			}
		},
	}
}

func pluginRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm", "uninstall"},
		Short:   "Supprime un plugin",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			name := args[0]

			switch name {
			case "rtk":
				if err := plugin.RTKRemove(); err != nil {
					return err
				}
				fmt.Fprintf(a.IO.Out, "%s Plugin RTK supprimé.\n",
					common.SuccessStyle.Render(common.IconSuccess))
				return nil

			default:
				return fmt.Errorf("plugin inconnu: %s", name)
			}
		},
	}
}

func pluginStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Affiche l'état des plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()

			status := plugin.RTKStatus()

			fmt.Fprintln(a.IO.Out, common.Title.Render("  Plugin RTK  "))
			fmt.Fprintln(a.IO.Out)

			if status.Installed {
				fmt.Fprintf(a.IO.Out, "  %s Installé: %s\n",
					common.SuccessStyle.Render(common.IconSuccess), status.Path)
			} else {
				fmt.Fprintf(a.IO.Out, "  %s Non installé\n",
					common.ErrorStyle.Render(common.IconError))
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Installez avec: oh plugin install rtk"))
			}

			if status.BinaryFound {
				fmt.Fprintf(a.IO.Out, "  %s Binaire rtk: v%s\n",
					common.SuccessStyle.Render(common.IconSuccess), status.BinaryVer)
				if !isVersionAtLeast(status.BinaryVer, plugin.RTKMinVersion) {
					fmt.Fprintf(a.IO.Out, "  %s Version trop ancienne (min: %s)\n",
						common.WarningStyle.Render(common.IconWarning), plugin.RTKMinVersion)
				}
			} else {
				fmt.Fprintf(a.IO.Out, "  %s Binaire rtk non trouvé\n",
					common.ErrorStyle.Render(common.IconError))
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Installez avec: brew install rtk"))
			}

			return nil
		},
	}
}

// isVersionAtLeast is a helper used by the status command.
func isVersionAtLeast(version, minimum string) bool {
	return plugin.IsVersionAtLeast(version, minimum)
}
