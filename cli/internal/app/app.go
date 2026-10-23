// Package app provides the application-level dependency container.
// It wires together all services and is passed to command constructors.
package app

import (
	"fmt"
	"io"
	"os"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/platform"
)

// App is the central dependency container injected into all commands.
// Commands depend on App (and its interfaces), never on concrete implementations.
type App struct {
	Config      *config.Config
	Projects    domain.ProjectStore
	Sessions    domain.SessionStore
	AgentEvents domain.AgentEventStore
	Secrets     domain.SecretStore
	// Preferences and WorkflowUsage back the PreferenceService (prefsvc).
	Preferences   domain.PreferenceStore
	WorkflowUsage domain.WorkflowUsageReader
	// The other stores of the database. On a degraded startup (database not
	// open, schema to migrate) they are nil, like Projects and Sessions:
	// a caller checks before it uses one.
	Decisions     domain.DecisionStore
	Usage         domain.UsageStore
	Servers       domain.ServerStore
	Grants        domain.GrantStore
	Remotes       domain.RemoteStore
	Checkpoints   domain.CheckpointStorage
	Reports       domain.ReportStore
	WikiProposals domain.WikiProposalStore
	History       domain.HistoryStore
	Fanout        domain.FanoutStore
	// ToolVersion returns the version of the session tool, or
	// why it cannot be used (missing, unsupported release).
	ToolVersion func() (string, error)
	Stats       platform.StatsProvider // Session metrics provider (ADR-036)
	IO          *IOStreams
}

// IOStreams abstracts standard I/O for testability.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// DefaultIOStreams returns the standard OS streams.
func DefaultIOStreams() *IOStreams {
	return &IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
}

// New creates a fully-wired App instance.
// It loads configuration, initializes stores, and resolves the locale.
func New() (*App, error) {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	// Set locale from config
	i18n.SetLocale(cfg.CLI.Language)
	config.DeliverNotices() // the warnings read in hub.toml, in that language

	// Stores are initialized lazily by the caller (cmd/root.go) because
	// some commands (e.g., version, help) don't need database access.
	return &App{
		Config: cfg,
		IO:     DefaultIOStreams(),
	}, nil
}

// WithProjectStore sets the project store (used during bootstrap or in tests).
func (a *App) WithProjectStore(s domain.ProjectStore) *App {
	a.Projects = s
	return a
}

// WithSessionStore sets the session store.
func (a *App) WithSessionStore(s domain.SessionStore) *App {
	a.Sessions = s
	return a
}

// WithAgentEventStore sets the agent event store.
func (a *App) WithAgentEventStore(s domain.AgentEventStore) *App {
	a.AgentEvents = s
	return a
}

// WithSecretStore sets the secret store.
func (a *App) WithSecretStore(s domain.SecretStore) *App {
	a.Secrets = s
	return a
}

// WithPreferences sets the preference store and the workflow usage reader.
func (a *App) WithPreferences(s domain.PreferenceStore, usage domain.WorkflowUsageReader) *App {
	a.Preferences = s
	a.WorkflowUsage = usage
	return a
}

// WithDecisions sets the store of the pending decisions.
func (a *App) WithDecisions(s domain.DecisionStore) *App {
	a.Decisions = s
	return a
}

// WithUsage sets the usage ledger.
func (a *App) WithUsage(s domain.UsageStore) *App {
	a.Usage = s
	return a
}

// WithServers sets the stores of the tool servers and of their proxy grants.
func (a *App) WithServers(servers domain.ServerStore, grants domain.GrantStore) *App {
	a.Servers = servers
	a.Grants = grants
	return a
}

// WithRemotes sets the store of the remote references.
func (a *App) WithRemotes(s domain.RemoteStore) *App {
	a.Remotes = s
	return a
}

// WithCheckpoints sets the checkpoint store (state and session outputs).
func (a *App) WithCheckpoints(s domain.CheckpointStorage) *App {
	a.Checkpoints = s
	return a
}

// WithReports sets the report store.
func (a *App) WithReports(s domain.ReportStore) *App {
	a.Reports = s
	return a
}

// WithWikiProposals sets the store of the wiki proposals.
func (a *App) WithWikiProposals(s domain.WikiProposalStore) *App {
	a.WikiProposals = s
	return a
}

// WithHistory sets the store that counts and deletes the history past its retention.
func (a *App) WithHistory(s domain.HistoryStore) *App {
	a.History = s
	return a
}

// WithFanout sets the store of the fan-out (sessions and fan-out record).
func (a *App) WithFanout(s domain.FanoutStore) *App {
	a.Fanout = s
	return a
}

// WithIO overrides the IO streams (useful for testing).
func (a *App) WithIO(ioStreams *IOStreams) *App {
	a.IO = ioStreams
	return a
}

// WithToolVersion sets the probe of the session tool version.
func (a *App) WithToolVersion(f func() (string, error)) *App {
	a.ToolVersion = f
	return a
}

// WithStats sets the session metrics provider (ADR-036).
func (a *App) WithStats(s platform.StatsProvider) *App {
	a.Stats = s
	return a
}
