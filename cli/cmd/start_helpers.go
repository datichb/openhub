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
	"github.com/datichb/openhub/cli/internal/provider"
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

func displayOrDefault(detected, fallback string) string {
	if detected != "" {
		return detected
	}
	if fallback != "" {
		return fallback
	}
	return "—"
}

// hubProviderCfg returns the hub-level provider.Config for the given provider name.
// Used as the fallback layer in the provider config cascade.
func hubProviderCfg(a *app.App, prov string) provider.Config {
	switch provider.Name(prov) {
	case provider.Bedrock:
		return provider.Config{
			AWSProfile: a.Config.Provider.Bedrock.AWSProfile,
			AWSRegion:  a.Config.Provider.Bedrock.AWSRegion,
		}
	default:
		return provider.Config{}
	}
}
