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

	// store holds the SQLite connection for cleanup.
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
		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == "completion" {
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

// MustApp returns the initialized application instance.
// It panics if the app has not been initialized — this indicates a programming
// error (command registered without PersistentPreRunE running first).
func MustApp() *app.App {
	if application == nil {
		fmt.Fprintln(os.Stderr, "oh: erreur interne — application non initialisée.")
		fmt.Fprintln(os.Stderr, "Vérifiez que la configuration (~/.oh/hub.toml) est accessible.")
		os.Exit(1)
	}
	return application
}

// TryApp returns the application instance or nil if initialization failed.
// Use only in commands that explicitly handle the nil case (e.g., doctor checks).
func TryApp() *app.App {
	return application
}

// GetApp returns the application instance. Returns nil before initialization.
// Deprecated: use MustApp() for commands that require an initialized app,
// or TryApp() for commands that handle the nil case.
func GetApp() *app.App {
	return application
}

// RootCmd returns the root cobra command (used by subpackages to register commands).
func RootCmd() *cobra.Command {
	return rootCmd
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
