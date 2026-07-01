package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

var (
	// application is the shared App instance, initialized in PersistentPreRunE.
	application *app.App

	// storeClosed tracks whether we've already cleaned up.
	store *sqlite.Store
)

var rootCmd = &cobra.Command{
	Use:   "oh",
	Short: "OpenHub CLI — orchestrateur pour opencode",
	Long: `oh est le CLI monolithique d'OpenHub.
Il orchestre les sessions opencode, gère les projets, déploie les agents/skills/MCP,
et fournit un TUI interactif pour le suivi de développement.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip heavy init for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" {
			return nil
		}
		return initApp()
	},
}

// Execute runs the root command.
func Execute() error {
	defer func() {
		if store != nil {
			store.Close()
		}
	}()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

// App returns the application instance. Panics if called before initialization.
func App() *app.App {
	if application == nil {
		panic("App() called before initialization")
	}
	return application
}

// initApp wires up all dependencies.
func initApp() error {
	a, err := app.New()
	if err != nil {
		return err
	}

	// Open SQLite store
	s, err := sqlite.OpenDefault()
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	store = s

	// Wire stores
	a.WithProjectStore(sqlite.NewProjectStore(s))
	a.WithSessionStore(sqlite.NewSessionStore(s))
	a.WithSecretStore(keychain.New())

	application = a
	return nil
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}
