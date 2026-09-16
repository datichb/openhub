package launcher

import (
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/prompt"
)

// LaunchOpts configures a single session launch.
// The caller resolves ProjectID, ProjectPath, Agent, and Prompt before calling Launch.
type LaunchOpts struct {
	// ── Project (pre-resolved by caller) ──
	ProjectID   string
	ProjectPath string

	// ── Session ──
	Agent     string
	Prompt    string
	ExtraArgs []string

	// ── Provider override (empty = cascade from project → hub → default) ──
	Provider string

	// ── Behaviour ──
	SkipSummary bool // do not display the pre-launch summary
	SkipConfirm bool // do not ask for confirmation
	SkipDeploy  bool // do not run auto-deploy

	// ── Callbacks (optional, set by the caller) ──

	// DeployFunc runs the auto-deploy check. Called with the app and resolved provider.
	// The caller is responsible for providing this because deploy depends on heavy
	// cmd-level state (hubDir, buildDeployPlan) that the launcher should not own.
	DeployFunc func(a *app.App, provider string)

	// SummaryFunc displays the pre-launch summary. Same rationale as DeployFunc:
	// the summary format is owned by cmd/, not by the launcher.
	SummaryFunc func(provider string, stack prompt.StackInfo, bearerToken string)
}

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
// The launcher calls these methods at specific points in the pipeline.
// Two implementations exist: CLILaunchUI (for terminal) and TUILaunchUI (for the dashboard).
type LaunchUI interface {
	// Confirm asks the user to confirm the launch. Returns (true, nil) to proceed.
	Confirm(title string) (bool, error)

	// Notify displays a message to the user (info, warning, error, success).
	Notify(msg string, level Level)

	// SuspendAndExec returns a function that suspends the UI and runs the given
	// function (which takes over the terminal). Returns nil for CLI mode
	// (where no suspend is needed — the caller runs opencode.Run directly).
	SuspendAndExec() func(func() error) error
}
