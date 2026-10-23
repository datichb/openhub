package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/config"
)

// The teams view changes the teams it added, removed or restored on the list
// hub.toml holds now, not the list it loaded.
func TestTeamsPatchTouchesOnlyTheTeamsOfTheAction(t *testing.T) {
	a := config.TeamConfig{ID: "a", Enabled: true}
	b := config.TeamConfig{ID: "b", Enabled: true}
	c := config.TeamConfig{ID: "c", Enabled: true}
	d := config.TeamConfig{ID: "d", Enabled: true}

	t.Run("add", func(t *testing.T) {
		disk := &config.Config{Teams: []config.TeamConfig{a, d}} // d was added elsewhere
		assert.NoError(t, teamsPatch([]config.TeamConfig{a}, []config.TeamConfig{a, b})(disk))
		assert.Equal(t, []config.TeamConfig{a, d, b}, disk.Teams)
	})
	t.Run("remove", func(t *testing.T) {
		disk := &config.Config{Teams: []config.TeamConfig{a, b, d}}
		assert.NoError(t, teamsPatch([]config.TeamConfig{a, b}, []config.TeamConfig{a})(disk))
		assert.Equal(t, []config.TeamConfig{a, d}, disk.Teams)
	})
	t.Run("undo of a removal puts the team back", func(t *testing.T) {
		disk := &config.Config{Teams: []config.TeamConfig{a, d}}
		assert.NoError(t, teamsPatch([]config.TeamConfig{a}, []config.TeamConfig{a, c})(disk))
		assert.Equal(t, []config.TeamConfig{a, d, c}, disk.Teams)
	})
	t.Run("a changed team takes its new value", func(t *testing.T) {
		archived := b
		archived.Enabled = false
		disk := &config.Config{Teams: []config.TeamConfig{a, b, d}}
		assert.NoError(t, teamsPatch([]config.TeamConfig{a, b}, []config.TeamConfig{a, archived})(disk))
		assert.Equal(t, []config.TeamConfig{a, archived, d}, disk.Teams)
	})
}
