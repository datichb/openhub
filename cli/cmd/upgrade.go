package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/selfupdate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [component]",
	Short: "Met à jour un composant (opencode, oh)",
	Long: `Met à jour opencode ou oh vers la dernière version ou une version spécifique.

Exemples:
  oh upgrade opencode          Installe la dernière version d'opencode
  oh upgrade opencode 1.17.12  Installe une version spécifique d'opencode
  oh upgrade oh                Met à jour le binaire oh lui-même
  oh upgrade oh --check        Vérifie si une mise à jour est disponible`,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.AddCommand(upgradeOpencodeCmd())
	upgradeCmd.AddCommand(upgradeOhCmd())
}

func upgradeOpencodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "opencode [version]",
		Short: "Met à jour le binaire opencode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			installDir := a.Config.Opencode.InstallDir

			var targetVersion string
			if len(args) > 0 {
				targetVersion = strings.TrimPrefix(args[0], "v")
			}

			currentVersion := opencode.InstalledVersion(installDir)
			if currentVersion != "" {
				fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.upgrade.current_version", currentVersion))
			}

			if targetVersion == "" {
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.upgrade.checking"))

				release, err := opencode.LatestRelease()
				if err != nil {
					return fmt.Errorf("%s", i18n.Tf("cmd.upgrade.check_failed", err))
				}
				targetVersion = release.Version()

				if currentVersion == targetVersion {
					fmt.Fprintf(a.IO.Out, "%s %s\n",
						theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.upgrade.already_uptodate", currentVersion))
					return nil
				}
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconArrow), i18n.Tf("cmd.upgrade.downloading", targetVersion))

			var lastPercent int
			binPath, err := opencode.Download(targetVersion, installDir, func(downloaded, total int64) {
				if total > 0 {
					percent := int(downloaded * 100 / total)
					if percent != lastPercent && percent%5 == 0 {
						lastPercent = percent
						fmt.Fprintf(a.IO.Out, "\r  %s",
							i18n.Tf("cmd.upgrade.progress", percent, downloaded/1024/1024, total/1024/1024))
					}
				}
			})
			if err != nil {
				return fmt.Errorf("%s", i18n.Tf("cmd.upgrade.download_failed", err))
			}

			fmt.Fprintln(a.IO.Out)
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.upgrade.installed", targetVersion, binPath))

			return nil
		},
	}
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
			fmt.Fprintf(a.IO.Out, "%s Vérification des mises à jour...\n",
				theme.SuccessStyle.Render(theme.IconArrow))

			release, err := selfupdate.LatestRelease()
			if err != nil {
				return fmt.Errorf("impossible de vérifier les mises à jour: %w", err)
			}
			latest := release.Version()

			fmt.Fprintf(a.IO.Out, "  Version actuelle : %s\n", current)
			fmt.Fprintf(a.IO.Out, "  Dernière version : %s\n", latest)

			resolvedVersion := targetVersion
			if resolvedVersion == "" {
				resolvedVersion = latest
			}

			// Already up to date
			if current == resolvedVersion && targetVersion == "" {
				fmt.Fprintf(a.IO.Out, "%s oh est déjà à jour (%s)\n",
					theme.SuccessStyle.Render(theme.IconSuccess), current)
				return nil
			}

			if checkOnly {
				if current != resolvedVersion {
					fmt.Fprintf(a.IO.Out, "%s Mise à jour disponible : %s → %s\n",
						theme.WarningStyle.Render(theme.IconWarning), current, resolvedVersion)
					fmt.Fprintf(a.IO.Out, "  Lancez 'oh upgrade oh' pour mettre à jour.\n")
				}
				return nil
			}

			fmt.Fprintf(a.IO.Out, "%s Téléchargement de oh v%s...\n",
				theme.SuccessStyle.Render(theme.IconArrow), resolvedVersion)

			var lastPercent int
			binPath, err := selfupdate.Update(resolvedVersion, func(downloaded, total int64) {
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
				return fmt.Errorf("mise à jour échouée: %w", err)
			}

			fmt.Fprintln(a.IO.Out)
			fmt.Fprintf(a.IO.Out, "%s oh v%s installé avec succès → %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess), resolvedVersion, binPath)
			fmt.Fprintf(a.IO.Out, "  Redémarrez le terminal pour prendre en compte la nouvelle version.\n")

			return nil
		},
	}

	cmd.Flags().Bool("check", false, "Vérifier uniquement si une mise à jour est disponible")
	return cmd
}

