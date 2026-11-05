package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func projectSetTeamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-team <project> <team>",
		Short: i18n.T("v54r.project.set_team.short"),
		Long:  i18n.T("v54r.project.set_team.long"),
		Args:  cobra.ExactArgs(2),
		RunE:  runProjectSetTeam,
	}
	cmd.ValidArgsFunction = func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return completeProjectIDs(c, args, toComplete)
		case 1:
			var ids []string
			if a := TryApp(); a != nil {
				for _, t := range a.Config.Teams {
					if t.Enabled {
						ids = append(ids, t.ID)
					}
				}
			}
			return ids, cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return cmd
}

func runProjectSetTeam(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	project, err := resolveProject(ctx, a, args[0], projectRead)
	if err != nil {
		return err
	}
	previous := ""
	if project.TeamID != nil {
		previous = *project.TeamID
	}
	if err := setProjectTeam(ctx, a, project, args[1]); err != nil {
		return err
	}
	if previous == args[1] {
		fmt.Fprintln(a.IO.Out, i18n.Tf("v54r.project.set_team.unchanged", theme.Bold.Render(project.Name), theme.Bold.Render(args[1])))
		return nil
	}
	cliout.Success(a.IO.Out, i18n.Tf("v54r.project.set_team.done", theme.Bold.Render(project.Name), theme.Bold.Render(args[1])))
	if previous != "" {
		cliout.Warn(a.IO.Out, i18n.Tf("v54r.project.set_team.moved", theme.Bold.Render(previous)))
	}
	return nil
}

// checkProjectTeam refuses a team a project cannot be attached to: unknown or
// archived.
func checkProjectTeam(a *app.App, teamID string) error {
	team := a.Config.FindTeam(teamID)
	switch {
	case team == nil:
		return errors.New(i18n.Tf("cmd.teams.not_found", teamID, theme.Bold.Render("oh teams list")))
	case !team.Enabled:
		return errors.New(i18n.Tf("v54r.team.archived", teamID, theme.Bold.Render("oh teams restore "+teamID)))
	}
	return nil
}

// setProjectTeam attaches project to the team teamID. The team must exist and
// be enabled (an archived team is refused: it is restored first); a solo space
// gets the workflow folders of the project, as for any attachment to it.
func setProjectTeam(ctx context.Context, a *app.App, project *domain.Project, teamID string) error {
	if err := checkProjectTeam(a, teamID); err != nil {
		return err
	}
	if team := a.Config.FindTeam(teamID); team.Solo {
		repo := teamstate.NewRepo("", team.StatePath)
		if !repo.IsCloned() {
			return errors.New(i18n.Tf("v54r.project.set_team.solo_not_cloned", teamID))
		}
		return attachProjectToSolo(ctx, a, repo, project, teamID)
	}
	id := teamID
	project.TeamID = &id
	project.TeamConfig = nil // the former per-project override is replaced
	return a.Projects.Update(ctx, project)
}
