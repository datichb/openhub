// Package views defines the View interface and full-screen views
// for the unified TUI shell.
package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TeamResolution holds the effective team configuration for the currently active
// project, already resolved (project override → hub fallback). Views use this
// to get per-project team data without depending on the config or cmd packages.
//
// This mirrors config.ResolvedTeamConfig but lives in the views package to avoid
// import cycles (views ← cmd ← config; views must not import config or cmd).
type TeamResolution struct {
	// Enabled reports whether team features are active for this project.
	Enabled bool
	// StateRepo is the Git remote URL of the team-state repository.
	StateRepo string
	// StatePath is the local filesystem path of the team-state clone.
	StatePath string
	// MemberID is the current user's member identifier in the team-state.
	MemberID string
}

// ResolveTeamFunc is a callback that views call to obtain the effective team
// configuration for the currently active project. The implementation lives in
// cmd/tui.go where both app.App and the active project are available.
//
// The function must be cheap to call (cached in the wiring layer if necessary).
type ResolveTeamFunc func() TeamResolution

// View defines the contract for a navigable view in the TUI shell.
// All views must be safe to Mount/Unmount multiple times throughout
// the shell lifecycle.
type View interface {
	// ID returns the unique identifier for this view (e.g., "home", "board").
	ID() string

	// Title returns the localized display title for breadcrumb and menu.
	Title() string

	// Mount populates the shell's content panel with this view's widgets.
	// Called when the view becomes active via the router.
	Mount(content *tview.Flex, app *tview.Application)

	// Unmount cleans up resources (stop timers, close channels).
	// Called when navigating away from this view.
	Unmount()

	// StatusHints returns contextual keybinding hints for the omnibar.
	StatusHints() string

	// HandleKey processes view-specific key events.
	// Return nil to consume the event, or return it to propagate to the shell.
	HandleKey(event *tcell.EventKey) *tcell.EventKey
}

// ContextCommand represents a command that a view injects into the omnibar
// when that view is active. This allows views to provide domain-specific
// commands without polluting the global command registry.
type ContextCommand struct {
	// ID is the unique identifier (e.g., "toggle.auto_update", "language.fr").
	ID string
	// Label is the display text in omnibar suggestions.
	Label string
	// Aliases are additional fuzzy-match terms.
	Aliases []string
	// Description is a one-line help text.
	Description string
	// Category groups commands in the suggestion list (e.g., "Config").
	Category string
	// Action is called when the command is executed.
	Action func()
	// RunsDirect marks actions that MUST execute synchronously on the tview event
	// loop and MUST NOT be deferred via QueueUpdateDraw. Set to true when the
	// action calls SuspendAndExec — see shell.Command.RunsDirect for the full
	// explanation of why this is necessary.
	RunsDirect bool
}

// CommandProvider is an optional interface that views can implement to supply
// contextual commands to the omnibar when the view is active.
// Contextual commands appear with higher priority than global commands.
type CommandProvider interface {
	// ContextCommands returns commands available in the current view state.
	// Called on each omnibar keystroke — implementations should cache results.
	ContextCommands() []ContextCommand
}
