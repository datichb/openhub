package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// oh config set teams.<id>.tracker.<key> writes the section of the team, with the typed
// setters of [tracker]; unset puts the team back on the default.
func TestConfigSetTeamTrackerKeys(t *testing.T) {
	i18ntest.Set(t, "en")
	c := &config.Config{Teams: []config.TeamConfig{{ID: "acme"}, {ID: "b"}}}
	f, ok := lookupConfigField("teams.acme.tracker.auto_sync")
	require.True(t, ok)
	require.NoError(t, f.Set(c, "true"))
	require.NotNil(t, c.Teams[0].Tracker.AutoSync)
	assert.True(t, *c.Teams[0].Tracker.AutoSync)
	assert.Nil(t, c.Tracker.AutoSync, "the default [tracker] is not touched")
	assert.True(t, c.Teams[1].Tracker.IsZero())

	f, _ = lookupConfigField("teams.acme.tracker.max_in_progress_per_member")
	require.NoError(t, f.Set(c, "3"))
	assert.Equal(t, 3, *c.Teams[0].Tracker.MaxInProgressPerMember)
	require.Error(t, f.Set(c, "-1"))
	assert.Equal(t, 3, *c.Teams[0].Tracker.MaxInProgressPerMember)

	f, _ = lookupConfigField("teams.acme.tracker.auto_sync")
	f.Unset(c)
	assert.Nil(t, c.Teams[0].Tracker.AutoSync)

	f, ok = lookupConfigField("teams.nope.tracker.auto_sync")
	require.True(t, ok)
	require.ErrorContains(t, f.Set(c, "true"), "nope")

	_, ok = lookupConfigField("teams.acme.tracker.nonsense")
	assert.False(t, ok)
	_, ok = lookupConfigField("teams.acme.name")
	assert.False(t, ok)

	// the assistants write the section of the team they configure
	e := &localEdits{team: "acme"}
	e.set("tracker.push_labels", "true")
	e.set("session.notify", "off")
	for _, op := range e.ops {
		require.NoError(t, op(c))
	}
	assert.True(t, *c.Teams[0].Tracker.PushLabels)
	assert.Nil(t, c.Tracker.PushLabels)
	assert.Equal(t, "off", c.Session.Notify, "keys that are not the tracker's are unchanged")
}
