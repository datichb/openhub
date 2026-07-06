package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

func projectListCmd() *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Liste les projets enregistrés",
		RunE: func(cmd *cobra.Command, args []string) error {
			a := MustApp()
			ctx := cmd.Context()

			// Validate status value
			switch status {
			case "", "active", "archived":
				// valid
			default:
				return fmt.Errorf("%s", i18n.Tf("cmd.project.status_invalid", status))
			}

			projects, err := a.Projects.List(ctx, domain.ProjectStatus(status))
			if err != nil {
				return fmt.Errorf("listing projects: %w", err)
			}

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				return json.NewEncoder(os.Stdout).Encode(projects)
			}

			if len(projects) == 0 {
				fmt.Fprintln(a.IO.Out, common.Subtitle.Render(i18n.T("cmd.project.none")))
				return nil
			}

			w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, i18n.T("cmd.project.list.header"))
			for _, p := range projects {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					p.ID, p.Name, p.Language, statusIcon(p.Status), p.Path)
			}
			w.Flush()

			fmt.Fprintf(a.IO.Out, "\n%s\n", common.Subtitle.Render(i18n.Tf("cmd.project.list.count", len(projects))))
			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "Filtrer par statut (active, archived)")
	cmd.Flags().Bool("json", false, "Output in JSON format")
	return cmd
}

func statusIcon(s domain.ProjectStatus) string {
	switch s {
	case domain.ProjectStatusActive:
		return common.SuccessStyle.Render(common.IconSuccess + " active")
	case domain.ProjectStatusArchived:
		return common.Subtitle.Render(common.IconDot + " archived")
	default:
		return string(s)
	}
}
