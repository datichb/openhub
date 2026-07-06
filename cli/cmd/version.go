package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// Build-time variables injected via -ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Affiche la version de oh",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("oh %s\n", Version)
		fmt.Printf("  %s\n", i18n.Tf("cmd.version.commit", Commit))
		fmt.Printf("  %s\n", i18n.Tf("cmd.version.built", BuildDate))
		fmt.Printf("  %s\n", i18n.Tf("cmd.version.go", runtime.Version()))
		fmt.Printf("  %s\n", i18n.Tf("cmd.version.os_arch", runtime.GOOS, runtime.GOARCH))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
