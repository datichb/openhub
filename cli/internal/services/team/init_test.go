package team

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func reload(t *testing.T) map[string]config.TeamConfig {
	t.Helper()
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	byID := map[string]config.TeamConfig{}
	for _, tc := range cfg.Teams {
		byID[tc.ID] = tc
	}
	return byID
}

// mustInit joins the team and fails the test if Init fails or warns.
func mustInit(t *testing.T, svc *Service, p InitParams) {
	t.Helper()
	res, err := svc.Init(t.Context(), p)
	require.NoError(t, err)
	require.NoError(t, res.StructureWarning)
}

func TestSaveTeamAddsOrReplacesWithoutTouchingTheOthers(t *testing.T) {
	configtest.Home(t)
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "solo", Name: "Solo", Enabled: true, Solo: true, StatePath: "/solo", MemberID: "alice"},
		{ID: "acme", Name: "Acme", Enabled: true, StateRepo: "git@gitlab.com:acme/acme.git", StatePath: "/acme", MemberID: "alice"},
	}, Team: config.TeamConfig{StateRepo: "legacy"}}
	require.NoError(t, config.Save(cfg))
	svc := New(cfg, nil, nil)

	require.NoError(t, svc.SaveTeam("git@gitlab.com:beta/beta.git", "/beta", "bob", ""))
	teams := reload(t)
	require.Len(t, teams, 3)
	assert.True(t, teams["solo"].Solo)
	assert.Equal(t, config.TeamConfig{ID: "beta", Enabled: true, StateRepo: "git@gitlab.com:beta/beta.git", StatePath: "/beta", MemberID: "bob"}, teams["beta"])

	require.NoError(t, svc.SaveTeam("git@gitlab.com:acme/acme.git", "/acme-new", "carol", ""))
	teams = reload(t)
	require.Len(t, teams, 3, "the team of the same repo is replaced, not duplicated")
	assert.Equal(t, "Acme", teams["acme"].Name)
	assert.Equal(t, "carol", teams["acme"].MemberID)
	assert.Equal(t, "/acme-new", teams["acme"].StatePath)
	assert.Empty(t, cfg.Team.StateRepo, "the legacy [team] section is cleared")
}

func TestInitClonesRegistersTheMemberAndWritesHubToml(t *testing.T) {
	configtest.Home(t)
	bare, _ := gittest.BareClone(t)
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "solo", Enabled: true, Solo: true, StatePath: "/solo", MemberID: "alice"}}}
	require.NoError(t, config.Save(cfg))
	statePath := filepath.Join(t.TempDir(), "state")
	svc := New(cfg, nil, nil)

	params := InitParams{StateRepo: bare, StatePath: statePath, MemberID: "dave", DisplayName: "Dave", GitLabUsername: "dave.g", Role: "dev"}
	mustInit(t, svc, params)

	repo := teamstate.NewRepo(bare, statePath)
	require.True(t, repo.IsCloned())
	m, err := repo.GetMember(t.Context(), "dave")
	require.NoError(t, err)
	assert.Equal(t, "Dave", m.DisplayName)
	assert.Equal(t, "dave.g", m.GitLabUsername)
	assert.Equal(t, "semi-auto", m.DefaultMode)
	assert.Empty(t, gittest.Run(t, statePath, "status", "--porcelain", "--untracked-files=all"),
		"the layout (catalog/ included) is committed, nothing is left untracked in the clone")
	assert.Contains(t, gittest.Run(t, bare, "ls-tree", "-r", "--name-only", "main"), "catalog/agents/.gitkeep")

	teams := reload(t)
	assert.True(t, teams["solo"].Solo)
	got := teams[config.RepoNameFromRemote(bare)]
	assert.True(t, got.Enabled)
	assert.Equal(t, statePath, got.StatePath)
	assert.Equal(t, "dave", got.MemberID)

	// Joining again updates the member instead of failing.
	params.DisplayName = "David"
	mustInit(t, svc, params)
	m, err = repo.GetMember(t.Context(), "dave")
	require.NoError(t, err)
	assert.Equal(t, "David", m.DisplayName)
	assert.Len(t, reload(t), 2)
}

// Joining again (oh init → team) replaced the whole member and lost its
// tracker_username.
func TestInitKeepsTheTrackerUsernameOfAMember(t *testing.T) {
	configtest.Home(t)
	bare, _ := gittest.BareClone(t)
	cfg := &config.Config{}
	require.NoError(t, config.Save(cfg))
	statePath := filepath.Join(t.TempDir(), "state")
	svc := New(cfg, nil, nil)
	params := InitParams{StateRepo: bare, StatePath: statePath, MemberID: "ben", DisplayName: "Ben", GitLabUsername: "ben1", Role: "dev",
		TrackerUsername: "@ben"}
	mustInit(t, svc, params)
	repo := teamstate.NewRepo(bare, statePath)
	m, err := repo.GetMember(t.Context(), "ben")
	require.NoError(t, err)
	assert.Equal(t, "ben", m.TrackerUsername, "set at the first join, without the @")

	params.TrackerUsername = ""
	params.DisplayName = "Benjamin"
	mustInit(t, svc, params)
	m, err = repo.GetMember(t.Context(), "ben")
	require.NoError(t, err)
	assert.Equal(t, "Benjamin", m.DisplayName)
	assert.Equal(t, "ben", m.TrackerUsername, "joining again without a value keeps it")
}

func TestInitFailureNamesTheStep(t *testing.T) {
	configtest.Home(t)
	cfg := &config.Config{}
	svc := New(cfg, nil, nil)

	_, err := svc.Init(t.Context(), InitParams{StateRepo: filepath.Join(t.TempDir(), "no-such-remote"), StatePath: filepath.Join(t.TempDir(), "state"), MemberID: "dave"})
	var ie *InitError
	require.ErrorAs(t, err, &ie)
	assert.Equal(t, StepClone, ie.Step)
	assert.Empty(t, cfg.Teams, "hub.toml is not touched when the clone fails")
}

// The GitLab of an aliased repository is written with the team, and kept when
// the same repository is joined again without it.
func TestSaveTeamKeepsTheGitLabOfTheRepository(t *testing.T) {
	configtest.Home(t)
	cfg := &config.Config{}
	svc := New(cfg, nil, nil)

	require.NoError(t, svc.SaveTeam("git@mon-alias:acme/core.git", "/core", "alice", "https://gitlab.corp.example"))
	assert.Equal(t, "https://gitlab.corp.example", cfg.FindTeam("core").GitLabURL)

	require.NoError(t, svc.SaveTeam("git@mon-alias:acme/core.git", "/core", "alice", ""))
	assert.Equal(t, "https://gitlab.corp.example", cfg.FindTeam("core").GitLabURL)
}

// The commit of the structure of the team-state fails (here the remote
// refuses it): Init does not fail, it tells so in its result, and the member
// and hub.toml are written all the same. The failure is no longer only in a log.
func TestInitStructureCommitFailureIsAWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the remote hook is a shell script")
	}
	if testing.Short() {
		t.Skip("the push is retried four times: about 8 s")
	}
	configtest.Home(t)
	bare, _ := gittest.BareClone(t)
	hook := "#!/bin/sh\nwhile read old new ref; do\n  if [ \"$(git log -1 --format=%s \"$new\")\" = \"team: init structure\" ]; then\n    echo \"structure refused\" >&2\n    exit 1\n  fi\ndone\n"
	require.NoError(t, os.WriteFile(filepath.Join(bare, "hooks", "pre-receive"), []byte(hook), 0o755))
	cfg := &config.Config{}
	require.NoError(t, config.Save(cfg))
	statePath := filepath.Join(t.TempDir(), "state")
	svc := New(cfg, nil, nil)

	res, err := svc.Init(t.Context(), InitParams{StateRepo: bare, StatePath: statePath, MemberID: "dave", DisplayName: "Dave", Role: "dev"})
	require.NoError(t, err, "a structure that cannot be committed does not stop the join")
	require.Error(t, res.StructureWarning)
	assert.True(t, teamstate.IsStructureWarning(res.StructureWarning))

	repo := teamstate.NewRepo(bare, statePath)
	assert.True(t, repo.HasMember(t.Context(), "dave"), "the member is registered")
	assert.FileExists(t, filepath.Join(statePath, "catalog", "agents", ".gitkeep"), "the folders exist locally")
	assert.Contains(t, reload(t), config.RepoNameFromRemote(bare), "hub.toml is written")
}

// One path for the clone of a team-state: the one Rejoindre uses
// (~/.oh/team-states/<host>/<name>), not ~/.oh/team-state; a team configured
// before keeps its own folder.
func TestInitClonesUnderTheTeamStatesFolder(t *testing.T) {
	configtest.Home(t)
	bare, _ := gittest.BareClone(t)
	cfg := &config.Config{}
	require.NoError(t, config.Save(cfg))
	svc := New(cfg, nil, nil)

	mustInit(t, svc, InitParams{StateRepo: bare, MemberID: "dave", DisplayName: "Dave", Role: "dev"})

	want := config.TeamStatePath(bare)
	assert.Contains(t, want, "team-states", "the folder Rejoindre and the tracker setup use")
	assert.True(t, teamstate.NewRepo(bare, want).IsCloned(), "cloned there")
	assert.NoDirExists(t, config.DefaultTeamStatePath(), "not in the former ~/.oh/team-state")
	assert.Equal(t, want, reload(t)[config.RepoNameFromRemote(bare)].StatePath, "hub.toml points there")
}

func TestStatePathForKeepsTheFolderOfAConfiguredTeam(t *testing.T) {
	configtest.Home(t)
	old := config.DefaultTeamStatePath()
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "acme", Enabled: true, StateRepo: "https://gitlab.example.com/acme/state.git", StatePath: old},
		{ID: "solo", Enabled: true, Solo: true, StatePath: "/solo"},
	}}
	svc := New(cfg, nil, nil)

	assert.Equal(t, old, svc.StatePathFor("https://gitlab.example.com/acme/state"), "the team of that repository, written another way")
	assert.Equal(t, old, svc.StatePathFor("git@gitlab.example.com:acme/state.git"), "over SSH")
	assert.Equal(t, config.TeamStatePath("https://gitlab.example.com/other/state.git"),
		svc.StatePathFor("https://gitlab.example.com/other/state.git"), "another repository: its own folder")
	assert.Equal(t, config.TeamStatePath("x"), New(nil, nil, nil).StatePathFor("x"), "no configuration")
}

// A folder that holds the clone of another repository is not reused.
func TestInitRefusesAFolderThatHoldsAnotherRepository(t *testing.T) {
	configtest.Home(t)
	bare, clone := gittest.BareClone(t)
	other, _ := gittest.BareClone(t)
	cfg := &config.Config{}
	require.NoError(t, config.Save(cfg))

	_, err := New(cfg, nil, nil).Init(t.Context(), InitParams{StateRepo: other, StatePath: clone, MemberID: "dave", Role: "dev"})

	var initErr *InitError
	require.ErrorAs(t, err, &initErr)
	assert.Equal(t, StepClone, initErr.Step)
	var mismatch *teamstate.RemoteMismatchError
	require.ErrorAs(t, err, &mismatch)
	assert.Equal(t, bare, mismatch.Have)
	assert.Empty(t, reload(t), "nothing is written to hub.toml")
	assert.False(t, teamstate.NewRepo(bare, clone).HasMember(t.Context(), "dave"), "no member was added to the clone of the other repository")
}
