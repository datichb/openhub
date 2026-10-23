package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
)

func TestSoloTeam_Validate(t *testing.T) {
	t.Parallel()
	solo := config.TeamConfig{ID: "solo", Enabled: true, Solo: true, StatePath: "/x/teams/solo", MemberID: "alice"}
	require.NoError(t, solo.Validate())
	noPath := solo
	noPath.StatePath = ""
	assert.Error(t, noPath.Validate())
	noRepo := solo
	noRepo.Solo = false
	assert.Error(t, noRepo.Validate(), "a regular team still needs state_repo")
}

func TestSoloTeam_NeverActive(t *testing.T) {
	t.Parallel()
	solo := config.TeamConfig{ID: "solo", Enabled: true, Solo: true, StatePath: "/s", MemberID: "alice"}
	acme := config.TeamConfig{ID: "acme", Enabled: true, StateRepo: "git@x:a.git", MemberID: "alice"}

	cfg := &config.Config{Teams: []config.TeamConfig{solo}}
	assert.Equal(t, config.TeamConfig{}, cfg.ActiveTeam())
	assert.Nil(t, cfg.DefaultTeam())

	cfg.Teams = []config.TeamConfig{solo, acme}
	assert.Equal(t, "acme", cfg.ActiveTeam().ID)
	assert.Equal(t, "acme", cfg.DefaultTeam().ID)

	disabled := acme
	disabled.Enabled = false
	cfg.Teams = []config.TeamConfig{solo, disabled}
	assert.Equal(t, "acme", cfg.ActiveTeam().ID, "first non-solo team when none is enabled")
}

func TestSoloTeam_ResolveForProject(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "solo", Enabled: true, Solo: true, StatePath: "/s", MemberID: "alice"}}}
	id := "solo"
	got := config.ResolveTeamForProject(cfg, &domain.Project{ID: "web", TeamID: &id})
	assert.False(t, got.Enabled, "team features are off in a solo space")
	assert.True(t, got.Solo)
	assert.Equal(t, "solo", got.TeamID)
	assert.Equal(t, "/s", got.StatePath)
	assert.Equal(t, "alice", got.MemberID)
}

// Saving a team (oh init, oh team init) adds it or replaces its own entry:
// the solo space and the other teams stay as they are, and the entry that is
// replaced keeps its name.
func TestUpsertTeam_NeverOverwritesAnotherTeam(t *testing.T) {
	configtest.Home(t)
	solo := config.TeamConfig{ID: "solo", Enabled: true, Solo: true, StatePath: "/s/solo", MemberID: "alice"}
	acme := config.TeamConfig{ID: "acme", Name: "ACME", StateRepo: "git@x:acme/state.git", StatePath: "/s/acme", MemberID: "alice"}
	beta := config.TeamConfig{ID: "beta", Name: "Beta Team", Enabled: true, StateRepo: "git@x:beta.git", StatePath: "/s/beta", MemberID: "alice"}
	cfg := &config.Config{Teams: []config.TeamConfig{solo, acme, beta}}
	require.Equal(t, "beta", cfg.DefaultTeam().ID, "precondition: beta is the default team, not the first entry")

	gamma := config.TeamConfig{ID: "gamma", Enabled: true, StateRepo: "git@x:gamma.git", StatePath: "/s/gamma", MemberID: "alice"}
	cfg.UpsertTeam(gamma)
	require.Len(t, cfg.Teams, 4, "a new team is added")
	assert.Equal(t, solo, cfg.Teams[0], "the solo space is untouched")
	assert.Equal(t, gamma, cfg.Teams[3], "the new team comes last")

	reconfigured := beta
	reconfigured.Name = ""
	reconfigured.MemberID = "bob"
	cfg.UpsertTeam(reconfigured)
	require.Len(t, cfg.Teams, 4, "reconfiguring a team adds no entry")
	assert.Equal(t, solo, cfg.Teams[0], "the solo space is still untouched")
	assert.Equal(t, acme, cfg.Teams[1], "acme is untouched")
	assert.Equal(t, "bob", cfg.Teams[2].MemberID, "beta is replaced in place")
	assert.Equal(t, "Beta Team", cfg.Teams[2].Name, "beta keeps its name")
	assert.True(t, cfg.Teams[2].Enabled, "beta stays enabled")
	assert.Equal(t, gamma, cfg.Teams[3], "gamma is untouched")

	byRepo := config.TeamConfig{ID: "acme-state", StateRepo: acme.StateRepo, StatePath: "/s/acme2", MemberID: "carol"}
	cfg.UpsertTeam(byRepo)
	require.Len(t, cfg.Teams, 4, "a team found by its repository is not duplicated")
	assert.Equal(t, "acme-state", cfg.Teams[1].ID, "acme is updated through its repository")
	assert.Equal(t, "ACME", cfg.Teams[1].Name, "acme keeps its name")
	assert.Equal(t, "/s/acme2", cfg.Teams[1].StatePath, "acme has the new path")
	assert.Equal(t, "carol", cfg.Teams[1].MemberID, "acme has the new member")

	check := func(t *testing.T, cfg *config.Config, when string) {
		t.Helper()
		require.Len(t, cfg.Teams, 4, "%s: four teams", when)
		assert.True(t, cfg.Teams[0].Solo, "%s: the solo space is still solo", when)
		assert.Equal(t, "/s/solo", cfg.Teams[0].StatePath, "%s: the solo space keeps its path", when)
		assert.Equal(t, "acme-state", cfg.Teams[1].ID, "%s: acme in second position", when)
		assert.Equal(t, "ACME", cfg.Teams[1].Name, "%s: acme keeps its name", when)
		assert.Equal(t, "beta", cfg.Teams[2].ID, "%s: beta in third position", when)
		assert.Equal(t, "Beta Team", cfg.Teams[2].Name, "%s: beta keeps its name", when)
		assert.Equal(t, "gamma", cfg.Teams[3].ID, "%s: gamma last", when)
		require.NotNil(t, cfg.DefaultTeam(), "%s: there is a default team", when)
		assert.Equal(t, "beta", cfg.DefaultTeam().ID, "%s: beta is still the default team", when)
		assert.Equal(t, "bob", cfg.ActiveTeam().MemberID, "%s: the active team is beta, reconfigured", when)
	}
	check(t, cfg, "in memory")

	require.NoError(t, config.Save(cfg), "saving the hub")
	config.Reset()
	loaded, err := config.Load()
	require.NoError(t, err, "loading the hub")
	check(t, loaded, "after reload")
}

// gitlab_url (the real GitLab of an aliased team-state repository) is read
// back, resolved, and kept when the team is registered again without it.
func TestTeamGitLabURL(t *testing.T) {
	cfg := &config.Config{}
	cfg.UpsertTeam(config.TeamConfig{ID: "core", Enabled: true, StateRepo: "git@alias:acme/core.git", MemberID: "alice", GitLabURL: "https://gitlab.corp.example"})
	assert.Equal(t, "https://gitlab.corp.example", cfg.ActiveTeam().GitLabURL)
	assert.Equal(t, "https://gitlab.corp.example", config.ResolveTeamConfig(cfg.ActiveTeam(), nil).GitLabURL)

	cfg.UpsertTeam(config.TeamConfig{ID: "core", Enabled: true, StateRepo: "git@alias:acme/core.git", MemberID: "alice"})
	assert.Equal(t, "https://gitlab.corp.example", cfg.FindTeam("core").GitLabURL, "the same repository keeps its declared GitLab")

	cfg.UpsertTeam(config.TeamConfig{ID: "core", Enabled: true, StateRepo: "git@other:acme/core.git", MemberID: "alice"})
	assert.Empty(t, cfg.FindTeam("core").GitLabURL, "another repository: the old declaration no longer applies")
}
