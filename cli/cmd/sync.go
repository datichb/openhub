package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronise la configuration hub vers le projet",
	Long:  "Copie les fichiers de configuration du hub vers le projet actuel sans déployer les agents/skills.",
	RunE:  runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().StringP("project", "j", "", "ID du projet (auto-detect sinon)")
}

func runSync(cmd *cobra.Command, args []string) error {
	a := GetApp()

	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(a, projectID)
	if err != nil {
		return err
	}

	provider := a.Config.Opencode.Channel // use configured defaults
	_ = provider

	hubDir := findHubDir()
	if hubDir == "" {
		return fmt.Errorf("impossible de trouver le répertoire hub")
	}

	fmt.Fprintf(a.IO.Out, "%s Synchronisation vers %s\n",
		common.Title.Render("oh sync"), common.Bold.Render(project.Name))

	plan := &deploy.Plan{
		ProjectPath: project.Path,
		ProjectID:   project.ID,
		HubDir:      hubDir,
		Phases: []deploy.Phase{
			deploy.DeployConfig("", ""), // sync config only (preserve existing)
		},
	}

	results, err := deploy.Execute(plan)
	for _, r := range results {
		icon := common.SuccessStyle.Render(common.IconSuccess)
		if !r.Success {
			icon = common.ErrorStyle.Render(common.IconError)
		}
		fmt.Fprintf(a.IO.Out, "  %s %s\n", icon, r.Name)
	}

	if err != nil {
		return err
	}

	fmt.Fprintf(a.IO.Out, "\n%s Synchronisation terminée.\n",
		common.SuccessStyle.Render(common.IconSuccess))
	return nil
}
