package config

import (
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boolP(b bool) *bool { return &b }
func intP(n int) *int    { return &n }

func TestTrackerFor_TeamSectionWinsOverTheDefault(t *testing.T) {
	t.Parallel()
	c := &Config{
		Tracker: TrackerLocalConfig{AutoSync: boolP(true), PushLabels: boolP(true), TrackerTokenKey: "k.default", TrackerTokenURL: "https://gl.default"},
		Teams: []TeamConfig{
			{ID: "acme", Tracker: TrackerLocalConfig{AutoSync: boolP(false), TrackerTokenKey: "k.acme"}},
			{ID: "other"},
		},
	}
	acme := c.TrackerFor("acme")
	assert.False(t, *acme.AutoSync, "the team section wins")
	assert.True(t, *acme.PushLabels, "what it leaves unset is the default's")
	assert.Equal(t, "k.acme", acme.TrackerTokenKey)
	assert.Empty(t, acme.TrackerTokenURL, "a key does not inherit the server declared next to the key of another level")

	for _, id := range []string{"other", "unknown", ""} {
		assert.Equal(t, c.Tracker, c.TrackerFor(id), id)
	}
	assert.Equal(t, TrackerLocalConfig{}, c.TeamTracker("other"))
	assert.False(t, *c.TeamTracker("acme").AutoSync)

	c.TrackerForWrite("acme").SetTokenDecl("k.new", "https://gl.new")
	assert.Equal(t, "k.new", c.Teams[0].Tracker.TrackerTokenKey)
	assert.Equal(t, "k.default", c.Tracker.TrackerTokenKey, "the default is not touched")
	c.TrackerForWrite("unknown").SetTokenDecl("k.x", "")
	assert.Equal(t, "k.x", c.Tracker.TrackerTokenKey, "no such team: the default")
}

// [teams.tracker] is written under its [[teams]] entry, read back, and an empty one is not written.
func TestTeamTracker_TomlRoundTrip(t *testing.T) {
	t.Parallel()
	c := Config{Teams: []TeamConfig{
		{ID: "acme", Enabled: true, StateRepo: "git@h:a/a.git", Tracker: TrackerLocalConfig{AutoSync: boolP(false), MaxInProgressPerMember: intP(2)}},
		{ID: "b", Enabled: true, StateRepo: "git@h:a/b.git"},
	}}
	data, err := toml.Marshal(c)
	require.NoError(t, err)
	text := string(data)
	assert.Contains(t, text, "[teams.tracker]")
	assert.Equal(t, 1, countOf(text, "[teams.tracker]"), "the team without overrides has no section")
	var back Config
	require.NoError(t, toml.Unmarshal(data, &back))
	assert.False(t, *back.Teams[0].Tracker.AutoSync)
	assert.Equal(t, 2, *back.Teams[0].Tracker.MaxInProgressPerMember)
	assert.True(t, back.Teams[1].Tracker.IsZero())

	m := c.ToMap()
	assert.Equal(t, false, m["teams.acme.tracker.auto_sync"])
	assert.EqualValues(t, 2, m["teams.acme.tracker.max_in_progress_per_member"])
	assert.NotContains(t, m, "teams.b.tracker.auto_sync")
}

func countOf(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

// Re-registering a team (rejoin, teams add) keeps the personal overrides of its section.
func TestUpsertTeam_KeepsTheTrackerSection(t *testing.T) {
	t.Parallel()
	c := &Config{Teams: []TeamConfig{{ID: "acme", Enabled: true, StateRepo: "git@h:a/a.git", Tracker: TrackerLocalConfig{AutoSync: boolP(true)}}}}
	c.UpsertTeam(TeamConfig{ID: "acme", Enabled: true, StateRepo: "git@h:a/a.git", MemberID: "ben"})
	assert.True(t, *c.Teams[0].Tracker.AutoSync)
	c.UpsertTeam(TeamConfig{ID: "acme", Enabled: true, StateRepo: "", MemberID: "ben"})
	assert.True(t, *c.Teams[0].Tracker.AutoSync)
}
