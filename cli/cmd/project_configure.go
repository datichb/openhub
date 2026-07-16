package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
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
				form := common.NewForm(
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
				form := common.NewForm(
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

			absPath, err := filepath.Abs(expandPath(newPath))
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
	)

	cmd := &cobra.Command{
		Use:   "configure [project-id]",
		Short: "Configure les paramètres d'un projet",
		Long: `Configure les paramètres spécifiques d'un projet : provider LLM, modèle,
langage. Ces paramètres sont persistés et utilisés par oh start.

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
				cmd.Flags().Changed("language")

			if !hasFlags {
				return runProjectConfigureInteractive(ctx, a, project)
			}

			// Apply flag values
			changed := false
			if cmd.Flags().Changed("provider") {
				project.Provider = provider
				changed = true
			}
			if cmd.Flags().Changed("model") {
				project.Model = model
				changed = true
			}
			if cmd.Flags().Changed("language") {
				project.Language = language
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

	return cmd
}

func runProjectConfigureInteractive(ctx context.Context, a *app.App, project *domain.Project) error {
	// Shared state across wizard steps
	language := project.Language
	provider := project.Provider
	model := project.Model
	var modifyAgents bool
	changed := false

	languageOptions := []string{"Go", "TypeScript", "Python", "Rust", "Java",
		i18n.T("form.option.other"), i18n.T("form.option.no_change")}
	languageValues := []string{"go", "typescript", "python", "rust", "java", "other", "_keep"}

	// Find initial index for dropdown
	initialLangIdx := len(languageValues) - 1 // default to "_keep"
	for i, v := range languageValues {
		if v == project.Language {
			initialLangIdx = i
			break
		}
	}

	steps := []views.WizardStep{
		// Step 1: Language / Provider / Model
		{
			Label: i18n.T("cmd.init.language") + " / Provider / Model",
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddDropDown(
					i18n.T("cmd.init.language"),
					languageOptions, initialLangIdx,
					func(_ string, idx int) {
						if idx >= 0 && idx < len(languageValues) {
							language = languageValues[idx]
						}
					},
				)
				form.AddInputField(
					i18n.T("form.project.provider_select"),
					displayOrDefault(project.Provider, ""), 0, nil,
					func(text string) { provider = text },
				)
				form.AddInputField(
					i18n.T("form.project.model_input"),
					displayOrDefault(project.Model, ""), 0, nil,
					func(text string) { model = text },
				)
				form.AddButton("Next", func() { onDone() })
				return form
			},
			OnDone: func() error {
				if language != "_keep" && language != project.Language {
					project.Language = language
					changed = true
				}
				if provider != project.Provider {
					project.Provider = provider
					changed = true
				}
				if model != project.Model {
					project.Model = model
					changed = true
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Language", Value: displayOrDefault(project.Language, "-")},
					{Label: "Provider", Value: displayOrDefault(project.Provider, i18n.T("form.configure.hub_default"))},
					{Label: "Model", Value: displayOrDefault(project.Model, i18n.T("form.configure.hub_default"))},
				}
			},
			Processing: i18n.T("form.configure.applying"),
		},
		// Step 2: Agents modification
		{
			Label: i18n.T("form.configure.modify_agents"),
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				// Discover available agents
				available := discoverAgents()
				if len(available) == 0 {
					form.AddCheckbox("No agents found — skip", true, nil)
					form.AddButton("Next", func() { onDone() })
					return form
				}
				// Add a checkbox per agent (default: current project agents selected)
				agentSelected := make(map[string]bool)
				for _, ag := range project.Agents {
					agentSelected[ag] = true
				}
				for _, ag := range available {
					agName := ag
					form.AddCheckbox(agName, agentSelected[agName],
						func(checked bool) { agentSelected[agName] = checked })
				}
				form.AddButton("Apply", func() {
					// Build selected list
					var selected []string
					for _, ag := range available {
						if agentSelected[ag] {
							selected = append(selected, ag)
						}
					}
					project.Agents = selected
					changed = true
					modifyAgents = true
					onDone()
				})
				return form
			},
			OnDone: func() error {
				return nil
			},
			InfoFields: func() []views.InfoField {
				if modifyAgents {
					return []views.InfoField{
						{Label: "Agents", Value: fmt.Sprintf("%d configured", len(project.Agents))},
					}
				}
				return []views.InfoField{
					{Label: "Agents", Value: "unchanged"},
				}
			},
			Processing: i18n.T("form.configure.applying"),
		},
	}

	wizResult := views.RunWizard(views.WizardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "project configure",
			StatusHints: "enter confirm · esc skip",
		},
		Steps: steps,
	})

	if wizResult.Aborted {
		return nil
	}
	if wizResult.Err != nil {
		return wizResult.Err
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
