package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronise agents, skills et config vers les projets",
	Long: `Synchronise le contenu du hub (agents, skills, configuration, MCP) vers un ou tous les projets enregistrés.

Par défaut, synchronise le projet courant (détection auto) ou celui spécifié par --project.
Utilisez --all pour synchroniser tous les projets actifs en une seule commande.
Utilisez --dry-run pour prévisualiser les changements sans les appliquer.`,
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.Flags().StringP("project", "j", "", "ID du projet (auto-detect sinon)")
	syncCmd.Flags().Bool("all", false, "Synchroniser tous les projets actifs")
	syncCmd.Flags().Bool("dry-run", false, "Afficher les changements sans les appliquer")

	_ = syncCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
}

func runSync(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	allMode, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	hubDir := findHubDir()
	if hubDir == "" {
		return fmt.Errorf("%s", i18n.T("cmd.project.hub_not_found"))
	}

	// Determine which projects to sync
	var projects []domain.Project

	if allMode {
		// Sync all active projects
		list, err := a.Projects.List(ctx, domain.ProjectStatusActive)
		if err != nil {
			return fmt.Errorf("listing projects: %w", err)
		}
		if len(list) == 0 {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.WarningStyle.Render(common.IconWarning), i18n.T("cmd.sync.no_projects"))
			return nil
		}
		projects = list
	} else {
		// Single project
		projectID, _ := cmd.Flags().GetString("project")
		project, err := resolveProject(ctx, a, projectID)
		if err != nil {
			return err
		}
		projects = []domain.Project{*project}
	}

	// Header
	fmt.Fprintln(a.IO.Out)
	if dryRun {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.Title.Render("oh sync --dry-run"), i18n.Tf("cmd.sync.title_dryrun", len(projects)))
	} else {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.Title.Render("oh sync"), i18n.Tf("cmd.sync.title", len(projects)))
	}
	fmt.Fprintf(a.IO.Out, "  %s\n\n", i18n.Tf("cmd.sync.source_label", hubDir))

	// Process each project
	var totalSuccess, totalFailed int

	for i, project := range projects {
		if len(projects) > 1 {
			fmt.Fprintf(a.IO.Out, "  [%d/%d] %s (%s)\n",
				i+1, len(projects), common.Bold.Render(project.Name), project.Path)
		} else {
			fmt.Fprintf(a.IO.Out, "  %s\n",
				i18n.Tf("cmd.sync.project_label", fmt.Sprintf("%s (%s)", common.Bold.Render(project.Name), project.Path)))
		}

		if dryRun {
			err := syncDryRun(a, hubDir, project.Path)
			if err != nil {
				fmt.Fprintf(a.IO.Out, "    %s %v\n",
					common.ErrorStyle.Render(common.IconError), err)
				totalFailed++
			} else {
				totalSuccess++
			}
		} else {
			err := syncProject(a, hubDir, &project)
			if err != nil {
				fmt.Fprintf(a.IO.Out, "    %s %v\n",
					common.ErrorStyle.Render(common.IconError), err)
				totalFailed++
			} else {
				totalSuccess++
			}
		}

		if i < len(projects)-1 {
			fmt.Fprintln(a.IO.Out)
		}
	}

	// Summary
	fmt.Fprintln(a.IO.Out)
	if totalFailed > 0 {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.WarningStyle.Render(common.IconWarning), i18n.Tf("cmd.sync.done_partial", totalSuccess, totalFailed))
	} else {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess), i18n.Tf("cmd.sync.done", totalSuccess))
	}
	return nil
}

// syncProject performs a full sync (agents + skills + config + MCP) on one project.
func syncProject(a *app.App, hubDir string, project *domain.Project) error {
	plan := buildDeployPlan(a, project.Path, project.ID, hubDir, "", "", project.Agents, project.ModelOverrides)

	start := time.Now()
	results, err := deploy.Execute(plan)

	for _, r := range results {
		icon := common.SuccessStyle.Render(common.IconSuccess)
		if !r.Success {
			icon = common.ErrorStyle.Render(common.IconError)
		}
		fmt.Fprintf(a.IO.Out, "    %s %s\n", icon, r.Name)
	}

	if err != nil {
		return err
	}

	fmt.Fprintf(a.IO.Out, "    %s\n", i18n.Tf("cmd.sync.project_done", time.Since(start).Round(time.Millisecond)))
	return nil
}

// syncDryRun shows what would change without applying.
func syncDryRun(a *app.App, hubDir, projectPath string) error {
	report, err := deploy.ComputeDiff(hubDir, projectPath, nil)
	if err != nil {
		return fmt.Errorf("computing diff: %w", err)
	}

	if !report.HasChanges() {
		fmt.Fprintf(a.IO.Out, "    %s %s\n",
			common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.sync.uptodate"))
		return nil
	}

	added, modified, removed, _ := report.Summary()

	for _, f := range report.Files {
		switch f.Status {
		case deploy.FileAdded:
			fmt.Fprintf(a.IO.Out, "      %s %s\n", common.SuccessStyle.Render("+"), f.RelPath)
		case deploy.FileModified:
			fmt.Fprintf(a.IO.Out, "      %s %s\n", common.WarningStyle.Render("~"), f.RelPath)
		case deploy.FileRemoved:
			fmt.Fprintf(a.IO.Out, "      %s %s\n", common.ErrorStyle.Render("-"), f.RelPath)
		}
	}

	fmt.Fprintf(a.IO.Out, "    %s\n",
		i18n.Tf("cmd.sync.changes", added, modified, removed))
	return nil
}
