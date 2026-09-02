package cmd

import (
	"context"
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/components/floating"
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

	teamsAddCmd.Flags().String("repo", "", i18n.T("cmd.teams.add.flags.repo"))
	teamsAddCmd.Flags().String("member-id", "", i18n.T("cmd.teams.add.flags.member_id"))
	teamsAddCmd.Flags().String("id", "", i18n.T("cmd.teams.add.flags.id"))
	teamsAddCmd.Flags().String("name", "", i18n.T("cmd.teams.add.flags.name"))

	teamsRemoveCmd.Flags().BoolP("force", "f", false, i18n.T("cmd.teams.remove.flags.force"))
}

func runTeamsList(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	_ = cmd.Context()

	teams := a.Config.Teams

	if len(teams) == 0 {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.Subtitle.Render(theme.IconInfo),
			i18n.Tf("cmd.teams.list.empty",
				theme.Bold.Render("oh teams add --repo <url> --member-id <id>")))
		return nil
	}

	w := tabwriter.NewWriter(a.IO.Out, 0, 4, 2, ' ', 0)
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
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.ID, name, t.MemberID, truncateStr(t.StateRepo, 50), status)
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

func runTeamsAdd(cmd *cobra.Command, _ []string) error {
	a := MustApp()

	repo, _ := cmd.Flags().GetString("repo")
	memberID, _ := cmd.Flags().GetString("member-id")
	id, _ := cmd.Flags().GetString("id")
	name, _ := cmd.Flags().GetString("name")

	if repo == "" {
		return errors.New(i18n.T("cmd.teams.add.repo_required"))
	}
	if memberID == "" {
		return errors.New(i18n.T("cmd.teams.add.member_id_required"))
	}

	// Auto-derive ID from repo if not provided
	if id == "" {
		id = config.RepoNameFromRemote(repo)
	}

	// Check for duplicates
	if existing := a.Config.FindTeam(id); existing != nil {
		return errors.New(i18n.Tf("cmd.teams.add.duplicate_id", id, existing.StateRepo))
	}
	if existing := a.Config.FindTeamByRepo(repo); existing != nil {
		return errors.New(i18n.Tf("cmd.teams.add.duplicate_repo", existing.ID))
	}

	// Build the new team entry
	newTeam := config.TeamConfig{
		ID:        id,
		Name:      name,
		Enabled:   true,
		StateRepo: repo,
		StatePath: config.TeamStatePath(repo),
		MemberID:  memberID,
	}

	a.Config.Teams = append(a.Config.Teams, newTeam)

	if err := config.Save(a.Config); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.teams.add.success", theme.Bold.Render(newTeam.DisplayName())))
	fmt.Fprintf(a.IO.Out, "%s\n", i18n.Tf("cmd.teams.add.detail_id", id))
	fmt.Fprintf(a.IO.Out, "%s\n", i18n.Tf("cmd.teams.add.detail_repo", repo))
	fmt.Fprintf(a.IO.Out, "%s\n", i18n.Tf("cmd.teams.add.detail_member", memberID))
	fmt.Fprintf(a.IO.Out, "\n%s\n",
		i18n.Tf("cmd.teams.add.sync_hint", theme.Bold.Render("oh team init")))

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
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.WarningStyle.Render(theme.IconWarning),
				i18n.Tf("cmd.teams.remove.attached_projects", len(attached)))
			for _, name := range attached {
				fmt.Fprintf(a.IO.Out, "  - %s\n", name)
			}
			fmt.Fprintf(a.IO.Out, "%s\n\n", i18n.T("cmd.teams.remove.will_become_solo"))
		}
	}

	// Confirmation prompt unless --force
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		var confirm bool
		_ = floating.Run(floating.Config{
			Title: i18n.T("cmd.teams.remove.short"),
			Form: theme.NewForm(huh.NewGroup(huh.NewConfirm().
				Title(i18n.Tf("cmd.teams.remove.confirm", teamID)).
				Value(&confirm))),
		})
		if !confirm {
			return nil
		}
	}

	// Remove from config
	newTeams := make([]config.TeamConfig, 0, len(a.Config.Teams)-1)
	for _, t := range a.Config.Teams {
		if t.ID != teamID {
			newTeams = append(newTeams, t)
		}
	}
	a.Config.Teams = newTeams

	if err := config.Save(a.Config); err != nil {
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

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.teams.remove.success", theme.Bold.Render(teamID)))

	return nil
}

// truncateStr truncates a string to maxLen, adding "..." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.teams.detach.success",
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
	for i := range a.Config.Teams {
		if a.Config.Teams[i].ID == teamID {
			a.Config.Teams[i].Enabled = false
			break
		}
	}

	if err := config.Save(a.Config); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.teams.archive.success",
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
	for i := range a.Config.Teams {
		if a.Config.Teams[i].ID == teamID {
			a.Config.Teams[i].Enabled = true
			break
		}
	}

	if err := config.Save(a.Config); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.teams.restore.success", theme.Bold.Render(teamID)))
	return nil
}
