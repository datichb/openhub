package cmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
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
			a := GetApp()

			var projectID string
			if len(args) > 0 {
				projectID = args[0]
			} else {
				// Interactive selection
				projects, err := a.Projects.List(domain.ProjectStatusActive)
				if err != nil {
					return err
				}
				if len(projects) == 0 {
					fmt.Fprintln(a.IO.Out, common.Subtitle.Render("Aucun projet à supprimer."))
					return nil
				}

				options := make([]huh.Option[string], len(projects))
				for i, p := range projects {
					options[i] = huh.NewOption(fmt.Sprintf("%s (%s)", p.Name, p.Path), p.ID)
				}

				selectForm := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title("Projet à supprimer").
							Options(options...).
							Value(&projectID),
					),
				)
				if err := selectForm.Run(); err != nil {
					return err
				}
			}

			// Verify project exists
			project, err := a.Projects.Get(projectID)
			if err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					return fmt.Errorf("projet %q introuvable", projectID)
				}
				return err
			}

			// Confirm deletion (unless --force)
			if !force {
				var confirm bool
				confirmForm := huh.NewForm(
					huh.NewGroup(
						huh.NewConfirm().
							Title(fmt.Sprintf("Supprimer %q ?", project.Name)).
							Description("Les fichiers sur disque ne seront pas supprimés.").
							Value(&confirm),
					),
				)
				if err := confirmForm.Run(); err != nil {
					return err
				}
				if !confirm {
					fmt.Fprintln(a.IO.Out, common.Subtitle.Render("Annulé."))
					return nil
				}
			}

			if err := a.Projects.Delete(projectID); err != nil {
				return fmt.Errorf("deleting project: %w", err)
			}

			fmt.Fprintf(a.IO.Out, "%s Projet %s supprimé du registre.\n",
				common.SuccessStyle.Render(common.IconSuccess),
				common.Bold.Render(project.Name),
			)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Supprime sans confirmation")
	return cmd
}
