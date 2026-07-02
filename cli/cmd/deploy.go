package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Déploie agents, skills et config dans un projet",
	Long: `Déploie les agents, skills, configuration provider, et serveurs MCP
dans le répertoire du projet cible. Opération transactionnelle avec rollback automatique en cas d'erreur.`,
	RunE: runDeploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringP("project", "j", "", "ID du projet (auto-detect sinon)")
	deployCmd.Flags().StringP("provider", "P", "", "Provider à configurer")
	deployCmd.Flags().StringP("model", "m", "", "Modèle à configurer")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	a := MustApp()

	// Resolve project
	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(a, projectID)
	if err != nil {
		return err
	}

	provider, _ := cmd.Flags().GetString("provider")
	model, _ := cmd.Flags().GetString("model")

	// Determine hub directory (where agents/ and skills/ live)
	hubDir := findHubDir()
	if hubDir == "" {
		return fmt.Errorf("impossible de trouver le répertoire hub (agents/, skills/)")
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s Déploiement vers %s\n",
		common.Title.Render("oh deploy"), common.Bold.Render(project.Name))
	fmt.Fprintf(a.IO.Out, "  Source: %s\n", hubDir)
	fmt.Fprintf(a.IO.Out, "  Cible:  %s\n", project.Path)
	fmt.Fprintln(a.IO.Out)

	// Build deployment plan
	plan := &deploy.Plan{
		ProjectPath: project.Path,
		ProjectID:   project.ID,
		HubDir:      hubDir,
		Provider:    provider,
		Model:       model,
		Phases: []deploy.Phase{
			deploy.DeployAgents(hubDir),
			deploy.DeploySkills(hubDir),
			deploy.DeployConfig(provider, model),
			deploy.DeployMCP(buildMCPServers(a), "oh"),
		},
	}

	// Execute
	start := time.Now()
	results, err := deploy.Execute(plan)

	// Display results
	for _, r := range results {
		icon := common.SuccessStyle.Render(common.IconSuccess)
		if !r.Success {
			icon = common.ErrorStyle.Render(common.IconError)
		}
		fmt.Fprintf(a.IO.Out, "  %s %s (%s)\n", icon, r.Name, r.Duration.Round(time.Millisecond))
		if !r.Success {
			fmt.Fprintf(a.IO.Out, "    %s\n", common.ErrorStyle.Render(r.Message))
		}
	}

	fmt.Fprintln(a.IO.Out)
	if err != nil {
		fmt.Fprintf(a.IO.Out, "%s Déploiement échoué (rollback effectué)\n",
			common.ErrorStyle.Render(common.IconError))
		return err
	}

	fmt.Fprintf(a.IO.Out, "%s Déploiement terminé en %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		time.Since(start).Round(time.Millisecond))
	return nil
}

// findHubDir locates the hub directory by looking upward from cwd.
func findHubDir() string {
	// Check common locations
	cwd, _ := os.Getwd()

	// Look for agents/ directory as marker
	candidates := []string{
		cwd,
		filepath.Dir(cwd),
		filepath.Join(os.Getenv("HOME"), ".oh", "hub"),
	}

	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "agents")); err == nil {
			return dir
		}
	}
	return ""
}

// buildMCPServers constructs MCP server definitions from the app config.
func buildMCPServers(a *app.App) []deploy.MCPServerDef {
	enabled := map[string]bool{
		"figma":   a.Config.MCP.Figma.Enabled,
		"gitlab":  a.Config.MCP.Gitlab.Enabled,
		"gslides": a.Config.MCP.Gslides.Enabled,
	}
	tokenKeys := map[string]string{
		"figma":   a.Config.MCP.Figma.Token,
		"gitlab":  a.Config.MCP.Gitlab.Token,
		"gslides": a.Config.MCP.Gslides.Token,
	}
	return deploy.DefaultMCPServers(enabled, tokenKeys)
}
