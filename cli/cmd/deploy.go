package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Déploie agents, skills et config dans un projet",
	Long: `Déploie les agents, skills, configuration provider, et serveurs MCP
dans le répertoire du projet cible. Opération transactionnelle avec rollback automatique en cas d'erreur.

Flags spéciaux :
  --check   Vérifie si des changements sont nécessaires (exit code 1 si oui)
  --diff    Affiche les changements sans les appliquer`,
	RunE: runDeploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().StringP("project", "j", "", "ID du projet (auto-detect sinon)")
	deployCmd.Flags().StringP("provider", "P", "", "Provider à configurer")
	deployCmd.Flags().StringP("model", "m", "", "Modèle à configurer")
	deployCmd.Flags().Bool("check", false, "Vérifie si les agents/skills ont changé depuis le dernier deploy")
	deployCmd.Flags().Bool("diff", false, "Affiche les changements sans les appliquer")

	_ = deployCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	checkMode, _ := cmd.Flags().GetBool("check")
	diffMode, _ := cmd.Flags().GetBool("diff")

	// Resolve project
	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(ctx, a, projectID)
	if err != nil {
		return err
	}

	// Determine hub directory (where agents/ and skills/ live)
	hubDir := findHubDir()
	if hubDir == "" {
		return fmt.Errorf("hub content not found. Run 'oh init' to initialize")
	}

	// --- Check mode: just report freshness status ---
	if checkMode {
		return runDeployCheck(a, hubDir, project.Path, project.Name, project.Agents)
	}

	// --- Diff mode: show changes without applying ---
	if diffMode {
		return runDeployDiff(a, hubDir, project.Path, project.Name, project.Agents)
	}

	// --- Normal deploy ---
	provider, _ := cmd.Flags().GetString("provider")
	model, _ := cmd.Flags().GetString("model")

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.Title.Render("oh deploy"), i18n.Tf("cmd.deploy.deploying", project.Name))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.deploy.source", hubDir))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.deploy.target", project.Path))
	fmt.Fprintln(a.IO.Out)

	// Build deployment plan (use project's selected agents from DB)
	plan := buildDeployPlan(a, project.Path, project.ID, hubDir, provider, model, project.Agents, project.ModelOverrides)

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
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.ErrorStyle.Render(common.IconError), i18n.T("cmd.deploy.failed"))
		return err
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.Tf("cmd.deploy.done", time.Since(start).Round(time.Millisecond)))
	return nil
}

// runDeployCheck verifies if agents/skills have changed since last deploy.
// Returns an error (exit code 1) if changes are detected — useful for CI/scripts.
func runDeployCheck(a *app.App, hubDir, projectPath, projectName string, selectedAgents []string) error {
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.Title.Render("oh deploy --check"), i18n.Tf("cmd.deploy.check_title", projectName))
	fmt.Fprintln(a.IO.Out)

	report, err := deploy.ComputeDiff(hubDir, projectPath, selectedAgents)
	if err != nil {
		return fmt.Errorf("calcul diff: %w", err)
	}

	if !report.HasChanges() {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.deploy.check_uptodate"))
		return nil
	}

	added, modified, removed, _ := report.Summary()
	fmt.Fprintf(a.IO.Out, "  %s %s\n",
		common.WarningStyle.Render(common.IconWarning), i18n.Tf("cmd.deploy.check_stale", added, modified, removed))
	fmt.Fprintln(a.IO.Out)

	// Show changed files summary
	for _, f := range report.Files {
		switch f.Status {
		case deploy.FileAdded:
			fmt.Fprintf(a.IO.Out, "    %s %s\n", common.SuccessStyle.Render("+"), f.RelPath)
		case deploy.FileModified:
			fmt.Fprintf(a.IO.Out, "    %s %s\n", common.WarningStyle.Render("~"), f.RelPath)
		case deploy.FileRemoved:
			fmt.Fprintf(a.IO.Out, "    %s %s\n", common.ErrorStyle.Render("-"), f.RelPath)
		}
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s\n",
		i18n.Tf("cmd.deploy.check_run_deploy", common.Bold.Render("oh deploy")))

	// Return error to signal "stale" state (exit code 1)
	return fmt.Errorf("%s", i18n.Tf("cmd.deploy.check_stale_error", added+modified+removed))
}

// runDeployDiff shows a detailed preview of what would change, without applying.
func runDeployDiff(a *app.App, hubDir, projectPath, projectName string, selectedAgents []string) error {
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.Title.Render("oh deploy --diff"), i18n.Tf("cmd.deploy.diff_title", projectName))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.deploy.source", hubDir))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.deploy.target", projectPath))
	fmt.Fprintln(a.IO.Out)

	report, err := deploy.ComputeDiff(hubDir, projectPath, selectedAgents)
	if err != nil {
		return fmt.Errorf("calcul diff: %w", err)
	}

	if !report.HasChanges() {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.deploy.diff_no_changes"))
		return nil
	}

	// Display detailed diff report
	fmt.Fprint(a.IO.Out, deploy.FormatDiffReport(report, false))
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.deploy.diff_apply", common.Bold.Render("oh deploy")))
	return nil
}

// findHubDir locates the hub directory (~/.oh/hub/).
func findHubDir() string {
	hubDir := hubcontent.HubContentDir()
	if _, err := os.Stat(filepath.Join(hubDir, "agents")); err == nil {
		return hubDir
	}
	return ""
}

// buildMCPServers constructs MCP server definitions from the app config.
func buildMCPServers(a *app.App) []deploy.MCPServerDef {
	enabled := map[string]bool{
		"figma":   a.Config.MCP.Figma.Enabled,
		"gitlab":  a.Config.MCP.Gitlab.Enabled,
		"gslides": a.Config.MCP.Gslides.Enabled,
		"team":    a.Config.Team.Enabled,
	}
	tokenKeys := map[string]string{
		"figma":   a.Config.MCP.Figma.Token,
		"gitlab":  a.Config.MCP.Gitlab.Token,
		"gslides": a.Config.MCP.Gslides.Token,
	}
	writeEnabled := map[string]bool{
		"gitlab": a.Config.MCP.Gitlab.WriteEnabled,
	}
	return deploy.DefaultMCPServers(enabled, tokenKeys, writeEnabled)
}

// buildDeployPlan creates a standard deployment plan with all phases.
// provider and model can be empty to inherit from project config.
// projectModelOvr can be nil if the project has no per-agent/family overrides.
func buildDeployPlan(a *app.App, projectPath, projectID, hubDir, provider, model string, selectedAgents []string, projectModelOvr *domain.ProjectModelOverrides) *deploy.Plan {
	// Read websearch setting from hub config
	v := configViper()
	websearchEnabled := v.GetBool("websearch.enabled")

	// Resolve provider for model normalization
	resolvedProvider := provider
	if resolvedProvider == "" {
		resolvedProvider = a.Config.Opencode.DefaultProvider
	}
	if resolvedProvider == "" {
		resolvedProvider = "bedrock" // ultimate fallback
	}

	// Build hub-level model overrides from hub.toml [models] section
	var hubOverrides *deploy.ModelOverrides
	if a.Config.Models.Default != "" || len(a.Config.Models.Families) > 0 || len(a.Config.Models.Agents) > 0 {
		hubOverrides = &deploy.ModelOverrides{
			Default:  a.Config.Models.Default,
			Families: a.Config.Models.Families,
			Agents:   a.Config.Models.Agents,
		}
	}

	// Build project-level model overrides combining the global model + per-agent/family from DB
	var projectOverrides *deploy.ModelOverrides
	if model != "" || projectModelOvr != nil {
		projectOverrides = &deploy.ModelOverrides{
			Default: model, // project-level global model (level 3)
		}
		if projectModelOvr != nil {
			projectOverrides.Families = projectModelOvr.Families // level 2
			projectOverrides.Agents = projectModelOvr.Agents     // level 1
		}
	}

	// Build list of enabled MCP servers for agent validation warnings
	mcpServers := buildMCPServers(a)
	var enabledMCPServers []string
	for _, s := range mcpServers {
		if s.Enabled {
			enabledMCPServers = append(enabledMCPServers, s.Name)
		}
	}

	return &deploy.Plan{
		ProjectPath:       projectPath,
		ProjectID:         projectID,
		HubDir:            hubDir,
		Provider:          provider,
		Model:             model,
		WebsearchEnabled:  websearchEnabled,
		SelectedAgents:    selectedAgents,
		EnabledMCPServers: enabledMCPServers,
		Phases: []deploy.Phase{
			deploy.DeployAgents(hubDir, selectedAgents),
			deploy.DeploySkills(hubDir, selectedAgents),
			deploy.DeployConfig(provider, model),
			deploy.DeployAgentConfig(hubDir, selectedAgents, projectOverrides, hubOverrides, resolvedProvider),
			deploy.DeployMCP(mcpServers, "oh"),
		},
	}
}
