package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandRegistry_SearchEmpty(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "start", Label: "Start", Category: "Sessions"},
		{ID: "board", Label: "Board", Category: "Projets"},
		{ID: "config", Label: "Config", Category: "Configuration"},
	})

	// Empty query returns all
	results := r.Search("")
	assert.Equal(t, 3, len(results))
}

func TestCommandRegistry_SearchExactID(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "start", Label: "Start", Category: "Sessions"},
		{ID: "board", Label: "Board", Category: "Projets"},
	})

	results := r.Search("start")
	assert.Greater(t, len(results), 0)
	assert.Equal(t, "start", results[0].ID)
}

func TestCommandRegistry_SearchByAlias(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "start", Label: "Start", Aliases: []string{"session", "code"}, Category: "Sessions"},
		{ID: "board", Label: "Board", Aliases: []string{"kanban"}, Category: "Projets"},
	})

	results := r.Search("kanban")
	assert.Greater(t, len(results), 0)
	assert.Equal(t, "board", results[0].ID)

	results = r.Search("session")
	assert.Greater(t, len(results), 0)
	assert.Equal(t, "start", results[0].ID)
}

func TestCommandRegistry_SearchFuzzy(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "audit.security", Label: "Audit Sécurité", Category: "Sessions"},
		{ID: "audit.performance", Label: "Audit Performance", Category: "Sessions"},
	})

	results := r.Search("secu")
	assert.Greater(t, len(results), 0)
	assert.Equal(t, "audit.security", results[0].ID)
}

func TestCommandRegistry_SearchRespectsEnabled(t *testing.T) {
	disabled := func() bool { return false }
	r := NewCommandRegistry([]Command{
		{ID: "start", Label: "Start", Category: "Sessions"},
		{ID: "hidden", Label: "Hidden", Category: "System", Enabled: disabled},
	})

	results := r.Search("")
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "start", results[0].ID)
}

func TestCommandRegistry_SearchPrefix(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "start", Label: "Start", Category: "Sessions"},
		{ID: "start.dev", Label: "Start Dev", Category: "Sessions"},
		{ID: "status", Label: "Status", Category: "Système"},
	})

	results := r.Search("sta")
	assert.Greater(t, len(results), 0)
	// "start" should rank higher than "status" (exact prefix on ID)
	assert.Equal(t, "start", results[0].ID)
}

func TestCommandRegistry_SearchCategory(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "deploy", Label: "Deploy", Category: "Projets"},
		{ID: "status", Label: "Status", Category: "Système"},
	})

	results := r.Search("projet")
	assert.Greater(t, len(results), 0)
	assert.Equal(t, "deploy", results[0].ID)
}

func TestFuzzyMatch(t *testing.T) {
	assert.True(t, fuzzyMatch("str", "start"))
	assert.True(t, fuzzyMatch("brd", "board"))
	assert.False(t, fuzzyMatch("xyz", "board"))
	assert.True(t, fuzzyMatch("", "anything"))
	assert.False(t, fuzzyMatch("long", "sh"))
}

func TestCommand_IsEnabled(t *testing.T) {
	// nil Enabled = always true
	cmd := Command{ID: "test"}
	assert.True(t, cmd.IsEnabled())

	// Explicit false
	cmd.Enabled = func() bool { return false }
	assert.False(t, cmd.IsEnabled())

	// Explicit true
	cmd.Enabled = func() bool { return true }
	assert.True(t, cmd.IsEnabled())
}
