package team

import (
	"path/filepath"
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
	require.NoError(t, svc.Init(t.Context(), params))

	repo := teamstate.NewRepo(bare, statePath)
	require.True(t, repo.IsCloned())
	m, err := repo.GetMember("dave")
	require.NoError(t, err)
	assert.Equal(t, "Dave", m.DisplayName)
	assert.Equal(t, "dave.g", m.GitLabUsername)
	assert.Equal(t, "semi-auto", m.DefaultMode)
	assert.Empty(t, gittest.Run(t, statePath, "status", "--porcelain", "--untracked-files=all"),
		"A5: the layout (catalog/ included) is committed, nothing is left untracked in the clone")
	assert.Contains(t, gittest.Run(t, bare, "ls-tree", "-r", "--name-only", "main"), "catalog/agents/.gitkeep")

	teams := reload(t)
	assert.True(t, teams["solo"].Solo)
	got := teams[config.RepoNameFromRemote(bare)]
	assert.True(t, got.Enabled)
	assert.Equal(t, statePath, got.StatePath)
	assert.Equal(t, "dave", got.MemberID)

	// Joining again updates the member instead of failing.
	params.DisplayName = "David"
	require.NoError(t, svc.Init(t.Context(), params))
	m, err = repo.GetMember("dave")
	require.NoError(t, err)
	assert.Equal(t, "David", m.DisplayName)
	assert.Len(t, reload(t), 2)
}

// Bug 12: joining again (oh init → team) replaced the whole member and lost its
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
	require.NoError(t, svc.Init(t.Context(), params))
	repo := teamstate.NewRepo(bare, statePath)
	m, err := repo.GetMember("ben")
	require.NoError(t, err)
	assert.Equal(t, "ben", m.TrackerUsername, "set at the first join, without the @")

	params.TrackerUsername = ""
	params.DisplayName = "Benjamin"
	require.NoError(t, svc.Init(t.Context(), params))
	m, err = repo.GetMember("ben")
	require.NoError(t, err)
	assert.Equal(t, "Benjamin", m.DisplayName)
	assert.Equal(t, "ben", m.TrackerUsername, "joining again without a value keeps it")
}

func TestInitFailureNamesTheStep(t *testing.T) {
	configtest.Home(t)
	cfg := &config.Config{}
	svc := New(cfg, nil, nil)

	err := svc.Init(t.Context(), InitParams{StateRepo: filepath.Join(t.TempDir(), "no-such-remote"), StatePath: filepath.Join(t.TempDir(), "state"), MemberID: "dave"})
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
