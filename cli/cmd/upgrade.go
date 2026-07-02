package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [component]",
	Short: "Met à jour un composant (opencode)",
	Long: `Met à jour opencode vers la dernière version ou une version spécifique.

Exemples:
  oh upgrade opencode          Installe la dernière version
  oh upgrade opencode 1.17.12  Installe une version spécifique`,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.AddCommand(upgradeOpencodeCmd())
}

func upgradeOpencodeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "opencode [version]",
		Short: "Met à jour le binaire opencode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			installDir := a.Config.Opencode.InstallDir

			// Determine target version
			var targetVersion string
			if len(args) > 0 {
				targetVersion = strings.TrimPrefix(args[0], "v")
			}

			// Check current version
			currentVersion := opencode.InstalledVersion(installDir)
			if currentVersion != "" {
				fmt.Fprintf(a.IO.Out, "  Version actuelle: %s\n", currentVersion)
			}

			// Resolve latest if needed
			if targetVersion == "" {
				fmt.Fprintf(a.IO.Out, "%s Recherche de la dernière version...\n",
					common.SuccessStyle.Render(common.IconArrow))

				release, err := opencode.LatestRelease()
				if err != nil {
					return fmt.Errorf("impossible de vérifier les mises à jour: %w", err)
				}
				targetVersion = release.Version()

				if currentVersion == targetVersion {
					fmt.Fprintf(a.IO.Out, "%s opencode est déjà à jour (v%s)\n",
						common.SuccessStyle.Render(common.IconSuccess), currentVersion)
					return nil
				}
			}

			fmt.Fprintf(a.IO.Out, "%s Téléchargement de opencode v%s...\n",
				common.SuccessStyle.Render(common.IconArrow), targetVersion)

			// Download with progress
			var lastPercent int
			binPath, err := opencode.Download(targetVersion, installDir, func(downloaded, total int64) {
				if total > 0 {
					percent := int(downloaded * 100 / total)
					if percent != lastPercent && percent%5 == 0 {
						lastPercent = percent
						fmt.Fprintf(a.IO.Out, "\r  Progression: %d%% (%d/%d MB)",
							percent, downloaded/1024/1024, total/1024/1024)
					}
				}
			})
			if err != nil {
				return fmt.Errorf("échec du téléchargement: %w", err)
			}

			fmt.Fprintln(a.IO.Out) // newline after progress
			fmt.Fprintf(a.IO.Out, "%s opencode v%s installé: %s\n",
				common.SuccessStyle.Render(common.IconSuccess), targetVersion, binPath)

			return nil
		},
	}
}
