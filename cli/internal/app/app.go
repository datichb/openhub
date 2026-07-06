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
)

// App is the central dependency container injected into all commands.
// Commands depend on App (and its interfaces), never on concrete implementations.
type App struct {
	Config   *config.Config
	Projects domain.ProjectStore
	Sessions domain.SessionStore
	Secrets  domain.SecretStore
	IO       *IOStreams
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

// WithSecretStore sets the secret store.
func (a *App) WithSecretStore(s domain.SecretStore) *App {
	a.Secrets = s
	return a
}

// WithIO overrides the IO streams (useful for testing).
func (a *App) WithIO(io *IOStreams) *App {
	a.IO = io
	return a
}
