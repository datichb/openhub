package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/i18n"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Affiche la version de oh",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "oh %s\n", buildinfo.Version)
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", i18n.Tf("cmd.version.commit", buildinfo.Commit))
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", i18n.Tf("cmd.version.built", buildinfo.BuildDate))
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", i18n.Tf("cmd.version.go", runtime.Version()))
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", i18n.Tf("cmd.version.os_arch", runtime.GOOS, runtime.GOARCH))
	},
}

func init() {
	rootCmd.AddCommand(withInitLevel(versionCmd, levelNone))
}
