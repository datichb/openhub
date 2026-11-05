package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: i18n.T("cmd.teams.short"),
	Long:  i18n.T("cmd.teams.long"),
}

var teamsListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("cmd.teams.list.short"),
	RunE:  runTeamsList,
}

var teamsAddCmd = &cobra.Command{
	Use:   "add",
	Short: i18n.T("cmd.teams.add.short"),
	Long:  i18n.T("cmd.teams.add.long"),
	RunE:  runTeamsAdd,
}

var teamsRemoveCmd = &cobra.Command{
	Use:   "remove [team-id]",
	Short: i18n.T("cmd.teams.remove.short"),
	Long:  i18n.T("cmd.teams.remove.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsRemove,
}

var teamsDetachCmd = &cobra.Command{
	Use:   "detach [project-name]",
	Short: i18n.T("cmd.teams.detach.short"),
	Long:  i18n.T("cmd.teams.detach.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsDetach,
}

var teamsArchiveCmd = &cobra.Command{
	Use:   "archive [team-id]",
	Short: i18n.T("cmd.teams.archive.short"),
	Long:  i18n.T("cmd.teams.archive.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsArchive,
}

var teamsRestoreCmd = &cobra.Command{
	Use:   "restore [team-id]",
	Short: i18n.T("cmd.teams.restore.short"),
	Long:  i18n.T("cmd.teams.restore.long"),
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamsRestore,
}

func init() {
	rootCmd.AddCommand(teamsCmd)
	teamsCmd.AddCommand(teamsListCmd)
	teamsCmd.AddCommand(teamsAddCmd)
	teamsCmd.AddCommand(teamsRemoveCmd)
	teamsCmd.AddCommand(teamsDetachCmd)
	teamsCmd.AddCommand(teamsArchiveCmd)
	teamsCmd.AddCommand(teamsRestoreCmd)

	addTeamRejoinFlags(teamsAddCmd)
	teamsAddCmd.Flags().String("id", "", i18n.T("cmd.teams.add.flags.id"))
	teamsAddCmd.Flags().String("name", "", i18n.T("cmd.teams.add.flags.name"))

	addYesFlag(teamsRemoveCmd)
}

func runTeamsList(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	_ = cmd.Context()

	teams := a.Config.Teams

	if len(teams) == 0 {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.Subtitle.Render(theme.IconInfo),
			i18n.Tf("cmd.teams.list.empty",
				theme.Bold.Render("oh team rejoin --repo <url>")))
		return nil
	}

	w := cliout.Table(a.IO.Out)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		theme.Bold.Render(i18n.T("cmd.teams.list.header_id")),
		theme.Bold.Render(i18n.T("cmd.teams.list.header_name")),
		theme.Bold.Render(i18n.T("cmd.teams.list.header_member")),
		theme.Bold.Render(i18n.T("cmd.teams.list.header_repo")),
		theme.Bold.Render(i18n.T("cmd.teams.list.header_status")))

	for _, t := range teams {
		status := theme.SuccessStyle.Render(i18n.T("cmd.teams.list.status_active"))
		if !t.Enabled {
			status = theme.Subtitle.Render(i18n.T("cmd.teams.list.status_inactive"))
		}
		name := t.DisplayName()
		repo := textfmt.Clip(t.StateRepo, 50)
		if t.Solo {
			repo = i18n.T("cmd.team.solo.list_repo")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.ID, name, t.MemberID, repo, status)
	}
	w.Flush()

	// Show project count per team
	fmt.Fprintln(a.IO.Out)
	projects, err := a.Projects.List(context.Background(), "")
	if err == nil {
		teamProjects := make(map[string]int)
		soloCount := 0
		for _, p := range projects {
			if p.TeamID != nil && *p.TeamID != "" {
				teamProjects[*p.TeamID]++
			} else {
				soloCount++
			}
		}
		for _, t := range teams {
			count := teamProjects[t.ID]
			if count > 0 {
				fmt.Fprintf(a.IO.Out, "  %s\n",
					i18n.Tf("cmd.teams.list.project_count", t.DisplayName(), count))
			}
		}
		if soloCount > 0 {
			fmt.Fprintf(a.IO.Out, "  %s\n",
				i18n.Tf("cmd.teams.list.project_count",
					theme.Subtitle.Render(i18n.T("cmd.teams.list.solo")), soloCount))
		}
	}

	return nil
}

// runTeamsAdd is `oh teams add`: the same command as `oh team rejoin` (the access
// is probed and set up, the team-state cloned, the member picked or created and
// verified, hub.toml written), with the ID and the name of the team to choose.
// It used to write hub.toml alone, with a member_id nobody checked: the ID of
// a colleague became "your" card.
func runTeamsAdd(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	f := readRejoinFlags(cmd)
	f.noRetroTag = true // the sessions of this machine belong to the first team
	f.teamID, _ = cmd.Flags().GetString("id")
	f.teamName, _ = cmd.Flags().GetString("name")
	f.gitlabURL = strings.TrimSpace(f.gitlabURL)
	if msg := teamGitLabURLProblem(f.gitlabURL); msg != "" {
		return errors.New(msg)
	}
	if f.repo == "" {
		return errors.New(i18n.T("cmd.teams.add.repo_required"))
	}

	// A duplicate is told before anything is asked or cloned
	if f.teamID != "" {
		if existing := a.Config.FindTeam(f.teamID); existing != nil {
			return errors.New(i18n.Tf("cmd.teams.add.duplicate_id", f.teamID, existing.StateRepo))
		}
	}
	if existing := a.Config.FindTeamByRepo(f.repo); existing != nil {
		return errors.New(i18n.Tf("cmd.teams.add.duplicate_repo", existing.ID))
	}

	r := &cliRejoin{ctx: ctx, a: a, p: newCLIPrompter(a), flags: f}
	if err := r.run(); err != nil {
		return fmt.Errorf("%s: %w", i18n.Tf("v54a.teams.add.failed", f.repo), err)
	}
	return nil
}

func runTeamsRemove(cmd *cobra.Command, args []string) error {
	a := MustApp()
	teamID := args[0]

	// Find the team
	team := a.Config.FindTeam(teamID)
	if team == nil {
		return errors.New(i18n.Tf("cmd.teams.not_found",
			teamID, theme.Bold.Render("oh teams list")))
	}

	// Check for projects still attached
	projects, err := a.Projects.List(context.Background(), "")
	if err == nil {
		var attached []string
		for _, p := range projects {
			if p.TeamID != nil && *p.TeamID == teamID {
				attached = append(attached, p.Name)
			}
		}
		if len(attached) > 0 {
			cliout.Warn(a.IO.Out, i18n.Tf("cmd.teams.remove.attached_projects", len(attached)))
			for _, name := range attached {
				fmt.Fprintf(a.IO.Out, "  - %s\n", name)
			}
			fmt.Fprintf(a.IO.Out, "%s\n\n", i18n.T("cmd.teams.remove.will_become_solo"))
		}
	}

	confirm, err := confirmAction(cmd, i18n.Tf("cmd.teams.remove.confirm", teamID), false)
	if err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	// Remove from config
	if err := updateHubConfig(a, func(c *config.Config) error {
		newTeams := make([]config.TeamConfig, 0, len(c.Teams))
		for _, t := range c.Teams {
			if t.ID != teamID {
				newTeams = append(newTeams, t)
			}
		}
		c.Teams = newTeams
		return nil
	}); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}

	// Detach projects from the removed team
	if projects != nil {
		ctx := context.Background()
		for i := range projects {
			if projects[i].TeamID != nil && *projects[i].TeamID == teamID {
				projects[i].TeamID = nil
				_ = a.Projects.Update(ctx, &projects[i])
			}
		}
	}

	cliout.Success(a.IO.Out, i18n.Tf("cmd.teams.remove.success", theme.Bold.Render(teamID)))

	return nil
}

// runTeamsDetach detaches a single project from its team.
func runTeamsDetach(_ *cobra.Command, args []string) error {
	a := MustApp()
	projectName := args[0]

	ctx := context.Background()
	project, err := a.Projects.GetByName(ctx, projectName)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.Tf("cmd.teams.detach.project_not_found", projectName), err)
	}

	if project.TeamID == nil {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.Subtitle.Render(theme.IconWarning),
			i18n.Tf("cmd.teams.detach.not_attached", theme.Bold.Render(projectName)))
		return nil
	}

	teamID := *project.TeamID
	project.TeamID = nil
	if err := a.Projects.Update(ctx, project); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.teams.detach.error"), err)
	}

	cliout.Success(a.IO.Out, i18n.Tf("cmd.teams.detach.success",
		theme.Bold.Render(projectName),
		theme.Bold.Render(teamID)))
	return nil
}

// runTeamsArchive disables a team without removing it from config.
func runTeamsArchive(_ *cobra.Command, args []string) error {
	a := MustApp()
	teamID := args[0]

	team := a.Config.FindTeam(teamID)
	if team == nil {
		return errors.New(i18n.Tf("cmd.teams.not_found",
			teamID, theme.Bold.Render("oh teams list")))
	}

	if !team.Enabled {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.Subtitle.Render(theme.IconWarning),
			i18n.Tf("cmd.teams.archive.already_archived", theme.Bold.Render(teamID)))
		return nil
	}

	// Set enabled = false
	if err := updateHubConfig(a, func(c *config.Config) error {
		for i := range c.Teams {
			if c.Teams[i].ID == teamID {
				c.Teams[i].Enabled = false
				break
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}

	cliout.Success(a.IO.Out, i18n.Tf("cmd.teams.archive.success",
		theme.Bold.Render(teamID),
		theme.Bold.Render("oh teams restore "+teamID)))
	return nil
}

// runTeamsRestore re-enables an archived team.
func runTeamsRestore(_ *cobra.Command, args []string) error {
	a := MustApp()
	teamID := args[0]

	team := a.Config.FindTeam(teamID)
	if team == nil {
		return errors.New(i18n.Tf("cmd.teams.not_found",
			teamID, theme.Bold.Render("oh teams list")))
	}

	if team.Enabled {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.Subtitle.Render(theme.IconWarning),
			i18n.Tf("cmd.teams.restore.already_active", theme.Bold.Render(teamID)))
		return nil
	}

	// Set enabled = true
	if err := updateHubConfig(a, func(c *config.Config) error {
		for i := range c.Teams {
			if c.Teams[i].ID == teamID {
				c.Teams[i].Enabled = true
				break
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}

	cliout.Success(a.IO.Out, i18n.Tf("cmd.teams.restore.success", theme.Bold.Render(teamID)))
	return nil
}
