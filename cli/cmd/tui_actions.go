package cmd

import (
	"context"
	"fmt"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
)

// runDeployForProject deploys hub content to a project in-process.
func runDeployForProject(a *app.App, project *domain.Project) error {
	hubDir := findHubDir()
	if hubDir == "" {
		return fmt.Errorf("hub content not found")
	}

	plan := buildDeployPlan(a, project.Path, project.ID, hubDir, project.Provider, "", project.Agents, project.ModelOverrides, project.MCPConfig)

	_, err := deploy.Execute(plan)
	if err != nil {
		return fmt.Errorf("deploy %s: %w", project.Name, err)
	}
	return nil
}

// runSyncAll synchronizes hub content to all registered projects.
func runSyncAll(a *app.App) error {
	ctx := context.Background()
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		return fmt.Errorf("listing projects: %w", err)
	}

	var lastErr error
	for i := range projects {
		if err := runDeployForProject(a, &projects[i]); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// runUpgradeOpencode updates the opencode binary to the latest version.
func runUpgradeOpencode() error {
	installDir := config.HubDir() + "/bin"
	_, err := opencode.Download("latest", installDir, nil)
	return err
}
