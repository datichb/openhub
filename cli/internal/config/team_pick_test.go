package config_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

func twoTeams() *config.Config {
	return &config.Config{Teams: []config.TeamConfig{
		{ID: "a", Enabled: true, StateRepo: "git@x:a/team.git", StatePath: "/s/a", MemberID: "alice"},
		{ID: "b", Enabled: true, StateRepo: "git@x:b/team.git", StatePath: "/s/b", MemberID: "bob"},
	}}
}

func projectOf(team string) *domain.Project {
	if team == "" {
		return &domain.Project{ID: "p"}
	}
	return &domain.Project{ID: "p", TeamID: &team}
}

func TestPickTeam_Order(t *testing.T) {
	t.Parallel()
	cfg := twoTeams()
	cfg.Teams = append(cfg.Teams, config.TeamConfig{ID: "c", Enabled: true, StateRepo: "git@x:c/team.git", StatePath: "/s/c", MemberID: "carol", Default: true})

	cases := []struct {
		name string
		sel  config.TeamSelection
		want string
		from config.TeamSource
	}{
		{"flag beats everything", config.TeamSelection{Flag: "a", TUI: "b", Project: projectOf("b")}, "a", config.TeamFromFlag},
		{"tui beats the project", config.TeamSelection{TUI: "b", Project: projectOf("a")}, "b", config.TeamFromTUI},
		{"project beats the default", config.TeamSelection{Project: projectOf("b")}, "b", config.TeamFromProject},
		{"default when nothing points", config.TeamSelection{}, "c", config.TeamFromDefault},
		{"default for a project without team", config.TeamSelection{Project: projectOf("")}, "c", config.TeamFromDefault},
		{"a stale TUI choice is ignored", config.TeamSelection{TUI: "gone"}, "c", config.TeamFromDefault},
		{"a project of an unknown team is ignored", config.TeamSelection{Project: projectOf("gone")}, "c", config.TeamFromDefault},
	}
	for _, tc := range cases {
		got, from, err := cfg.PickTeam(tc.sel)
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.want, got.ID, tc.name)
		assert.Equal(t, tc.from, from, tc.name)
	}
}

func TestPickTeam_FirstEnabledWithoutDefault(t *testing.T) {
	t.Parallel()
	cfg := twoTeams()
	cfg.Teams[0].Enabled = false // a is archived
	got, from, err := cfg.PickTeam(config.TeamSelection{})
	require.NoError(t, err)
	assert.Equal(t, "b", got.ID)
	assert.Equal(t, config.TeamFromFirst, from)
}

func TestPickTeam_NeverAnArchivedTeamByDefault(t *testing.T) {
	t.Parallel()
	cfg := twoTeams()
	cfg.Teams[0].Default = true
	cfg.Teams[0].Enabled = false // the default team was archived
	got, from, err := cfg.PickTeam(config.TeamSelection{})
	require.NoError(t, err)
	assert.Equal(t, "b", got.ID, "the archived default team is passed over")
	assert.Equal(t, config.TeamFromFirst, from)

	cfg.Teams[1].Enabled = false
	got, from, err = cfg.PickTeam(config.TeamSelection{Project: projectOf("a")})
	require.NoError(t, err)
	assert.Equal(t, config.TeamConfig{}, got, "every team archived: none applies")
	assert.Equal(t, config.TeamNone, from)
}

func TestPickTeam_FlagGivesAnyKnownTeam(t *testing.T) {
	t.Parallel()
	cfg := twoTeams()
	cfg.Teams[1].Enabled = false
	got, from, err := cfg.PickTeam(config.TeamSelection{Flag: "b"})
	require.NoError(t, err)
	assert.Equal(t, "b", got.ID, "--team names an archived team: the caller says it is archived")
	assert.Equal(t, config.TeamFromFlag, from)
	assert.False(t, got.Resolved().Enabled)

	_, _, err = cfg.PickTeam(config.TeamSelection{Flag: "zzz"})
	var unknown *config.UnknownTeamError
	require.True(t, errors.As(err, &unknown))
	assert.Equal(t, "zzz", unknown.ID)
}

func TestPickTeam_ProjectOfAnArchivedOrSoloTeamFallsBack(t *testing.T) {
	t.Parallel()
	cfg := twoTeams()
	cfg.Teams = append(cfg.Teams, config.TeamConfig{ID: "solo", Enabled: true, Solo: true, StatePath: "/s/solo", MemberID: "alice"})
	cfg.Teams[1].Enabled = false

	got, _, _ := cfg.PickTeam(config.TeamSelection{Project: projectOf("b")})
	assert.Equal(t, "a", got.ID, "the team of the project is archived")
	got, _, _ = cfg.PickTeam(config.TeamSelection{Project: projectOf("solo")})
	assert.Equal(t, "a", got.ID, "a solo space is not a team")
}

func TestPickTeam_LegacySection(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Team: config.TeamConfig{Enabled: true, StateRepo: "git@x:old.git", MemberID: "me"}}
	got, from, err := cfg.PickTeam(config.TeamSelection{})
	require.NoError(t, err)
	assert.Equal(t, "git@x:old.git", got.StateRepo)
	assert.Equal(t, config.TeamFromLegacy, from)
}

func TestSetDefaultTeam(t *testing.T) {
	t.Parallel()
	cfg := twoTeams()
	require.NoError(t, cfg.SetDefaultTeam("b"))
	assert.Equal(t, "b", cfg.DefaultTeam().ID)
	require.NoError(t, cfg.SetDefaultTeam("a"))
	assert.True(t, cfg.Teams[0].Default)
	assert.False(t, cfg.Teams[1].Default, "a single default team")
	assert.NoError(t, config.ValidateTeams(cfg.Teams))

	var unknown *config.UnknownTeamError
	require.True(t, errors.As(cfg.SetDefaultTeam("zzz"), &unknown))

	cfg.Teams[1].Enabled = false
	assert.Error(t, cfg.SetDefaultTeam("b"), "an archived team cannot be the default one")
	cfg.Teams = append(cfg.Teams, config.TeamConfig{ID: "solo", Enabled: true, Solo: true, StatePath: "/s", MemberID: "alice"})
	assert.Error(t, cfg.SetDefaultTeam("solo"), "a solo space cannot be the default one")
	assert.True(t, cfg.Teams[0].Default, "a refused change leaves the default team as it was")

	cfg.ClearDefaultTeam("a")
	assert.False(t, cfg.Teams[0].Default)
}

func TestValidateTeams_TwoDefaults(t *testing.T) {
	t.Parallel()
	cfg := twoTeams()
	cfg.Teams[0].Default, cfg.Teams[1].Default = true, true
	assert.Error(t, config.ValidateTeams(cfg.Teams))
}
