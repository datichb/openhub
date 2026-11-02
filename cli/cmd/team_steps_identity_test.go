package cmd

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The Identity step of the team wizards changes the form fields of an
// existing member and keeps the rest of the record: the mode the member chose
// was overwritten by "semi-auto" when the step wrote the whole file.
func TestTeamIdentityStepKeepsWhatTheFormDoesNotShow(t *testing.T) {
	repo, err := teamstate.InitSolo(t.Context(), filepath.Join(t.TempDir(), "state"), teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	require.NoError(t, repo.PatchMember(t.Context(), "alice", func(m *teamstate.Member) error {
		m.DefaultMode = "auto"
		m.TrackerUsername = "alice.tracker"
		m.GitLabUsername = "alice"
		return nil
	}))

	s := &teamStepState{Ctx: t.Context(), Repo: repo, HasMember: true, MemberID: "alice",
		DisplayName: "Alice B.", GitLabUsername: "alice", TrackerUsername: "alice.tracker",
		MattermostUsername: "alice-mm", Role: "lead"}
	require.NoError(t, buildTeamIdentityStep(s, teamStepOpts{}).OnDone())

	m, err := repo.GetMember("alice")
	require.NoError(t, err)
	assert.Equal(t, "auto", m.DefaultMode, "the mode chosen by the member is kept")
	assert.Equal(t, "alice.tracker", m.TrackerUsername)
	assert.Equal(t, "Alice B.", m.DisplayName)
	assert.Equal(t, "alice-mm", m.MattermostUsername)
	assert.Equal(t, "lead", m.Role)

	// The step reads the fiche on top of the repository: a field it does
	// not know is not lost either.
	s.DisplayName = "Alice C."
	require.NoError(t, buildTeamIdentityStep(s, teamStepOpts{}).OnDone())
	m, err = repo.GetMember("alice")
	require.NoError(t, err)
	assert.Equal(t, "Alice C.", m.DisplayName)
	assert.Equal(t, "auto", m.DefaultMode)

	// A new member still starts in semi-auto.
	s = &teamStepState{Ctx: t.Context(), Repo: repo, MemberID: "bob", DisplayName: "Bob", Role: "dev"}
	require.NoError(t, buildTeamIdentityStep(s, teamStepOpts{}).OnDone())
	m, err = repo.GetMember("bob")
	require.NoError(t, err)
	assert.Equal(t, "semi-auto", m.DefaultMode)
}

// One commit per change: the step used to commit twice (the write and a
// second CommitAndPush of the same file).
func TestTeamIdentityStepCommitsOnce(t *testing.T) {
	repo, err := teamstate.InitSolo(t.Context(), filepath.Join(t.TempDir(), "state"), teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	before := gittest.Run(t, repo.Path(), "rev-list", "--count", "HEAD")
	s := &teamStepState{Ctx: t.Context(), Repo: repo, HasMember: true, MemberID: "alice", DisplayName: "Alice B.", Role: "lead"}
	require.NoError(t, buildTeamIdentityStep(s, teamStepOpts{}).OnDone())
	after := gittest.Run(t, repo.Path(), "rev-list", "--count", "HEAD")
	assert.NotEqual(t, before, after)
	assert.Equal(t, "members: update alice", gittest.Run(t, repo.Path(), "log", "-1", "--format=%s"))
}
