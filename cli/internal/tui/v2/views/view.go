// Package views defines the View interface and full-screen views
// for the unified TUI shell.
package views

import (
	"context"

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

// SelectOption represents a selectable option with a friendly label and a stored value.
type SelectOption struct {
	Label string // Friendly display (e.g. "Français")
	Value string // Stored value (e.g. "fr")
}

// ModalAction represents a button action in a scrollable modal.
type ModalAction struct {
	Label    string
	Callback func()
}

// ShellAccess provides access to shell overlay capabilities from views.
type ShellAccess interface {
	// Context returns the shell lifecycle context, cancelled on app exit.
	// Use this in goroutines to enable cancellation on quit/SIGINT.
	Context() context.Context
	ShowInputModal(title, currentValue string, onConfirm func(newValue string))
	ShowPasswordModal(title string, onConfirm func(value string))
	ShowSelectModal(title string, options []SelectOption, currentValue string, onConfirm func(value string))
	ShowMultiSelectModal(title string, options []SelectOption, selected []string, onConfirm func(selected []string))
	ShowScrollableModal(title, content string, actions []ModalAction)
	ShowToastMsg(msg string, success bool)
	ShowInlineForm(cfg InlineFormConfig)
	// NavigateTo navigates to a registered view by ID.
	NavigateTo(viewID string)
	// SetProjectMode activates or deactivates project mode with the given project.
	// Passing nil deactivates project mode (returns to hub mode).
	// Deprecated: use SetMode(ModeProject) + SetActiveProject instead.
	SetProjectMode(project *ActiveProject)
	// SetActiveProject sets the active project without triggering navigation.
	SetActiveProject(project *ActiveProject)
	// ActiveProject returns the currently active project, or nil in hub mode.
	ActiveProject() *ActiveProject

	// ── Mode navigation (ADR-032 Phase 3) ───────────────────────────────
	// SetMode switches the shell to the given mode and navigates to its landing view.
	SetMode(mode Mode)
	// Mode returns the current navigation mode.
	Mode() Mode
	// SetActiveTeam sets the active team context for team mode.
	SetActiveTeam(team *ActiveTeam)
	// ActiveTeam returns the currently active team, or nil outside team mode.
	ActiveTeam() *ActiveTeam
}

// ActiveProject holds the minimal project context for the TUI project mode.
type ActiveProject struct {
	ID   string
	Name string
	Path string
}

// ActiveTeam holds the minimal team context for the TUI team mode (ADR-032 Phase 3).
type ActiveTeam struct {
	ID   string
	Name string
}

// ─────────────────────────────────────────────────────────────────────────────
// Navigation modes (ADR-032 Phase 3)
// ─────────────────────────────────────────────────────────────────────────────

// Mode represents the navigation context of the TUI shell.
// Each mode filters the omnibar commands and determines the landing view.
type Mode string

const (
	// ModeHub is the default multi-scope mode — shows teams, projects, and global commands.
	ModeHub Mode = "hub"
	// ModeTeam focuses the TUI on a single team — shows team board, status, activity.
	ModeTeam Mode = "team"
	// ModeProject focuses the TUI on a single project — shows board, sessions, config.
	ModeProject Mode = "project"
)

// ModeAware is an optional interface that views can implement to declare
// in which modes they are relevant. Views that do not implement ModeAware
// are considered global (visible in all modes).
type ModeAware interface {
	SupportedModes() []Mode
}

// ProjectInfo is a minimal project representation for the tracker mapping UI.
type ProjectInfo struct {
	ID   string
	Name string
}

// SyncTrackerResult holds the outcome of a tracker sync for display in a modal.
type SyncTrackerResult struct {
	ClaimsCreated int
	ClaimsUpdated int
	LabelsPushed  int
	Projects      []string // per-project summary lines
	Warnings      []string
	Errors        []string
}
