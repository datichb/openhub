package cmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
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
			} else {
				// Interactive selection
				projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
				if err != nil {
					return err
				}
				if len(projects) == 0 {
					fmt.Fprintln(a.IO.Out, common.Subtitle.Render(i18n.T("form.project.none_to_remove")))
					return nil
				}

				options := make([]huh.Option[string], len(projects))
				for i, p := range projects {
					options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", p.Name, p.Path), p.ID)
				}

				selectForm := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title(i18n.T("form.project.select_remove")).
							Options(options...).
							Value(&projectID),
					),
				)
				if err := selectForm.Run(); err != nil {
					return err
				}
			}

			// Verify project exists
			project, err := a.Projects.Get(ctx, projectID)
			if err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					return fmt.Errorf("%s", i18n.Tf("cmd.project.not_found", projectID))
				}
				return err
			}

			// Confirm deletion (unless --force)
			if !force {
				var confirm bool
				confirmForm := huh.NewForm(
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
					fmt.Fprintln(a.IO.Out, common.Subtitle.Render(i18n.T("form.project.cancelled")))
					return nil
				}
			}

			if err := a.Projects.Delete(ctx, projectID); err != nil {
				return fmt.Errorf("deleting project: %w", err)
			}

			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.SuccessStyle.Render(common.IconSuccess),
				i18n.Tf("form.project.removed", common.Bold.Render(project.Name)),
			)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Supprime sans confirmation")
	return cmd
}
