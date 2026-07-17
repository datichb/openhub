// Package views defines the View interface and full-screen views
// for the unified TUI shell.
package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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

	// StatusHints returns contextual keybinding hints for the status bar.
	StatusHints() string

	// HandleKey processes view-specific key events.
	// Return nil to consume the event, or return it to propagate to the shell.
	HandleKey(event *tcell.EventKey) *tcell.EventKey
}
