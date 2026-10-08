package cmd

import (
	"context"
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
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/prettylog"
	"github.com/datichb/openhub/cli/internal/sessionstats"
	"github.com/datichb/openhub/cli/internal/storage/filecrypt"
	"github.com/datichb/openhub/cli/internal/storage/keychain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var (
	// application is the shared App instance, initialized in PersistentPreRunE.
	application *app.App

	// store holds the SQLite connection for cleanup.
	store *sqlite.Store
)

var rootCmd = &cobra.Command{
	Use:           "oh",
	Short:         i18n.T("cmd.root.short"),
	Long:          i18n.T("cmd.root.long"),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if canLaunchTUI() {
			projectName, _ := cmd.Flags().GetString("project")
			return runTUIWithProject(projectName)
		}
		return cmd.Help()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Configure structured logging
		verbose, _ := cmd.Flags().GetBool("verbose")
		logLevel := slog.LevelWarn
		if verbose {
			logLevel = slog.LevelDebug
		}
		logFormat, _ := cmd.Flags().GetString("log-format")
		switch logFormat {
		case "json":
			slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))
		default:
			slog.SetDefault(slog.New(prettylog.NewPrettyHandler(os.Stderr, &prettylog.Options{Level: logLevel})))
		}

		// Propagate --no-tui to the TUI detection layer
		if noTUI, _ := cmd.Flags().GetBool("no-tui"); noTUI {
			common.SetNoTUI(true)
		}

		// Skip heavy init for commands that don't need it
		if cmd.Name() == "version" || cmd.Name() == "help" || cmd.Name() == "completion" || isRunnerInstall(cmd) {
			return nil
		}

		// Auto-bootstrap hub content if needed (extract embedded agents/skills/permissions).
		// This replaces the former hard gate that required `oh init` before any command.
		// The TUI's inline first-run wizard handles the rest (provider, credentials, project).
		if cmd.Name() != "init" && cmd.Name() != "doctor" && cmd.Name() != "purge" {
			if hubcontent.NeedsExtract() {
				if err := hubcontent.Extract(hubcontent.HubContentDir()); err != nil {
					slog.Warn("failed to extract hub content", "error", err)
				}
			}
		}

		if err := initApp(); err != nil {
			return err
		}
		// Former workflow configurations → team-state (v5 phase 2, v38).
		if cmd.Name() != "init" && cmd.Name() != "purge" {
			migrateLegacyWorkflowsAtStartup(cmd.Context(), os.Stderr)
		}
		// Former deployments: offered once (the TUI shows its own screen).
		if offerCleanupFor(cmd) {
			offerDeployCleanupAtStartup(cmd.Context(), os.Stderr, cmd.Name())
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
	// Before the store closes: the in-process daemon (Windows) puts its
	// sessions to sleep.
	defer stopInProcessDaemon()

	if err := rootCmd.Execute(); err != nil {
		var exit *ExitError
		if !errors.As(err, &exit) {
			ensureLocale()
			fmt.Fprintln(os.Stderr, localizeError(err))
		}
		return err
	}
	return nil
}

// ExitError ends oh with Code without printing anything more (the command
// already reported the failure, or a proxied command printed its own).
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// ExitCode returns the process exit code for an error returned by Execute.
func ExitCode(err error) int {
	var exit *ExitError
	if errors.As(err, &exit) && exit.Code > 0 {
		return exit.Code
	}
	if err != nil {
		return 1
	}
	return 0
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

// ReloadApp re-initializes the application from disk configuration.
// Call this after the init wizard writes config to disk to pick up changes.
func ReloadApp() (*app.App, error) {
	if err := initApp(); err != nil {
		return nil, err
	}
	return application, nil
}

// TryApp returns the application instance or nil if initialization failed.
// Use only in commands that explicitly handle the nil case (e.g., doctor checks).
func TryApp() *app.App {
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

	// Auto-migrate legacy [team] → [[teams]] (ADR-029).
	// Creates a backup of hub.toml before writing if migration occurs.
	if migrated, backup, mErr := config.RunMigrationIfNeeded(a.Config); mErr != nil {
		slog.Warn("team config migration failed", "error", mErr)
	} else if migrated {
		slog.Info("migrated [team] to [[teams]]", "backup", backup)
	}

	// Auto-migrate legacy token key names (gitlab-token → openhub.mcp.gitlab.token).
	if config.MigrateTokenKeys(a.Config) {
		if err := config.Save(a.Config); err != nil {
			slog.Warn("token key migration save failed", "error", err)
		} else {
			slog.Info("migrated token keys to openhub.mcp.* convention")
		}
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
	a.WithAgentEventStore(sqlite.NewAgentEventStore(s))
	prefs := sqlite.NewPreferenceStore(s)
	a.WithPreferences(prefs, prefs)
	a.WithSecretStore(resolveSecretStore())

	// Session tool (supported release only) and statistics from the oh registry.
	a.WithToolVersion(func() (string, error) {
		if err := requireV2(context.Background()); err != nil {
			return "", err
		}
		return v5Tool.Version, nil
	})
	a.WithStats(sessionstats.New(a.Sessions, a.Projects))

	// Auto-migrate legacy ProjectTeamConfig → TeamID (ADR-029).
	if activeTeam := a.Config.ActiveTeam(); activeTeam.ID != "" {
		if n, mErr := config.MigrateProjectTeamToTeamID(s.DB(), activeTeam.ID); mErr != nil {
			slog.Warn("project team_id migration failed", "error", mErr)
		} else if n > 0 {
			slog.Info("migrated project team_config to team_id", "count", n)
		}
	}

	// Auto-migrate legacy keychain entries (gitlab-token → openhub.mcp.gitlab.token, etc.)
	if n := config.MigrateKeychainKeys(a.Secrets); n > 0 {
		slog.Info("migrated keychain keys to unified convention", "count", n)
	}

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
		return keychain.New(config.HubDir())
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

	err := theme.NewForm(
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
	// Enable hook traversal so that child PersistentPreRunE (e.g. teamCmd)
	// no longer shadow the root's — Cobra walks up the tree and executes
	// each ancestor's hook in order. Available since Cobra v1.6.0.
	cobra.EnableTraverseRunHooks = true

	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output (debug logging)")
	rootCmd.PersistentFlags().String("log-format", "pretty", "Log output format: pretty (default) or json")
	rootCmd.PersistentFlags().Bool("no-tui", false, "Disable rich TUI (use inline prompts only)")
	rootCmd.Flags().StringP("project", "p", "", "Démarrer directement en mode projet pour ce projet")
	_ = rootCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
	// The paged overview is the help of oh itself; subcommands keep their
	// own help (flags, examples).
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		localizeHelp(c.Root())
		if c == rootCmd {
			customHelpFunc(c, args)
			return
		}
		defaultHelp(c, args)
	})
	rootCmd.AddCommand(secretsCmd)
}

// localizeCommands translates the help of the command tree into the
// current locale: Short, Long and flag usages. A text comes from its
// conventional key ("cmd.<command-path>.short", ".long",
// ".flags.<flag-name>", e.g. "cmd.project.list.short"), else from the key of
// the message it holds (texts built with i18n.T before the locale was
// known). Every help has one or the other (TestEveryHelpIsTranslated).
func localizeCommands(cmd *cobra.Command) {
	key := cmdI18nKey(cmd)
	cmd.Short = localizedHelp(key+".short", cmd.Short)
	cmd.Long = localizedHelp(key+".long", cmd.Long)
	localize := func(f *pflag.Flag) {
		if f.Name != "help" {
			f.Usage = localizedHelp(key+".flags."+f.Name, f.Usage)
		}
	}
	cmd.Flags().VisitAll(localize)
	cmd.PersistentFlags().VisitAll(localize) // merged into Flags() only once parsed
	localizeHelpFlag(cmd)
	for _, sub := range cmd.Commands() {
		localizeCommands(sub)
	}
}

// localizedHelp returns the translation of key, else of the message text
// holds, else text.
func localizedHelp(key, text string) string {
	if t := i18n.T(key); t != key {
		return t
	}
	if k := i18n.KeyOf(text); k != "" {
		return i18n.T(k)
	}
	return text
}

// localizeHelp sets the locale of the hub configuration (help is shown
// without initializing the app) and translates the command tree.
func localizeHelp(root *cobra.Command) {
	if application == nil {
		if c, err := config.Load(); err == nil && c != nil && c.CLI.Language != "" {
			i18n.SetLocale(c.CLI.Language)
		}
	}
	localizeCommands(root)
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
		names = append(names, p.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
