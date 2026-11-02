package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func projectListCmd() *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
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
				return fmt.Errorf(i18n.T("cmd.project_list.listing_projects"), err)
			}

			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				return cliout.JSONCompact(a.IO.Out, projects)
			}

			if len(projects) == 0 {
				fmt.Fprintln(a.IO.Out, theme.Subtitle.Render(i18n.T("cmd.project.none")))
				return nil
			}

			w := cliout.Table(a.IO.Out)
			fmt.Fprintln(w, i18n.T("cmd.project.list.header"))
			for _, p := range projects {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					p.ID, p.Name, p.Language, statusIcon(p.Status), p.Path)
			}
			w.Flush()

			fmt.Fprintf(a.IO.Out, "\n%s\n", theme.Subtitle.Render(i18n.Tf("cmd.project.list.count", len(projects))))
			return nil
		},
	}

	cmd.Flags().StringVarP(&status, "status", "s", "", "")
	cmd.Flags().Bool("json", false, "")
	return cmd
}

func statusIcon(s domain.ProjectStatus) string {
	switch s {
	case domain.ProjectStatusActive:
		return theme.SuccessStyle.Render(theme.IconSuccess + " active")
	case domain.ProjectStatusArchived:
		return theme.Subtitle.Render(theme.IconDot + " archived")
	default:
		return string(s)
	}
}
