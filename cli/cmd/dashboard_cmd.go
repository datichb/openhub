package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// dashboardCmd opens the TUI on the Metrics view of the hub, which shows what
// the dashboard showed.
var dashboardCmd = &cobra.Command{
	Use:  "dashboard",
	RunE: runDashboard,
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

func runDashboard(cmd *cobra.Command, args []string) error {
	if !canLaunchTUI() {
		return withExitCode(ExitUsage, errors.New(i18n.T("cmd.dashboard.needs_terminal")))
	}
	return runTUI(tuiLaunch{Hub: true, View: "metrics"})
}
