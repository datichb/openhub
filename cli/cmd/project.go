package cmd

import (
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:     "project",
	Aliases: []string{"p"},
	Short:   "Gestion des projets enregistrés",
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd())
	projectCmd.AddCommand(projectAddCmd())
	projectCmd.AddCommand(projectRemoveCmd())
}
