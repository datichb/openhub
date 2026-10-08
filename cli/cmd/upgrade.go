package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"

	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/selfupdate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [component]",
	Short: "Met à jour oh",
	Long:  i18n.T("cmd.upgrade.long"),
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.AddCommand(upgradeOhCmd())
}

func upgradeOhCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oh [version]",
		Short: "Met à jour le binaire oh lui-même",
		Long: `Télécharge et installe la dernière version du binaire oh en remplacement atomique.
Fonctionne pour les installations via install.sh ou téléchargement direct.
Les utilisateurs Homebrew peuvent utiliser: brew upgrade openhub`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			checkOnly, _ := cmd.Flags().GetBool("check")

			var targetVersion string
			if len(args) > 0 {
				targetVersion = strings.TrimPrefix(args[0], "v")
			}

			current := buildinfo.Version

			// Always check latest first for comparison
			fmt.Fprintf(a.IO.Out, "%s %s\n", theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.upgrade.oh.checking"))

			release, err := selfupdate.LatestRelease()
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.upgrade.oh.check_failed"), err)
			}
			latest := release.Version()

			fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.upgrade.oh.current", current))
			fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.upgrade.oh.latest", latest))

			plan := planUpgrade(current, latest, targetVersion)
			if plan.version == "" {
				msg := i18n.Tf("cmd.upgrade.oh.up_to_date", current)
				if plan.status == versionAhead {
					msg = i18n.Tf("cmd.upgrade.oh.ahead", current, latest)
				}
				fmt.Fprintf(a.IO.Out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), msg)
				return nil
			}
			if plan.downgrade {
				fmt.Fprintf(a.IO.Out, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning),
					i18n.Tf("cmd.upgrade.oh.downgrade", current, plan.version))
			}

			if checkOnly {
				if !plan.downgrade {
					fmt.Fprintf(a.IO.Out, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning),
						i18n.Tf("cmd.upgrade.oh.available", current, plan.version))
					fmt.Fprintf(a.IO.Out, "  %s\n", i18n.T("cmd.upgrade.oh.run_hint"))
				}
				return nil
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n", theme.SuccessStyle.Render(theme.IconArrow), i18n.Tf("cmd.upgrade.oh.downloading", plan.version))

			var lastPercent int
			binPath, err := selfupdate.Update(plan.version, func(downloaded, total int64) {
				if total > 0 {
					percent := int(downloaded * 100 / total)
					if percent != lastPercent && percent%5 == 0 {
						lastPercent = percent
						fmt.Fprintf(a.IO.Out, "\r  %d%% (%d/%d MB)",
							percent, downloaded/1024/1024, total/1024/1024)
					}
				}
			})
			if err != nil {
				return fmt.Errorf("%s: %w", i18n.T("cmd.upgrade.oh.failed"), err)
			}

			fmt.Fprintln(a.IO.Out)
			fmt.Fprintf(a.IO.Out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.upgrade.oh.installed", plan.version, binPath))
			fmt.Fprintf(a.IO.Out, "  %s\n", i18n.T("cmd.upgrade.oh.restart"))

			return nil
		},
	}

	cmd.Flags().Bool("check", false, "Vérifier uniquement si une mise à jour est disponible")
	return cmd
}
