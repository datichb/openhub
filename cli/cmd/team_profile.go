package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// oh team profile: the profile of the current member in the team-state
// (members.toml): display name, user names on GitLab, Mattermost and the
// tracker. tracker_username is what lets the tracker sync attribute the issues
// of the tracker to the member when the tracker is on another server than the
// team-state repository (the name is not the same on both).

var teamProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: i18n.T("cmd.team.profile.short"),
	Long:  i18n.T("cmd.team.profile.long"),
	Args:  cobra.NoArgs,
	RunE:  runTeamProfile,
}

func init() {
	teamCmd.AddCommand(teamProfileCmd)
	f := teamProfileCmd.Flags()
	f.String("team", "", i18n.T("cmd.team.profile.flags.team"))
	f.String("display-name", "", i18n.T("cmd.team.profile.flags.display_name"))
	f.String("gitlab-username", "", i18n.T("cmd.team.profile.flags.gitlab_username"))
	f.String("mattermost-username", "", i18n.T("cmd.team.profile.flags.mattermost_username"))
	f.String("tracker-username", "", i18n.T("cmd.team.profile.flags.tracker_username"))
}

// profilePatchFromFlags is the change the flags ask for: a flag that is given
// (even empty) changes its field, the others are left as they are.
func profilePatchFromFlags(cmd *cobra.Command) teamsvc.ProfilePatch {
	var p teamsvc.ProfilePatch
	for _, f := range []struct {
		name string
		to   **string
	}{
		{"display-name", &p.DisplayName},
		{"gitlab-username", &p.GitLabUsername},
		{"mattermost-username", &p.MattermostUsername},
		{"tracker-username", &p.TrackerUsername},
	} {
		if cmd.Flags().Changed(f.name) {
			v, _ := cmd.Flags().GetString(f.name)
			*f.to = &v
		}
	}
	return p
}

func runTeamProfile(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	svc := teamService(a)
	teamID, _ := cmd.Flags().GetString("team")
	patch := profilePatchFromFlags(cmd)

	var (
		m   *teamstate.Member
		err error
	)
	if patch.Empty() {
		m, err = svc.Profile(teamID)
	} else {
		m, err = svc.UpdateProfile(cmd.Context(), teamID, patch)
	}
	if err != nil {
		return profileError(err)
	}
	if !patch.Empty() {
		fmt.Fprintf(a.IO.Out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.team.profile.updated"))
	}
	printProfile(a.IO.Out, teamID, m)
	return nil
}

// profilePatchForField is the change of one field of the profile (a
// views.MemberField* key) to value.
func profilePatchForField(field, value string) teamsvc.ProfilePatch {
	var p teamsvc.ProfilePatch
	switch field {
	case views.MemberFieldDisplayName:
		p.DisplayName = &value
	case views.MemberFieldGitLabUsername:
		p.GitLabUsername = &value
	case views.MemberFieldMattermostUsername:
		p.MattermostUsername = &value
	case views.MemberFieldTrackerUsername:
		p.TrackerUsername = &value
	}
	return p
}

// profileError translates the errors of the profile of the team service.
func profileError(err error) error {
	switch {
	case errors.Is(err, teamsvc.ErrTeamNotConfigured):
		return errors.New(i18n.Tf("cmd.team.profile.err_no_team", theme.Bold.Render("oh team rejoin")))
	case errors.Is(err, teamsvc.ErrNoMemberID):
		return errors.New(i18n.Tf("cmd.team.profile.err_no_member", theme.Bold.Render("oh team rejoin")))
	case errors.Is(err, teamsvc.ErrNotCloned):
		return errors.New(i18n.Tf("cmd.team.profile.err_not_cloned", theme.Bold.Render("oh team rejoin")))
	case errors.Is(err, teamsvc.ErrEmptyDisplayName):
		return errors.New(i18n.T("cmd.team.profile.err_empty_name"))
	case errors.Is(err, teamstate.ErrMemberNotFound):
		return errors.New(i18n.Tf("cmd.team.profile.err_member_missing", theme.Bold.Render("oh team rejoin")))
	}
	return err
}

func printProfile(out interface{ Write([]byte) (int, error) }, teamID string, m *teamstate.Member) {
	none := theme.Subtitle.Render("—")
	val := func(s string) string {
		if s == "" {
			return none
		}
		return s
	}
	fmt.Fprintf(out, "\n  %s\n\n", i18n.Tf("cmd.team.profile.title", m.ID, firstNonEmpty(teamID, "hub")))
	fmt.Fprintf(out, "  display_name         %s\n", val(m.DisplayName))
	fmt.Fprintf(out, "  gitlab_username      %s\n", val(m.GitLabUsername))
	fmt.Fprintf(out, "  mattermost_username  %s\n", val(m.MattermostUsername))
	fmt.Fprintf(out, "  tracker_username     %s\n", val(m.TrackerUsername))
	fmt.Fprintf(out, "  role                 %s\n", val(m.Role))
	if m.TrackerUsername == "" {
		fmt.Fprintf(out, "\n  %s\n", i18n.Tf("cmd.team.profile.tracker_empty", firstNonEmpty(m.GitLabUsername, "—")))
	}
}

// trackerUsernameWanted reports whether a member who joins the team must say
// who they are on the tracker: the team has a tracker and it is not on the
// server of the team-state repository (remote, or gitlabURL when the remote is
// an SSH alias), so the GitLab name of the member is not their name there.
// The tracker URL is the one the sync calls (a GitLab without tracker_url is
// gitlab.com); when a server cannot be told, the name is asked.
func trackerUsernameWanted(repo *teamstate.Repo, remote, gitlabURL string) bool {
	cfg, err := repo.LoadConfig()
	if err != nil || cfg == nil || cfg.Tracker.Type == "" {
		return false
	}
	trackerURL := cfg.Tracker.TrackerURL
	if cfg.Tracker.Type == string(tracker.TypeGitLab) {
		trackerURL = tracker.GitLabBaseURL(trackerURL)
	}
	return !tracker.SameServer(trackerURL, firstNonEmpty(gitlabURL, remote))
}
