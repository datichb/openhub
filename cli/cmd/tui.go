package cmd

import (
	"context"
	"fmt"

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
	notifStore := shell.NewNotificationStore(50)

	builtViews := buildViews(a, notifStore)

	cfg := shell.Config{
		ProjectName:   a.Config.Name,
		Commands:      buildCommands(a),
		Views:         builtViews,
		HomeViewID:    homeViewID,
		Notifications: notifStore,
	}

	tuiShell = shell.New(cfg)

	// Pre-set the active project before navigating home so project.mode renders it
	if initialProject != nil {
		tuiShell.SetActiveProject(initialProject)
	}

	tuiShell.NavigateHome(cfg.HomeViewID)
	return tuiShell.Run()
}
