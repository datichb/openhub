package cmd

import (
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// oh migrate: one-off migrations to oh v5 (P3-T28). Each subcommand lives in
// its own file (migrate_<name>.go).
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: i18n.T("cmd.migrate.cleanup.parent_short"),
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
