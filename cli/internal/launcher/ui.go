// Package launcher holds the UI abstraction used while a session starts
// (CLI or TUI): confirmations, notifications, terminal suspension. Sessions
// start through the RunService (`oh run`, cmd/v5_run.go); the former launch
// pipeline was removed with opencode V1 (v5, P3-T30).
package launcher

// Level represents a notification severity level.
type Level int

const (
	LevelInfo    Level = iota
	LevelSuccess Level = iota
	LevelWarning Level = iota
	LevelError   Level = iota
)

// LaunchUI abstracts the UI layer (CLI vs TUI).
//
// The launch calls these methods at specific points.
// Two implementations exist: CLILaunchUI (for terminal) and TUILaunchUI (for the dashboard).
type LaunchUI interface {
	// Confirm asks the user to confirm the launch. Returns (true, nil) to proceed.
	Confirm(title string) (bool, error)

	// Notify displays a message to the user (info, warning, error, success).
	Notify(msg string, level Level)

	// SuspendAndExec returns a function that suspends the UI and runs the given
	// function (which takes over the terminal). Returns nil for CLI mode
	// (where no suspend is needed — the caller runs the client directly).
	SuspendAndExec() func(func() error) error
}
