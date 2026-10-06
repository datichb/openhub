package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

func TestSoloTeam_Validate(t *testing.T) {
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
	cfg := &config.Config{Teams: []config.TeamConfig{{ID: "solo", Enabled: true, Solo: true, StatePath: "/s", MemberID: "alice"}}}
	id := "solo"
	got := config.ResolveTeamForProject(cfg, &domain.Project{ID: "web", TeamID: &id})
	assert.False(t, got.Enabled, "team features are off in a solo space")
	assert.True(t, got.Solo)
	assert.Equal(t, "solo", got.TeamID)
	assert.Equal(t, "/s", got.StatePath)
	assert.Equal(t, "alice", got.MemberID)
}
