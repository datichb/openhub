package cmd

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/plugin"
	"github.com/datichb/openhub/cli/internal/tui/components/floating"
	"github.com/datichb/openhub/cli/internal/tui/theme"
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
			registry := plugin.NewRegistry()

			fmt.Fprintln(a.IO.Out, theme.Bold.Render(i18n.T("cmd.plugin.title_plugins")))
			fmt.Fprintln(a.IO.Out)

			for _, p := range registry.All() {
				status := p.Status()
				icon := theme.ErrorStyle.Render(theme.IconError)
				state := i18n.T("cmd.plugin.state_not_installed")
				if status.Installed {
					icon = theme.SuccessStyle.Render(theme.IconSuccess)
					state = i18n.T("cmd.plugin.state_installed")
				}
				fmt.Fprintf(a.IO.Out, "  %s %s — %s\n", icon, p.Name(), state)
				fmt.Fprintf(a.IO.Out, "    %s\n", p.Description())
				if status.BinaryFound {
					fmt.Fprintf(a.IO.Out, "    binary: v%s\n", status.BinaryVer)
				}
				fmt.Fprintln(a.IO.Out)
			}

			fmt.Fprintln(a.IO.Out, theme.Subtitle.Render(
				"  Répertoire plugins communautaires: "+plugin.CommunityPluginsDir()))

			return nil
		},
	}
}

func pluginInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install <name>",
		Short: "Installe un plugin (built-in ou communautaire)",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			registry := plugin.NewRegistry()
			names := make([]string, 0)
			for _, p := range registry.All() {
				names = append(names, p.Name())
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			name := args[0]

			registry := plugin.NewRegistry()
			p := registry.Get(name)
			if p == nil {
				return fmt.Errorf("%s", i18n.Tf("cmd.plugin.unknown", name))
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.plugin.installing"))

			if err := p.Install(context.Background(), ""); err != nil {
				return err
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.plugin.installed"))
			fmt.Fprintln(a.IO.Out, theme.Subtitle.Render("  "+i18n.T("cmd.plugin.restart_hint")))
			return nil
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

			registry := plugin.NewRegistry()
			p := registry.Get(name)
			if p == nil {
				return fmt.Errorf("%s", i18n.Tf("cmd.plugin.unknown_generic", name))
			}

			force, _ := cmd.Flags().GetBool("force")
			if !force {
				var confirm bool
				_ = floating.Run(floating.Config{
					Title: i18n.T("cmd.plugin.remove.short"),
					Form: theme.NewForm(huh.NewGroup(huh.NewConfirm().
						Title(i18n.Tf("cmd.plugin.remove.confirm", name)).
						Value(&confirm))),
				})
				if !confirm {
					return nil
				}
			}

			if err := p.Uninstall(context.Background(), ""); err != nil {
				return err
			}
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.plugin.removed"))
			return nil
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
			registry := plugin.NewRegistry()

			fmt.Fprintln(a.IO.Out, theme.Title.Render("  Plugin Status  "))
			fmt.Fprintln(a.IO.Out)

			for _, p := range registry.All() {
				status := p.Status()

				fmt.Fprintln(a.IO.Out, theme.Bold.Render("  "+p.Name()))
				if status.Installed {
					fmt.Fprintf(a.IO.Out, "  %s %s\n",
						theme.SuccessStyle.Render(theme.IconSuccess),
						i18n.Tf("cmd.plugin.status_installed", status.Path))
				} else {
					fmt.Fprintf(a.IO.Out, "  %s %s\n",
						theme.ErrorStyle.Render(theme.IconError), i18n.T("cmd.plugin.status_not_installed"))
					fmt.Fprintln(a.IO.Out, theme.Subtitle.Render("  "+i18n.T("cmd.plugin.status_install_hint")))
				}

				if status.BinaryFound {
					fmt.Fprintf(a.IO.Out, "  %s %s\n",
						theme.SuccessStyle.Render(theme.IconSuccess),
						i18n.Tf("cmd.plugin.status_binary_version", status.BinaryVer))
					if p.Name() == "rtk" && !isVersionAtLeast(status.BinaryVer, plugin.RTKMinVersion) {
						fmt.Fprintf(a.IO.Out, "  %s %s\n",
							theme.WarningStyle.Render(theme.IconWarning),
							i18n.Tf("cmd.plugin.status_version_old", plugin.RTKMinVersion))
					}
				}
				fmt.Fprintln(a.IO.Out)
			}

			return nil
		},
	}
}

// isVersionAtLeast is a helper used by the status command.
func isVersionAtLeast(version, minimum string) bool {
	return plugin.IsVersionAtLeast(version, minimum)
}
