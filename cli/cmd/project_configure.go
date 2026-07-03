package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

func init() {
	projectCmd.AddCommand(projectRenameCmd())
	projectCmd.AddCommand(projectMoveCmd())
	projectCmd.AddCommand(projectConfigureCmd())
}

func projectRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename [project-id] [new-name]",
		Short: "Renomme un projet",
		Long:  "Change le nom d'affichage d'un projet enregistré. L'ID ne change pas.",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			ctx := cmd.Context()

			// Resolve project
			var projectID string
			if len(args) > 0 {
				projectID = args[0]
			}
			project, err := resolveProject(ctx, a, projectID)
			if err != nil {
				return err
			}

			// Get new name
			var newName string
			if len(args) > 1 {
				newName = args[1]
			} else {
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title(i18n.T("common.new_name")).
							Description(i18n.Tf("form.configure.current", project.Name)).
							Value(&newName),
					),
				)
				if err := form.Run(); err != nil {
					return err
				}
			}

			if newName == "" {
				return fmt.Errorf("%s", i18n.T("cmd.project.rename.empty_name"))
			}

			oldName := project.Name
			project.Name = newName
			project.UpdatedAt = time.Now()

			if err := a.Projects.Update(ctx, project); err != nil {
				return fmt.Errorf("updating project: %w", err)
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.Tf("cmd.project.renamed", oldName, common.Bold.Render(newName)))
			return nil
		},
	}
}

func projectMoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "move [project-id] [new-path]",
		Short: "Change le chemin d'un projet",
		Long: `Met à jour le chemin enregistré d'un projet dans le hub.
Utile si le projet a été déplacé sur le filesystem.
Ne déplace PAS physiquement le dossier.`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			ctx := cmd.Context()

			// Resolve project
			var projectID string
			if len(args) > 0 {
				projectID = args[0]
			}
			project, err := resolveProject(ctx, a, projectID)
			if err != nil {
				return err
			}

			// Get new path
			var newPath string
			if len(args) > 1 {
				newPath = args[1]
			} else {
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title(i18n.T("common.new_path")).
							Description(i18n.Tf("form.configure.current", project.Path)).
							Value(&newPath),
					),
				)
				if err := form.Run(); err != nil {
					return err
				}
			}

			if newPath == "" {
				return fmt.Errorf("%s", i18n.T("cmd.project.move.empty_path"))
			}

			absPath, err := filepath.Abs(newPath)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			// Verify the new path exists
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				return fmt.Errorf("%s", i18n.Tf("cmd.project.add.dir_not_exist", absPath))
			}

			oldPath := project.Path
			project.Path = absPath
			project.UpdatedAt = time.Now()

			if err := a.Projects.Update(ctx, project); err != nil {
				return fmt.Errorf("updating project: %w", err)
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n  %s → %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.Tf("cmd.project.moved", common.Bold.Render(project.Name)),
				oldPath, absPath)
			return nil
		},
	}
}

func projectConfigureCmd() *cobra.Command {
	var (
		provider string
		model    string
		language string
		tracker  string
	)

	cmd := &cobra.Command{
		Use:   "configure [project-id]",
		Short: "Configure les paramètres d'un projet",
		Long: `Configure les paramètres spécifiques d'un projet : provider LLM, modèle,
langage, tracker. Ces paramètres sont persistés et utilisés par oh start.

En mode non-interactif, passez les flags correspondants.
Sans flags, lance un wizard interactif.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			ctx := cmd.Context()

			// Resolve project
			var projectID string
			if len(args) > 0 {
				projectID = args[0]
			}
			project, err := resolveProject(ctx, a, projectID)
			if err != nil {
				return err
			}

			// If no flags, run interactive
			hasFlags := cmd.Flags().Changed("provider") || cmd.Flags().Changed("model") ||
				cmd.Flags().Changed("language") || cmd.Flags().Changed("tracker")

			if !hasFlags {
				return runProjectConfigureInteractive(ctx, a, project)
			}

			// Apply flag values
			changed := false
			if cmd.Flags().Changed("provider") {
				// Store as a label for now — actual override mechanism in config task (1.8)
				changed = true
				fmt.Fprintf(a.IO.Out, "  Provider: %s\n", provider)
			}
			if cmd.Flags().Changed("model") {
				changed = true
				fmt.Fprintf(a.IO.Out, "  Model: %s\n", model)
			}
			if cmd.Flags().Changed("language") {
				project.Language = language
				changed = true
			}
			if cmd.Flags().Changed("tracker") {
				project.Tracker = tracker
				changed = true
			}

			if changed {
				project.UpdatedAt = time.Now()
				if err := a.Projects.Update(ctx, project); err != nil {
					return fmt.Errorf("updating project: %w", err)
				}
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					common.SuccessStyle.Render(common.IconSuccess),
					i18n.Tf("cmd.project.configured", common.Bold.Render(project.Name)))
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&provider, "provider", "P", "", "Provider LLM (bedrock, anthropic, openai)")
	cmd.Flags().StringVarP(&model, "model", "m", "", "Modèle LLM")
	cmd.Flags().StringVarP(&language, "language", "l", "", "Langage principal")
	cmd.Flags().StringVarP(&tracker, "tracker", "t", "", "Issue tracker")

	return cmd
}

func runProjectConfigureInteractive(ctx context.Context, a *app.App, project *domain.Project) error {
	fmt.Fprintf(a.IO.Out, "%s Configuration de %s\n\n",
		common.Title.Render("oh project configure"),
		common.Bold.Render(project.Name))

	language := project.Language
	tracker := project.Tracker

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(i18n.T("cmd.init.language")).
				Description(i18n.Tf("form.configure.current", displayOrDefault(project.Language, i18n.T("form.configure.undefined")))).
				Options(
					huh.NewOption("Go", "go"),
					huh.NewOption("TypeScript", "typescript"),
					huh.NewOption("Python", "python"),
					huh.NewOption("Rust", "rust"),
					huh.NewOption("Java", "java"),
					huh.NewOption(i18n.T("form.option.other"), "other"),
					huh.NewOption(i18n.T("form.option.no_change"), ""),
				).
				Value(&language),

			huh.NewSelect[string]().
				Title(i18n.T("cmd.init.tracker")).
				Description(i18n.Tf("form.configure.current", displayOrDefault(project.Tracker, i18n.T("form.configure.none")))).
				Options(
					huh.NewOption(i18n.T("form.option.none"), ""),
					huh.NewOption("GitHub Issues", "github"),
					huh.NewOption("GitLab Issues", "gitlab"),
					huh.NewOption("Jira", "jira"),
					huh.NewOption("Linear", "linear"),
					huh.NewOption(i18n.T("form.option.no_change"), "_keep"),
				).
				Value(&tracker),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	changed := false
	if language != "" && language != project.Language {
		project.Language = language
		changed = true
	}
	if tracker != "_keep" && tracker != project.Tracker {
		project.Tracker = tracker
		changed = true
	}

	if !changed {
		fmt.Fprintf(a.IO.Out, "  %s\n", i18n.T("cmd.project.no_changes"))
		return nil
	}

	project.UpdatedAt = time.Now()
	if err := a.Projects.Update(ctx, project); err != nil {
		return fmt.Errorf("updating project: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.SuccessStyle.Render(common.IconSuccess),
		i18n.T("cmd.project.config_updated"))
	return nil
}
