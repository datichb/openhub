package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// dashboardCmd is deprecated (removed in v6): it opens the TUI on the
// Metrics view of the hub, which shows what the dashboard showed.
var dashboardCmd = &cobra.Command{
	Use:        "dashboard",
	Deprecated: i18n.T("cmd.dashboard.deprecated"),
	RunE:       runDashboard,
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

func runDashboard(cmd *cobra.Command, args []string) error {
	warnDeprecatedCommand(cmd.ErrOrStderr(), "oh dashboard", "cmd.dashboard.deprecated")
	if !canLaunchTUI() {
		return withExitCode(ExitUsage, errors.New(i18n.T("cmd.dashboard.needs_terminal")))
	}
	return runTUI(tuiLaunch{Hub: true, View: "metrics", Notice: deprecatedCommandMessage("oh dashboard", "cmd.dashboard.deprecated")})
}
