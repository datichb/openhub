package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// hubWithSoloAndTeam writes a hub.toml with a solo space and the team acme,
// and returns the App that holds it.
func hubWithSoloAndTeam(t *testing.T, acmeRepo string) *app.App {
	t.Helper()
	configtest.Home(t)
	cfg := &config.Config{Teams: []config.TeamConfig{
		{ID: "solo", Name: "Solo", Enabled: true, Solo: true, StatePath: "/solo", MemberID: "alice"},
		{ID: "acme", Name: "Acme", Enabled: true, StateRepo: acmeRepo, StatePath: "/acme", MemberID: "alice"},
	}}
	require.NoError(t, config.Save(cfg))
	a := &app.App{Config: cfg, IO: app.DefaultIOStreams()}
	application = a
	t.Cleanup(func() { application = nil })
	return a
}

// reloadTeams reads hub.toml back from the disk.
func reloadTeams(t *testing.T) map[string]config.TeamConfig {
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

// The team paths of oh init and oh team add the team to hub.toml, or replace
// the entry of the same team, and leave the other spaces (the solo one
// included) as they are: they go through Config.UpsertTeam.
func TestTeamConfigIsUpserted(t *testing.T) {
	t.Run("writeTeamConfig adds a team next to the solo space and the others", func(t *testing.T) {
		hubWithSoloAndTeam(t, "git@gitlab.com:acme/team-state.git")

		require.NoError(t, writeTeamConfig("git@gitlab.com:beta/beta-state.git", "/beta", "bob"))

		teams := reloadTeams(t)
		require.Len(t, teams, 3, "the new team is added")
		assert.True(t, teams["solo"].Solo, "the solo space is untouched")
		assert.Equal(t, "Acme", teams["acme"].Name, "the other team is untouched")
		assert.Equal(t, config.TeamConfig{ID: "beta-state", Enabled: true, StateRepo: "git@gitlab.com:beta/beta-state.git", StatePath: "/beta", MemberID: "bob"},
			teams["beta-state"], "the new team")
	})

	t.Run("writeTeamConfig replaces the team of the same repo and keeps its name", func(t *testing.T) {
		hubWithSoloAndTeam(t, "git@gitlab.com:acme/team-state.git")

		require.NoError(t, writeTeamConfig("git@gitlab.com:acme/team-state.git", "/acme-new", "carol"))

		teams := reloadTeams(t)
		assert.Len(t, teams, 2, "the team is replaced, not duplicated: the solo space and the team")
		assert.True(t, teams["solo"].Solo, "the solo space is untouched")
		replaced := teams["team-state"]
		assert.Equal(t, "Acme", replaced.Name, "the name of the replaced entry is kept")
		assert.Equal(t, "carol", replaced.MemberID)
		assert.Equal(t, "/acme-new", replaced.StatePath)
	})

	t.Run("teamInitCore does the same after cloning the team-state", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		bare, _ := gittest.BareClone(t)
		a := hubWithSoloAndTeam(t, bare)
		statePath := filepath.Join(t.TempDir(), "state")

		require.NoError(t, teamInitCore(context.Background(), a, teamInitParams{
			StateRepo: bare, StatePath: statePath, MemberID: "dave", DisplayName: "Dave", Role: "dev",
		}))

		teams := reloadTeams(t)
		assert.True(t, teams["solo"].Solo, "the solo space is untouched")
		var found *config.TeamConfig
		for _, tc := range teams {
			if tc.StateRepo == bare {
				tc := tc
				found = &tc
			}
		}
		require.NotNil(t, found, "the team of the repo is in hub.toml")
		assert.Len(t, teams, 2, "the team of the same repo is replaced, not duplicated")
		assert.Equal(t, "Acme", found.Name, "the name of the replaced entry is kept")
		assert.Equal(t, "dave", found.MemberID)
		assert.Equal(t, statePath, found.StatePath)
	})
}
