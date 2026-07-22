package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// resolveProject finds the project to use. Priority:
// 1. --project flag (explicit ID or name)
// 2. Current directory detection
// 3. Interactive selection if multiple projects exist
func resolveProject(ctx context.Context, a *app.App, projectID string) (*domain.Project, error) {
	if projectID != "" {
		p, err := a.Projects.GetByName(ctx, projectID)
		if err == nil {
			return p, nil
		}
		p, err = a.Projects.Get(ctx, projectID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, fmt.Errorf("projet %q introuvable", projectID)
			}
			return nil, err
		}
		return p, nil
	}

	cwd, _ := os.Getwd()
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("cmd.project.no_projects"))
	}

	for i, p := range projects {
		absPath, _ := filepath.Abs(p.Path)
		if absPath == cwd || isSubPath(cwd, absPath) {
			return &projects[i], nil
		}
	}

	if len(projects) == 1 {
		return &projects[0], nil
	}

	var selectedID string
	options := make([]huh.Option[string], len(projects))
	for i, p := range projects {
		label := fmt.Sprintf("%s (%s)", p.Name, p.Language)
		options[i] = huh.NewOption(label, p.ID)
	}

	form := theme.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choisir un projet").
				Options(options...).
				Value(&selectedID),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}

	for i, p := range projects {
		if p.ID == selectedID {
			return &projects[i], nil
		}
	}
	return nil, fmt.Errorf("%s", i18n.T("cmd.quick.not_found"))
}

// ensureOpencode checks that the opencode binary is available.
// If not found, prompts the user to install it.
func ensureOpencode(a *app.App) error {
	_, err := opencode.FindBinary()
	if err == nil {
		return nil
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n\n",
		theme.WarningStyle.Render(theme.IconWarning), i18n.T("cmd.start.opencode_not_found"))

	var choice string
	form := theme.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(i18n.T("cmd.start.install_choice")).
				Options(
					huh.NewOption(i18n.T("cmd.start.install_brew"), "brew"),
					huh.NewOption(i18n.T("cmd.start.install_download"), "download"),
					huh.NewOption(i18n.T("cmd.start.install_cancel"), "cancel"),
				).
				Value(&choice),
		),
	)
	if err := form.Run(); err != nil {
		return fmt.Errorf("selection cancelled")
	}

	switch choice {
	case "brew":
		fmt.Fprintf(a.IO.Out, "\n  %s\n\n",
			i18n.Tf("cmd.start.install_run_brew", theme.Bold.Render("brew install anomalyco/tap/opencode")))
		return fmt.Errorf("%s", i18n.T("cmd.start.install_required"))
	case "download":
		return downloadOpencode(a)
	default:
		return fmt.Errorf("%s", i18n.T("cmd.start.install_required_generic"))
	}
}

// downloadOpencode downloads and installs the opencode binary.
func downloadOpencode(a *app.App) error {
	installDir := a.Config.Opencode.InstallDir
	version := a.Config.Opencode.Version
	if version == "" {
		version = "latest"
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.downloading"))

	var lastPercent int
	_, err := opencode.Download(version, installDir, func(downloaded, total int64) {
		if total > 0 {
			percent := int(downloaded * 100 / total)
			if percent != lastPercent && percent%5 == 0 {
				lastPercent = percent
				fmt.Fprintf(a.IO.Out, "\r  %s",
					i18n.Tf("cmd.start.download_progress", percent, downloaded/1024/1024, total/1024/1024))
			}
		}
	})
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s %s\n\n",
		theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.start.installed"))
	return nil
}

func displayOrDefault(detected, fallback string) string {
	if detected != "" {
		return detected
	}
	if fallback != "" {
		return fallback
	}
	return "—"
}
