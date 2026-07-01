package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "oh",
	Short: "OpenHub CLI — orchestrateur pour opencode",
	Long: `oh est le CLI monolithique d'OpenHub.
Il orchestre les sessions opencode, gère les projets, déploie les agents/skills/MCP,
et fournit un TUI interactif pour le suivi de développement.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}
