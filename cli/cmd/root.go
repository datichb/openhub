package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/filecrypt"
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
		// Configure structured logging
		verbose, _ := cmd.Flags().GetBool("verbose")
		logLevel := slog.LevelWarn
		if verbose {
			logLevel = slog.LevelDebug
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))

		// Skip heavy init for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == "completion" {
			return nil
		}
		if err := initApp(); err != nil {
			return err
		}
		// Localize Cobra command descriptions after locale is loaded
		localizeCommands(cmd.Root())
		return nil
	},
}

// Execute runs the root command.
func Execute() error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\noh: internal error (panic): %v\n", r)
			fmt.Fprintf(os.Stderr, "Please report this bug at https://github.com/datichb/openhub/issues\n")
			os.Exit(2)
		}
	}()
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
	a.WithSecretStore(resolveSecretStore())

	application = a
	return nil
}

// resolveSecretStore determines the best available secret storage:
// 1. OS keychain (go-keyring) — preferred
// 2. Encrypted file fallback (AES-256-GCM) — if keychain unavailable AND passphrase source exists
// 3. nil — if neither is available (callers are nil-safe)
func resolveSecretStore() domain.SecretStore {
	// Try OS keychain first
	if err := keychain.Probe(); err == nil {
		return keychain.New()
	}

	// Keychain unavailable — check if filecrypt fallback is viable
	if !filecrypt.IsAvailable() {
		slog.Warn("secrets unavailable", "reason", "no keychain, no terminal, OH_PASSPHRASE not set")
		return nil
	}

	slog.Warn("OS keychain unavailable, using encrypted file fallback")
	secretsPath := filepath.Join(config.HubDir(), "secrets.enc")
	return filecrypt.New(secretsPath, terminalPassphrasePrompt)
}

// terminalPassphrasePrompt provides the interactive passphrase prompt using huh.
func terminalPassphrasePrompt(creating bool) (string, error) {
	if creating {
		return promptCreatePassphrase()
	}
	return promptUnlockPassphrase()
}

// promptCreatePassphrase asks the user to create and confirm a new passphrase.
func promptCreatePassphrase() (string, error) {
	var passphrase, confirm string

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(i18n.T("secrets.fallback.prompt_create")).
				EchoMode(huh.EchoModePassword).
				Value(&passphrase),
			huh.NewInput().
				Title(i18n.T("secrets.fallback.prompt_confirm")).
				EchoMode(huh.EchoModePassword).
				Value(&confirm),
		),
	).Run()
	if err != nil {
		return "", err
	}

	if passphrase != confirm {
		return "", errors.New(i18n.T("secrets.fallback.mismatch"))
	}
	if len(passphrase) < 8 {
		return "", errors.New(i18n.T("secrets.fallback.too_short"))
	}
	return passphrase, nil
}

// promptUnlockPassphrase asks the user for their existing passphrase.
func promptUnlockPassphrase() (string, error) {
	var passphrase string

	err := huh.NewInput().
		Title(i18n.T("secrets.fallback.prompt_unlock")).
		EchoMode(huh.EchoModePassword).
		Value(&passphrase).
		Run()
	if err != nil {
		return "", err
	}
	return passphrase, nil
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output (debug logging)")
}

// localizeCommands recursively traverses the command tree and replaces
// Short/Long descriptions and flag usages with i18n translations if available.
// The key convention is: "cmd.<command-path>.short" and "cmd.<command-path>.long"
// For flags: "cmd.<command-path>.flags.<flag-name>"
// For example: "cmd.start.short", "cmd.project.list.short", "cmd.start.flags.project"
func localizeCommands(cmd *cobra.Command) {
	key := cmdI18nKey(cmd)
	if key != "" {
		if t := i18n.T(key + ".short"); t != key+".short" {
			cmd.Short = t
		}
		if t := i18n.T(key + ".long"); t != key+".long" {
			cmd.Long = t
		}
		// Localize flag descriptions
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			flagKey := key + ".flags." + f.Name
			if t := i18n.T(flagKey); t != flagKey {
				f.Usage = t
			}
		})
	}
	for _, sub := range cmd.Commands() {
		localizeCommands(sub)
	}
}

// cmdI18nKey builds the i18n key prefix for a cobra command.
// "oh" → "cmd.root"
// "oh start" → "cmd.start"
// "oh project list" → "cmd.project.list"
// "oh config set" → "cmd.config.set"
func cmdI18nKey(cmd *cobra.Command) string {
	if cmd.Parent() == nil {
		return "cmd.root"
	}
	parts := []string{}
	for c := cmd; c != nil && c.Parent() != nil; c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}
	return "cmd." + strings.Join(parts, ".")
}

// completeProjectIDs provides shell completion for the --project flag.
func completeProjectIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	a := TryApp()
	if a == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	projects, _ := a.Projects.List(cmd.Context(), "")
	var names []string
	for _, p := range projects {
		names = append(names, p.ID)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
