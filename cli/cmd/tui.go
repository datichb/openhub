package cmd

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// tuiShell holds a reference to the running shell for action callbacks.
var tuiShell *shell.Shell

// runTUI launches the unified TUI shell.
// If projectName is non-empty, the TUI starts directly in project mode.
func runTUI() error {
	return runTUIWithProject("")
}

// runTUIWithProject launches the TUI, optionally activating project mode
// immediately for the given project name.
func runTUIWithProject(projectName string) error {
	a := MustApp()

	// ── First-run detection: launch setup wizard if no provider configured ──
	if needsFirstRunWizard(a) && projectName == "" {
		completed := runFirstRunWizard(a)
		if !completed {
			return nil // user aborted wizard
		}
		// Reload config after wizard changes
		var err error
		a, err = ReloadApp()
		if err != nil {
			return fmt.Errorf("reload config after wizard: %w", err)
		}
	}

	homeViewID := "home"

	// Resolve initial project if requested
	var initialProject *views.ActiveProject
	if projectName != "" && a.Projects != nil {
		p, err := a.Projects.GetByName(context.Background(), projectName)
		if err != nil || p == nil {
			return fmt.Errorf("projet introuvable: %q", projectName)
		}
		initialProject = &views.ActiveProject{
			ID:   p.ID,
			Name: p.Name,
			Path: p.Path,
		}
		homeViewID = "project.mode"
	}

	// Create the notification store upfront so it can be shared between the
	// shell (which populates it via showToast) and the NotificationsView.
	notifStore := shell.NewNotificationStore(200)

	builtViews := buildViews(a, notifStore)

	cfg := shell.Config{
		ProjectName:   a.Config.Name,
		Commands:      buildCommands(a),
		Views:         builtViews,
		HomeViewID:    homeViewID,
		Notifications: notifStore,
	}

	tuiShell = shell.New(cfg)

	// ── Install TUI-aware slog handler ──────────────────────────────────
	// Redirect slog output to the TUI toast/notification system instead of
	// writing to stderr (which corrupts the tview terminal).
	originalSlogHandler := slog.Default().Handler()
	originalLogOutput := log.Writer()
	tuiHandler := shell.NewTUILogHandler(tuiShell, notifStore, slog.LevelWarn)
	slog.SetDefault(slog.New(tuiHandler))
	log.SetOutput(io.Discard) // suppress stdlib log writes (e.g. toast.go legacy)

	// Pre-set the active project before navigating home so project.mode renders it
	if initialProject != nil {
		tuiShell.SetActiveProject(initialProject)
	}

	tuiShell.NavigateHome(cfg.HomeViewID)
	err := tuiShell.Run()

	// ── Restore original logging ────────────────────────────────────────
	slog.SetDefault(slog.New(originalSlogHandler))
	if w, ok := originalLogOutput.(io.Writer); ok {
		log.SetOutput(w)
	} else {
		log.SetOutput(os.Stderr)
	}

	return err
}
