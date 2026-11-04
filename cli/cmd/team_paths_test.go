package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// teamStateBareAt makes the bare repository dir with a team-state in it: one
// member, "testuser". Two of them at <x>/acme/team-state.git and
// <y>/beta/team-state.git are two teams whose repositories have the same name.
func teamStateBareAt(t *testing.T, dir string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	gittest.Run(t, dir, "init", "-q", "--bare", "-b", "main")
	work := filepath.Join(t.TempDir(), "work")
	gittest.Run(t, filepath.Dir(work), "init", "-q", "-b", "main", work)
	gittest.Run(t, work, "remote", "add", "origin", dir)
	gittest.Commit(t, work, "members.toml", "[members.testuser]\ndisplay_name = \"Test User\"\nrole = \"dev\"\n", "members")
	gittest.Run(t, work, "push", "-q", "-u", "origin", "main")
	return dir
}

// pathsApp is an App on a hub made of the teams given, written to hub.toml.
func pathsApp(t *testing.T, teams ...config.TeamConfig) *app.App {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	configtest.Home(t)
	io, _, _ := testIO()
	require.NoError(t, config.Save(&config.Config{Teams: teams}))
	cfg, err := config.Load()
	require.NoError(t, err)
	a := &app.App{Config: cfg, Projects: &mockProjectStore{}, IO: io}
	application = a
	t.Cleanup(func() { application = nil })
	return a
}

func TestRejoin_TwoTeamsWhoseRepositoriesAreBothCalledTeamState(t *testing.T) {
	a := pathsApp(t)
	acme := teamStateBareAt(t, filepath.Join(t.TempDir(), "acme", "team-state.git"))
	beta := teamStateBareAt(t, filepath.Join(t.TempDir(), "beta", "team-state.git"))
	ctx := context.Background()

	ra, err := teamRejoinCore(ctx, a, teamRejoinParams{StateRepo: acme, MemberID: "testuser"})
	require.NoError(t, err)
	rb, err := teamRejoinCore(ctx, a, teamRejoinParams{StateRepo: beta, MemberID: "testuser"})
	require.NoError(t, err, "the second team can be joined")

	require.Len(t, a.Config.Teams, 2, "the second team does not replace the first")
	assert.NotEqual(t, ra.TeamID, rb.TeamID, "two teams, two IDs")
	ta, tb := a.Config.FindTeamByRepo(acme), a.Config.FindTeamByRepo(beta)
	require.NotNil(t, ta)
	require.NotNil(t, tb)
	assert.Equal(t, ra.TeamID, ta.ID)
	assert.Equal(t, rb.TeamID, tb.ID)
	assert.NotEqual(t, ta.StatePath, tb.StatePath, "two teams, two folders")
	for _, tc := range []*config.TeamConfig{ta, tb} {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		assert.True(t, repo.IsCloned(), "%s is cloned in %s", tc.ID, tc.StatePath)
		assert.NoError(t, repo.CheckRemote(), "the folder of %s holds its own repository", tc.ID)
	}

	// The file says the same, and joining again changes nothing
	config.Reset()
	disk, err := config.Read()
	require.NoError(t, err)
	require.Len(t, disk.Teams, 2)
	again, err := teamRejoinCore(ctx, a, teamRejoinParams{StateRepo: acme, MemberID: "testuser"})
	require.NoError(t, err)
	assert.Equal(t, ra.TeamID, again.TeamID)
	require.Len(t, a.Config.Teams, 2)
	assert.Equal(t, ta.StatePath, a.Config.FindTeamByRepo(acme).StatePath)
}

func TestRejoin_AConfiguredTeamKeepsItsFolderAndItsID(t *testing.T) {
	bare := teamStateBareAt(t, filepath.Join(t.TempDir(), "acme", "team-state.git"))
	chosen := filepath.Join(t.TempDir(), "my-clone")
	gittest.Run(t, filepath.Dir(chosen), "clone", "-q", bare, chosen)
	a := pathsApp(t, config.TeamConfig{ID: "acme", Name: "ACME", Enabled: true, StateRepo: bare, StatePath: chosen, MemberID: "old"})

	res, err := teamRejoinCore(context.Background(), a, teamRejoinParams{StateRepo: bare, MemberID: "testuser"})
	require.NoError(t, err)

	assert.Equal(t, "acme", res.TeamID, "the ID chosen with --id is not rewritten to the name of the repository")
	require.Len(t, a.Config.Teams, 1)
	assert.Equal(t, "acme", a.Config.Teams[0].ID)
	assert.Equal(t, "ACME", a.Config.Teams[0].Name)
	assert.Equal(t, chosen, a.Config.Teams[0].StatePath, "the clone of the team is used, not a second one")
	assert.Equal(t, "testuser", a.Config.Teams[0].MemberID)
	_, err = os.Stat(filepath.Join(config.HubDir(), "team-states"))
	assert.True(t, os.IsNotExist(err), "nothing was cloned under team-states")
}

func TestStatePathFor_ATeamAlreadyConfiguredAnswersWithItsFolder(t *testing.T) {
	bare := filepath.Join(t.TempDir(), "acme", "team-state.git")
	chosen := filepath.Join(t.TempDir(), "my-clone")
	a := pathsApp(t, config.TeamConfig{ID: "acme", Enabled: true, StateRepo: bare, StatePath: chosen, MemberID: "alice"})
	assert.Equal(t, chosen, teamStatePathFor(a, bare))
	assert.Equal(t, chosen, teamStatePathFor(a, bare+"/"), "whatever the spelling")
	assert.Equal(t, "acme", teamIDFor(a, bare))
	other := filepath.Join(t.TempDir(), "beta", "team-state.git")
	assert.Equal(t, config.TeamStatePath(other), teamStatePathFor(a, other))
	assert.NotEqual(t, chosen, teamStatePathFor(a, other))
}

func TestLoad_ACloneAtTheFormerPlaceIsKept(t *testing.T) {
	bare := teamStateBareAt(t, filepath.Join(t.TempDir(), "acme", "team-state.git"))
	configtest.Home(t)
	former := filepath.Join(config.HubDir(), "team-states", "local", "team-state")
	require.NoError(t, os.MkdirAll(filepath.Dir(former), 0o755))
	gittest.Run(t, filepath.Dir(former), "clone", "-q", bare, former)
	require.NoError(t, config.Save(&config.Config{Teams: []config.TeamConfig{{ID: "team-state", Enabled: true, StateRepo: bare, MemberID: "testuser"}}}))
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, former, cfg.Teams[0].StatePath, "a clone made by 5.3.2 is used, not cloned again")

	// a folder that holds another repository is never used
	other := teamStateBareAt(t, filepath.Join(t.TempDir(), "beta", "team-state.git"))
	require.NoError(t, config.Save(&config.Config{Teams: []config.TeamConfig{{ID: "beta", Enabled: true, StateRepo: other, MemberID: "testuser"}}}))
	config.Reset()
	cfg, err = config.Load()
	require.NoError(t, err)
	assert.NotEqual(t, former, cfg.Teams[0].StatePath, "the clone of acme is not the one of beta")
}

// Started from the folder of a project, a team without state_path reads and
// writes its own clone, never the repository of the project.
func TestTeamWithoutStatePath_NeverUsesTheWorkingDirectory(t *testing.T) {
	bare := teamStateBareAt(t, filepath.Join(t.TempDir(), "acme", "team-state.git"))
	a := pathsApp(t, config.TeamConfig{ID: "acme", Enabled: true, StateRepo: bare, MemberID: "testuser"})
	_, project := gittest.BareClone(t)
	t.Chdir(project)
	cwd, err := os.Getwd()
	require.NoError(t, err)
	headBefore := gittest.Run(t, project, "rev-parse", "HEAD")

	team := a.Config.FindTeam("acme")
	require.NotNil(t, team)
	require.NotEmpty(t, team.StatePath, "hub.toml is normalised when it is read")
	assert.NotEqual(t, cwd, team.StatePath)

	res := makeResolveTeamFunc(a)()
	assert.NotEmpty(t, res.StatePath, "the TUI is given the folder, not an empty path")
	assert.NotEqual(t, cwd, res.StatePath)
	assert.Equal(t, team.StatePath, res.StatePath)

	repo := teamstate.NewRepo(res.StateRepo, res.StatePath)
	require.NoError(t, repo.EnsureReady(context.Background()), "the pull of the Teams view goes to the team-state")
	assert.True(t, repo.IsCloned())
	assert.Equal(t, team.StatePath, repo.Path())

	_, err = os.Stat(filepath.Join(project, ".git", "oh.lock"))
	assert.True(t, os.IsNotExist(err), "the lock of the project is not taken")
	assert.Equal(t, headBefore, gittest.Run(t, project, "rev-parse", "HEAD"))
	assert.Empty(t, gittest.Run(t, project, "status", "--porcelain", "--untracked-files=all"))

	// A configuration that was not read from hub.toml and has no folder is
	// refused by the repository, it does not fall back to the directory.
	bad := teamstate.NewRepo(bare, "")
	assert.ErrorIs(t, bad.Pull(context.Background()), teamstate.ErrNoStatePath)
	assert.Equal(t, headBefore, gittest.Run(t, project, "rev-parse", "HEAD"))
}

func TestTeamsAdd_TheIDIsFreeAndTheFolderIsTheOneOfTheTeam(t *testing.T) {
	first := filepath.Join(t.TempDir(), "acme", "team-state.git")
	second := filepath.Join(t.TempDir(), "beta", "team-state.git")
	a := pathsApp(t, config.TeamConfig{ID: "team-state", Enabled: true, StateRepo: first, StatePath: filepath.Join(t.TempDir(), "c1"), MemberID: "alice"})

	cmd := &cobra.Command{}
	for _, f := range []string{"repo", "member-id", "id", "name", "gitlab-url"} {
		cmd.Flags().String(f, "", "")
	}
	require.NoError(t, cmd.Flags().Set("repo", second))
	require.NoError(t, cmd.Flags().Set("member-id", "bob"))
	require.NoError(t, runTeamsAdd(cmd, nil))

	require.Len(t, a.Config.Teams, 2, "the team of the other repository is kept")
	added := a.Config.FindTeamByRepo(second)
	require.NotNil(t, added)
	assert.NotEqual(t, "team-state", added.ID)
	assert.Equal(t, config.TeamStatePath(second), added.StatePath)
	assert.Equal(t, "team-state", a.Config.FindTeamByRepo(first).ID)
}
