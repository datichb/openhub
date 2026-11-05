package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// leaveSetup is a team of this machine ready to be left: hub.toml on disk with
// the team (the member is testuser), a fake keychain, two open claims and a
// finished one of the member, one claim of alice, a project attached.
func leaveSetup(t *testing.T) (*app.App, *teamstate.Repo, *bytes.Buffer) {
	t.Helper()
	i18ntest.Set(t, "en")
	configtest.Home(t)
	a, repo, out := setupClaimTestApp(t)
	team := "test-team"
	a.Projects = &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "api", TeamID: &team}}}
	a.Secrets = &mockSecretStore{secrets: map[string]string{
		"openhub.team.test-team.gitlab.token": "t1",
		"openhub.team.other.gitlab.token":     "t2",
		"openhub.mcp.gitlab.token":            "t3",
	}}
	require.NoError(t, config.Save(a.Config))
	addTeamMembers(t, repo, "alice")
	claimsCreate(t, repo, "M-1", "testuser", "")
	claimsCreate(t, repo, "M-2", "testuser", "")
	_, err := repo.CreateClaim(context.Background(), teamstate.Claim{TicketID: "M-3", Project: claimsProject, ClaimedBy: "testuser", Status: teamstate.ClaimStatusDone})
	require.NoError(t, err)
	claimsCreate(t, repo, "A-1", "alice", "")
	terminalIs(t, false)
	return a, repo, out
}

func leaveCmd(t *testing.T, flags ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "leave"}
	cmd.Flags().Bool("remove-member", false, "")
	cmd.Flags().Bool("force", false, "")
	addYesFlag(cmd)
	require.NoError(t, cmd.Flags().Set("yes", "true"))
	for _, f := range flags {
		require.NoError(t, cmd.Flags().Set(f, "true"))
	}
	cmd.SetContext(context.Background())
	return cmd
}

// freshClone is another clone of the team-state: what the colleagues see.
func freshClone(t *testing.T, bare string) *teamstate.Repo {
	t.Helper()
	other := teamstate.NewRepo(bare, filepath.Join(t.TempDir(), "colleague"))
	require.NoError(t, other.Clone(context.Background()))
	return other
}

// Leaving a team releases the open claims of the member in one commit (their
// card stays, Q4), deletes the clone and the keys of the team and only those,
// and takes the team out of hub.toml with its projects.
func TestTeamsLeaveReleasesDeletesAndDetaches(t *testing.T) {
	a, repo, out := leaveSetup(t)
	bare, clone := repo.Remote(), repo.Path()

	require.NoError(t, runTeamsLeave(leaveCmd(t), []string{"test-team"}))

	colleague := freshClone(t, bare)
	claims, err := colleague.ListClaims(context.Background(), claimsProject)
	require.NoError(t, err)
	var left []string
	for _, c := range claims {
		left = append(left, c.TicketID+":"+c.ClaimedBy)
	}
	assert.ElementsMatch(t, []string{"A-1:alice", "M-3:testuser"}, left, "the open claims are released, the others stay")
	_, err = colleague.GetMember(context.Background(), "testuser")
	require.NoError(t, err, "the card stays: it carries the history")
	var released []teamstate.Event
	events, err := colleague.ListEvents(context.Background(), claimsProject, time.Time{})
	require.NoError(t, err)
	for _, e := range events {
		if e.Type == teamstate.EventClaimReleased {
			released = append(released, e)
		}
	}
	require.Len(t, released, 2)
	for _, e := range released {
		assert.Equal(t, "testuser", e.Actor)
		assert.Equal(t, teamstate.ReleaseLeftTeam, e.Data["reason"])
	}

	_, err = os.Stat(clone)
	assert.True(t, os.IsNotExist(err), "the clone is deleted")
	secrets := a.Secrets.(*mockSecretStore).secrets
	assert.NotContains(t, secrets, "openhub.team.test-team.gitlab.token")
	assert.Contains(t, secrets, "openhub.team.other.gitlab.token", "the keys of another team stay")
	assert.Contains(t, secrets, "openhub.mcp.gitlab.token", "so do the keys that belong to no team")
	assert.Nil(t, a.Config.FindTeam("test-team"))
	onDisk, err := config.Read()
	require.NoError(t, err)
	assert.Nil(t, onDisk.FindTeam("test-team"))
	assert.Nil(t, a.Projects.(*mockProjectStore).projects[0].TeamID, "the project is detached")
	assert.Contains(t, out.String(), "2 claim(s) released")
	assert.Contains(t, out.String(), "Left team")
}

func TestTeamsLeaveRemovesTheCardOnRequest(t *testing.T) {
	_, repo, _ := leaveSetup(t)
	bare := repo.Remote()

	require.NoError(t, runTeamsLeave(leaveCmd(t, "remove-member"), []string{"test-team"}))

	colleague := freshClone(t, bare)
	_, err := colleague.GetMember(context.Background(), "testuser")
	require.ErrorIs(t, err, teamstate.ErrMemberNotFound)
	events, err := colleague.ListEvents(context.Background(), teamstate.TeamEventsProject, time.Time{})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, teamstate.EventMemberRemoved, events[0].Type)
	assert.Equal(t, true, events[0].Data["self"])
}

// The confirmation says what will be done and nothing is touched without it.
func TestTeamsLeaveAsksFirst(t *testing.T) {
	a, repo, out := leaveSetup(t)
	cmd := leaveCmd(t)
	require.NoError(t, cmd.Flags().Set("yes", "false"))

	err := runTeamsLeave(cmd, []string{"test-team"})
	require.Error(t, err)
	assert.Equal(t, ExitUsage, ExitCode(err), "without a terminal and without --yes")
	assert.Contains(t, out.String(), "release the 2 open claim(s) of testuser")
	assert.Contains(t, out.String(), "the card of testuser stays in the team (--remove-member removes it)")
	assert.Contains(t, out.String(), "delete 1 keychain key(s)")
	assert.Contains(t, out.String(), "detach 1 project(s): api")
	assert.True(t, repo.IsCloned())
	assert.NotNil(t, a.Config.FindTeam("test-team"))

	assert.Error(t, runTeamsLeave(leaveCmd(t), []string{"ghost"}))
}

// A team-state that cannot be reached: nothing is deleted, unless --force.
func TestTeamsLeaveStopsWhenTheTeamStateIsUnreachable(t *testing.T) {
	a, repo, out := leaveSetup(t)
	bare := repo.Remote()
	require.NoError(t, os.Rename(bare, bare+".away"))

	err := runTeamsLeave(leaveCmd(t), []string{"test-team"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--force")
	assert.True(t, repo.IsCloned(), "the clone stays")
	assert.NotNil(t, a.Config.FindTeam("test-team"), "so does the team")
	assert.Contains(t, a.Secrets.(*mockSecretStore).secrets, "openhub.team.test-team.gitlab.token")

	out.Reset()
	require.NoError(t, runTeamsLeave(leaveCmd(t, "force"), []string{"test-team"}))
	assert.Contains(t, out.String(), "Claims not released")
	assert.Nil(t, a.Config.FindTeam("test-team"))
	assert.False(t, repo.IsCloned())
}

// A clone that holds a commit the remote does not have is not deleted without
// --force (here the member has no claim, so nothing pushed it).
func TestTeamsLeaveKeepsACloneWithUnpushedWork(t *testing.T) {
	a, repo, _ := leaveSetup(t)
	for _, id := range []string{"M-1", "M-2"} {
		_, err := repo.ReleaseClaimAs(context.Background(), claimsProject, id, teamstate.ReleaseOpts{})
		require.NoError(t, err)
	}
	gittest.Commit(t, repo.Path(), "notes.txt", "local\n", "local work")

	err := runTeamsLeave(leaveCmd(t), []string{"test-team"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 unpushed commit")
	assert.True(t, repo.IsCloned())
	assert.NotNil(t, a.Config.FindTeam("test-team"))

	require.NoError(t, runTeamsLeave(leaveCmd(t, "force"), []string{"test-team"}))
	assert.False(t, repo.IsCloned())
}

// The folder is deleted only when it is the clone of the repository of the team.
func TestTeamsLeaveDoesNotDeleteAFolderThatIsNotItsClone(t *testing.T) {
	a, _, out := leaveSetup(t)
	_, other := gittest.BareClone(t) // the clone of another repository
	require.NoError(t, os.WriteFile(filepath.Join(other, "precious.txt"), []byte("x"), 0o644))
	a.Config.Teams[0].StatePath = other
	require.NoError(t, config.Save(a.Config))

	require.NoError(t, runTeamsLeave(leaveCmd(t), []string{"test-team"}))
	assert.FileExists(t, filepath.Join(other, "precious.txt"))
	assert.Contains(t, out.String(), "is not the clone of the repository of this team")
	assert.Nil(t, a.Config.FindTeam("test-team"))
}

// Without a clone nothing can be released: the keys and hub.toml are still cleaned.
func TestTeamsLeaveWithoutAClone(t *testing.T) {
	i18ntest.Set(t, "en")
	a, out := setupTeamsTestApp(t, []config.TeamConfig{{ID: "side", Enabled: true, StateRepo: "git@h:o/side.git", StatePath: filepath.Join(t.TempDir(), "nothing"), MemberID: "me"}}, nil)
	a.Secrets = &mockSecretStore{secrets: map[string]string{"openhub.team.side.gitlab.token": "t"}}

	require.NoError(t, runTeamsLeave(leaveCmd(t), []string{"side"}))
	assert.Contains(t, out.String(), "no clone on this machine")
	assert.Empty(t, a.Secrets.(*mockSecretStore).secrets)
	assert.Nil(t, a.Config.FindTeam("side"))
}
