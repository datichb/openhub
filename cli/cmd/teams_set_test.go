package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func runTeamsSetArgs(t *testing.T, args ...string) error {
	t.Helper()
	teamsSetCmd.SetContext(t.Context())
	return runTeamsSet(teamsSetCmd, args)
}

// The repository of a team moved: the team follows it, after the access and the
// history were checked; the clone, the folder and the card stay.
func TestTeamsSetRepo(t *testing.T) {
	i18ntest.Set(t, "en")
	configtest.Home(t)
	a, repo, _ := setupClaimTestApp(t)
	require.NoError(t, config.Save(a.Config))
	oldRemote, clonePath := repo.Remote(), repo.Path()

	moved := filepath.Join(t.TempDir(), "moved", "team-state.git")
	gittest.Run(t, t.TempDir(), "clone", "-q", "--bare", oldRemote, moved)

	t.Run("another team-state is refused, hub.toml is untouched", func(t *testing.T) {
		other, otherClone := claimRepos(t)
		gittest.Run(t, otherClone, "checkout", "-q", "--orphan", "unrelated")
		gittest.Commit(t, otherClone, "other.txt", "x\n", "another team")
		gittest.Run(t, otherClone, "push", "-q", "-f", "origin", "unrelated:main")
		err := runTeamsSetArgs(t, "test-team", "repo", other)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "another team")
		assert.Equal(t, oldRemote, a.Config.FindTeam("test-team").StateRepo)
		assert.Equal(t, oldRemote, gittest.Run(t, clonePath, "remote", "get-url", "origin"))
	})

	t.Run("unknown team, unknown field, solo, same", func(t *testing.T) {
		assert.Error(t, runTeamsSetArgs(t, "nope", "repo", moved))
		err := runTeamsSetArgs(t, "test-team", "color", "red")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "color")
		require.NoError(t, runTeamsSetArgs(t, "test-team", "repo", oldRemote))
		assert.Equal(t, oldRemote, a.Config.FindTeam("test-team").StateRepo)
	})

	t.Run("the same team-state moved", func(t *testing.T) {
		require.NoError(t, runTeamsSetArgs(t, "test-team", "repo", moved))
		config.Reset()
		saved, err := config.Load()
		require.NoError(t, err)
		team := saved.FindTeam("test-team")
		require.NotNil(t, team)
		assert.Equal(t, moved, team.StateRepo)
		assert.Equal(t, clonePath, team.StatePath, "the clone stays where it is")
		assert.Equal(t, "testuser", team.MemberID)
		assert.Equal(t, moved, gittest.Run(t, clonePath, "remote", "get-url", "origin"))
	})
}

// oh teams set <id> name|member-id|enabled|default.
func TestTeamsSetFields(t *testing.T) {
	i18ntest.Set(t, "en")
	configtest.Home(t)
	a, repo, out := setupClaimTestApp(t)
	a.Config.Teams = append(a.Config.Teams, config.TeamConfig{ID: "b", Enabled: true, StateRepo: "git@h:o/b.git", MemberID: "bob"})
	require.NoError(t, config.Save(a.Config))
	_ = repo

	require.NoError(t, runTeamsSetArgs(t, "test-team", "name", "Équipe Test"))
	got, err := config.Read()
	require.NoError(t, err)
	assert.Equal(t, "Équipe Test", got.FindTeam("test-team").Name)
	require.NoError(t, runTeamsSetArgs(t, "test-team", "name", ""))
	got, _ = config.Read()
	assert.Empty(t, got.FindTeam("test-team").Name)

	// default: on, then another, then off
	out.Reset()
	require.NoError(t, runTeamsSetArgs(t, "b", "default"))
	got, _ = config.Read()
	assert.True(t, got.FindTeam("b").Default)
	require.NoError(t, runTeamsSetArgs(t, "test-team", "default", "true"))
	got, _ = config.Read()
	assert.True(t, got.FindTeam("test-team").Default)
	assert.False(t, got.FindTeam("b").Default, "a single default")
	require.NoError(t, runTeamsSetArgs(t, "test-team", "default", "false"))
	got, _ = config.Read()
	assert.False(t, got.FindTeam("test-team").Default)

	// enabled false archives, true restores
	require.NoError(t, runTeamsSetArgs(t, "b", "enabled", "false"))
	got, _ = config.Read()
	assert.False(t, got.FindTeam("b").Enabled)
	require.NoError(t, runTeamsSetArgs(t, "b", "enabled", "true"))
	got, _ = config.Read()
	assert.True(t, got.FindTeam("b").Enabled)
	require.Error(t, runTeamsSetArgs(t, "b", "enabled", "perhaps"))

	// member-id: a card of the team-state, not a colleague's invention
	err = runTeamsSetArgs(t, "test-team", "member-id", "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no card")
	require.NoError(t, runTeamsSetArgs(t, "test-team", "member-id", "testuser"))
	require.Error(t, runTeamsSetArgs(t, "test-team", "member-id", "bad id"))

	// the rest: a value is required, a field is known, a team exists
	require.Error(t, runTeamsSetArgs(t, "b", "name"))
	require.Error(t, runTeamsSetArgs(t, "b", "color", "red"))
	require.Error(t, runTeamsSetArgs(t, "nope", "name", "x"))
}
