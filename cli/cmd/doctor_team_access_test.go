package cmd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The doctor probes the repository of each shared team: an expired token is a
// failure that says how to renew it, a repository that does not answer a
// warning, the network is not used offline, and the teams that have no
// repository to reach (solo, archived) are left out.
func TestDoctorTeamAccess(t *testing.T) {
	i18ntest.Set(t, "en")
	io, _, _ := testIO()
	application = &app.App{IO: io, Config: &config.Config{Teams: []config.TeamConfig{
		{ID: "ok", Enabled: true, StateRepo: "https://gitlab.example.com/o/ok.git"},
		{ID: "expired", Enabled: true, StateRepo: "https://gitlab.example.com/o/expired.git"},
		{ID: "down", Enabled: true, StateRepo: "https://gitlab.example.com/o/down.git"},
		{ID: "dup", Enabled: true, StateRepo: "git@gitlab.example.com:o/ok.git"},
		{ID: "solo", Enabled: true, Solo: true},
		{ID: "old", Enabled: false, StateRepo: "https://gitlab.example.com/o/old.git"},
	}}}
	t.Cleanup(func() { application = nil })
	prevProbe, prevOffline := teamAccessProbe, doctorOffline
	t.Cleanup(func() { teamAccessProbe, doctorOffline = prevProbe, prevOffline })
	var probed []string
	teamAccessProbe = func(_ context.Context, remote string) error {
		probed = append(probed, remote)
		switch remote {
		case "https://gitlab.example.com/o/expired.git":
			return &teamstate.AuthError{Message: "refused", Remote: remote, Cause: errors.New("fatal: Authentication failed")}
		case "https://gitlab.example.com/o/down.git":
			return errors.New("git ls-remote: fatal: Could not resolve host")
		}
		return nil
	}
	doctorOffline = func() bool { return false }

	results := teamAccessDoctorChecks(t.Context())
	require.Len(t, results, 3, "one per repository that can be reached")
	byName := map[string]string{}
	for _, r := range results {
		byName[r.Name] = r.Detail
		assert.Equal(t, doctorIDTeamAccess, r.ID)
	}
	assert.Equal(t, []string{"https://gitlab.example.com/o/ok.git", "https://gitlab.example.com/o/expired.git", "https://gitlab.example.com/o/down.git"}, probed)

	assert.True(t, results[0].OK)
	assert.False(t, results[1].OK, "an expired token fails")
	assert.Contains(t, results[1].Detail, "oh team rejoin --repo https://gitlab.example.com/o/expired.git")
	assert.Contains(t, results[1].Detail, "Authentication failed")
	assert.True(t, results[2].OK)
	assert.True(t, results[2].Warn, "a repository that does not answer is a warning")

	probed = nil
	doctorOffline = func() bool { return true }
	results = teamAccessDoctorChecks(t.Context())
	require.Len(t, results, 1)
	assert.True(t, results[0].OK)
	assert.Empty(t, probed, "offline: no network")
}

func TestDoctorTeamAccessWithoutATeamSaysNothing(t *testing.T) {
	io, _, _ := testIO()
	application = &app.App{IO: io, Config: &config.Config{}}
	t.Cleanup(func() { application = nil })
	assert.Empty(t, teamAccessDoctorChecks(t.Context()))
}
