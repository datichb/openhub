package cmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func projectRemoveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "remove [id]",
		Aliases: []string{"rm"},
		Short:   "Supprime un projet du registre",
		Long:    "Supprime un projet du registre (ne supprime pas les fichiers sur disque).",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			ctx := cmd.Context()

			var projectID string
			if len(args) > 0 {
				projectID = args[0]
			}

			// ── Non-interactive path: arg provided + force ──
			if projectID != "" && force {
				project, err := a.Projects.Get(ctx, projectID)
				if err != nil {
					if errors.Is(err, domain.ErrNotFound) {
						return fmt.Errorf("%s", i18n.Tf("cmd.project.not_found", projectID))
					}
					return err
				}
				if err := a.Projects.Delete(ctx, projectID); err != nil {
					return fmt.Errorf("deleting project: %w", err)
				}
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					theme.SuccessStyle.Render(theme.IconSuccess),
					i18n.Tf("form.project.removed", theme.Bold.Render(project.Name)))
				return nil
			}

			// ── Non-interactive with confirm (arg provided, no force) ──
			if projectID != "" && !force {
				project, err := a.Projects.Get(ctx, projectID)
				if err != nil {
					if errors.Is(err, domain.ErrNotFound) {
						return fmt.Errorf("%s", i18n.Tf("cmd.project.not_found", projectID))
					}
					return err
				}

				// Simple inline confirm (single field — stays on huh)
				var confirm bool
				confirmForm := theme.NewForm(
					huh.NewGroup(
						huh.NewConfirm().
							Title(i18n.Tf("form.project.confirm_delete", project.Name)).
							Description(i18n.T("form.project.delete_hint")).
							Value(&confirm),
					),
				)
				if err := confirmForm.Run(); err != nil {
					return err
				}
				if !confirm {
					fmt.Fprintln(a.IO.Out, theme.Subtitle.Render(i18n.T("form.project.cancelled")))
					return nil
				}

				if err := a.Projects.Delete(ctx, projectID); err != nil {
					return fmt.Errorf("deleting project: %w", err)
				}
				fmt.Fprintf(a.IO.Out, "%s %s\n",
					theme.SuccessStyle.Render(theme.IconSuccess),
					i18n.Tf("form.project.removed", theme.Bold.Render(project.Name)))
				return nil
			}

			// ── Interactive path: wizard to select + confirm ──
			projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
			if err != nil {
				return err
			}
			if len(projects) == 0 {
				fmt.Fprintln(a.IO.Out, theme.Subtitle.Render(i18n.T("form.project.none_to_remove")))
				return nil
			}

			// Build options for dropdown
			projectNames := make([]string, len(projects))
			projectIDs := make([]string, len(projects))
			for i, p := range projects {
				projectNames[i] = fmt.Sprintf("%s (%s)", p.Name, p.Path)
				projectIDs[i] = p.ID
			}

			var selectedIdx int
			var confirm bool

			steps := []views.WizardStep{
				{
					Label: "Select Project",
					Form: func(_ *tview.Application, onDone func()) *tview.Form {
						form := tview.NewForm()
						form.AddDropDown(
							i18n.T("form.project.select_remove"),
							projectNames, 0,
							func(_ string, idx int) { selectedIdx = idx })
						form.AddButton("Next", func() {
							projectID = projectIDs[selectedIdx]
							onDone()
						})
						return form
					},
					InfoFields: func() []views.InfoField {
						return []views.InfoField{
							{Label: "Project", Value: projectNames[selectedIdx]},
						}
					},
				},
				{
					Label: "Confirm",
					Skip:  force, // skip if --force
					Form: func(_ *tview.Application, onDone func()) *tview.Form {
						form := tview.NewForm()
						form.AddCheckbox(
							i18n.Tf("form.project.confirm_delete", projectNames[selectedIdx]),
							false,
							func(checked bool) { confirm = checked })
						form.AddButton("Delete", func() { onDone() })
						return form
					},
					OnDone: func() error {
						if !confirm {
							fmt.Fprintln(a.IO.Out, theme.Subtitle.Render(i18n.T("form.project.cancelled")))
							return fmt.Errorf("cancelled")
						}
						if err := a.Projects.Delete(ctx, projectID); err != nil {
							return fmt.Errorf("deleting project: %w", err)
						}
						return nil
					},
					InfoFields: func() []views.InfoField {
						return []views.InfoField{{Label: "Status", Value: "deleted"}}
					},
				},
			}

			wizResult := views.RunWizard(views.WizardConfig{
				Layout: layout.Config{
					ProjectName: a.Config.Name,
					Command:     "project remove",
					StatusHints: "enter confirm · esc skip",
				},
				Steps: steps,
			})

			if wizResult.Aborted {
				return nil
			}
			if wizResult.Err != nil {
				if wizResult.Err.Error() == "cancelled" {
					return nil
				}
				return wizResult.Err
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess),
				i18n.Tf("form.project.removed", theme.Bold.Render(projectNames[selectedIdx])))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Supprime sans confirmation")
	return cmd
}
