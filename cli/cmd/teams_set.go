package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// oh teams set <id> repo <url>: the repository of a team moved or was renamed
// on the server. The URL is the only thing that changes: the clone, the claims,
// the card and the keys of the team stay. The new URL is checked before hub.toml
// is touched: the access (probed, the token asked and kept as for a rejoin) and
// the history (it must be the same team-state, not another one).

var teamsSetCmd = &cobra.Command{
	Use:   "set <team-id> repo|name|member-id|enabled|default [value]",
	Short: i18n.T("v54a.teams.set.short"),
	Long:  i18n.T("v54a.teams.set.long"),
	Args:  cobra.RangeArgs(2, 3),
	RunE:  runTeamsSet,
}

func init() {
	teamsCmd.AddCommand(teamsSetCmd)
	teamsSetCmd.Flags().String("gitlab-url", "", i18n.T("cmd.team.rejoin.flags.gitlab_url"))
}

// repoMover is the clone of a team-state whose repository changes.
type repoMover interface {
	teamstate.RepoInfo
	VerifyRemote(ctx context.Context, remote string) error
	SetRemote(ctx context.Context, remote string) error
	HasMember(ctx context.Context, id string) bool
}

func runTeamsSet(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	id, field := args[0], strings.ToLower(args[1])
	value := ""
	if len(args) > 2 {
		value = strings.TrimSpace(args[2])
	}
	gitlabURL, _ := cmd.Flags().GetString("gitlab-url")
	if field == "repo" {
		if len(args) < 3 {
			return errors.New(i18n.T("v54a.teams.set.repo_required"))
		}
		return setTeamRepo(ctx, a, id, value, strings.TrimSpace(gitlabURL))
	}
	cmd.SetContext(ctx)
	return setTeamField(cmd, a, id, field, value, len(args) > 2)
}

func setTeamRepo(ctx context.Context, a *app.App, id, repoURL, gitlabURL string) error {
	team := a.Config.FindTeam(id)
	if team == nil {
		return errors.New(i18n.Tf("cmd.teams.not_found", id, theme.Bold.Render("oh teams list")))
	}
	if team.Solo || team.StateRepo == "" {
		return errors.New(i18n.Tf("v54a.teams.set.solo", id, theme.Bold.Render("oh team promote --remote <url>")))
	}
	if repoURL == "" {
		return errors.New(i18n.T("v54a.teams.set.repo_required"))
	}
	if msg := teamGitLabURLProblem(gitlabURL); msg != "" {
		return errors.New(msg)
	}
	if teamstate.SameRemote(team.StateRepo, repoURL) && gitlabURL == "" {
		cliout.Success(a.IO.Out, i18n.Tf("v54a.teams.set.same", id, team.StateRepo))
		return nil
	}
	if other := a.Config.FindTeamByRepo(repoURL); other != nil && other.ID != id {
		return errors.New(i18n.Tf("cmd.teams.add.duplicate_repo", other.ID))
	}

	// 1. the access, as a rejoin does
	if err := newCLIPrompter(a).ensureAccess(ctx, a, repoURL); err != nil {
		return err
	}
	// 2. the same team-state
	var clone repoMover = teamstate.NewRepo(team.StateRepo, config.ResolveStatePath(team.StateRepo, team.StatePath))
	if clone.IsCloned() {
		if err := clone.VerifyRemote(ctx, repoURL); err != nil {
			return setRepoError(repoURL, err)
		}
		if err := clone.SetRemote(ctx, repoURL); err != nil {
			return err
		}
		if team.MemberID != "" && !clone.HasMember(ctx, team.MemberID) {
			cliout.Warn(a.IO.Out, i18n.Tf("v54a.teams.set.member_missing", team.MemberID, id))
		}
	}
	// 3. hub.toml, last
	if err := updateHubConfig(a, func(c *config.Config) error {
		t := c.FindTeam(id)
		if t == nil {
			return fmt.Errorf("%s", i18n.Tf("cmd.teams.not_found", id, "oh teams list"))
		}
		if t.StatePath == "" {
			t.StatePath = config.ResolveStatePath(t.StateRepo, t.StatePath)
		}
		t.StateRepo = repoURL
		if gitlabURL != "" {
			t.GitLabURL = gitlabURL
		}
		return nil
	}); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}
	cliout.Success(a.IO.Out, i18n.Tf("v54a.teams.set.done", id, repoURL))
	return nil
}

// setRepoError words why the new repository cannot replace the old one.
func setRepoError(repoURL string, err error) error {
	switch {
	case errors.Is(err, teamstate.ErrUnrelatedRemote):
		return errors.New(i18n.Tf("v54a.teams.set.unrelated", repoURL))
	case errors.Is(err, teamstate.ErrEmptyRemote):
		return errors.New(i18n.Tf("v54a.teams.set.empty", repoURL))
	}
	if teamstate.AccessError(err) != nil {
		return errors.New(i18n.Tf("v532w.access.refused_https", repoURL, gitCause(err)))
	}
	return fmt.Errorf(i18n.T("v532w.probe.failed"), repoURL, gitCause(err))
}
