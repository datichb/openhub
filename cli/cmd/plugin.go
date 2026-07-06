package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
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

			fmt.Fprintln(a.IO.Out, common.Bold.Render(i18n.T("cmd.plugin.title_plugins")))
			icon := common.ErrorStyle.Render(common.IconError)
			state := i18n.T("cmd.plugin.state_not_installed")
			if status.Installed {
				icon = common.SuccessStyle.Render(common.IconSuccess)
				state = i18n.T("cmd.plugin.state_installed")
			}
			fmt.Fprintf(a.IO.Out, "  %s %s\n", icon, i18n.Tf("cmd.plugin.rtk_label", state))

			if status.BinaryFound {
				fmt.Fprintf(a.IO.Out, "    %s\n", i18n.Tf("cmd.plugin.rtk_binary_version", status.BinaryVer))
			} else {
				fmt.Fprintf(a.IO.Out, "    %s\n",
					common.WarningStyle.Render(i18n.T("cmd.plugin.rtk_binary_not_found")))
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
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"rtk"}, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			name := args[0]

			switch name {
			case "rtk":
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.plugin.installing"))

				if err := plugin.RTKInstall(); err != nil {
					return err
				}

				fmt.Fprintf(a.IO.Out, "%s %s\n",
					common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.plugin.installed"))
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.plugin.restart_hint")))
				return nil

			default:
				return fmt.Errorf("%s", i18n.Tf("cmd.plugin.unknown", name))
			}
		},
	}
}

func pluginRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm", "uninstall"},
		Short:   "Supprime un plugin",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			name := args[0]

			force, _ := cmd.Flags().GetBool("force")
			if !force {
				var confirm bool
				_ = huh.NewConfirm().
					Title(i18n.Tf("cmd.plugin.remove.confirm", name)).
					Value(&confirm).
					Run()
				if !confirm {
					return nil
				}
			}

			switch name {
			case "rtk":
				if err := plugin.RTKRemove(); err != nil {
					return err
				}
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.plugin.removed"))
				return nil

			default:
				return fmt.Errorf("%s", i18n.Tf("cmd.plugin.unknown_generic", name))
			}
		},
	}

	cmd.Flags().BoolP("force", "f", false, "Skip confirmation")
	return cmd
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
				fmt.Fprintf(a.IO.Out, "  %s %s\n",
					common.SuccessStyle.Render(common.IconSuccess), i18n.Tf("cmd.plugin.status_installed", status.Path))
			} else {
				fmt.Fprintf(a.IO.Out, "  %s %s\n",
					common.ErrorStyle.Render(common.IconError), i18n.T("cmd.plugin.status_not_installed"))
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.plugin.status_install_hint")))
			}

			if status.BinaryFound {
				fmt.Fprintf(a.IO.Out, "  %s %s\n",
					common.SuccessStyle.Render(common.IconSuccess), i18n.Tf("cmd.plugin.status_binary_version", status.BinaryVer))
				if !isVersionAtLeast(status.BinaryVer, plugin.RTKMinVersion) {
					fmt.Fprintf(a.IO.Out, "  %s %s\n",
						common.WarningStyle.Render(common.IconWarning), i18n.Tf("cmd.plugin.status_version_old", plugin.RTKMinVersion))
				}
			} else {
				fmt.Fprintf(a.IO.Out, "  %s %s\n",
					common.ErrorStyle.Render(common.IconError), i18n.T("cmd.plugin.status_binary_not_found"))
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.plugin.status_install_hint")))
			}

			return nil
		},
	}
}

// isVersionAtLeast is a helper used by the status command.
func isVersionAtLeast(version, minimum string) bool {
	return plugin.IsVersionAtLeast(version, minimum)
}
