package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func TestCommandRegistry_SearchEmpty(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "start", Label: "Start", Category: "Sessions"},
		{ID: "board", Label: "Board", Category: "Projets"},
		{ID: "config", Label: "Config", Category: "Configuration"},
	})

	// Empty query returns all
	results := r.Search("", views.ModeHub)
	assert.Equal(t, 3, len(results))
}

func TestCommandRegistry_SearchExactID(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "start", Label: "Start", Category: "Sessions"},
		{ID: "board", Label: "Board", Category: "Projets"},
	})

	results := r.Search("start", views.ModeHub)
	assert.Greater(t, len(results), 0)
	assert.Equal(t, "start", results[0].ID)
}

func TestCommandRegistry_SearchByAlias(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "start", Label: "Start", Aliases: []string{"session", "code"}, Category: "Sessions"},
		{ID: "board", Label: "Board", Aliases: []string{"kanban"}, Category: "Projets"},
	})

	results := r.Search("kanban", views.ModeHub)
	assert.Greater(t, len(results), 0)
	assert.Equal(t, "board", results[0].ID)

	results = r.Search("session", views.ModeHub)
	assert.Greater(t, len(results), 0)
	assert.Equal(t, "start", results[0].ID)
}

func TestCommandRegistry_SearchFuzzy(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "audit.security", Label: "Audit Sécurité", Category: "Sessions"},
		{ID: "audit.performance", Label: "Audit Performance", Category: "Sessions"},
	})

	results := r.Search("secu", views.ModeHub)
	assert.Greater(t, len(results), 0)
	assert.Equal(t, "audit.security", results[0].ID)
}

func TestCommandRegistry_SearchRespectsEnabled(t *testing.T) {
	disabled := func() bool { return false }
	r := NewCommandRegistry([]Command{
		{ID: "start", Label: "Start", Category: "Sessions"},
		{ID: "hidden", Label: "Hidden", Category: "System", Enabled: disabled},
	})

	results := r.Search("", views.ModeHub)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "start", results[0].ID)
}

func TestCommandRegistry_SearchPrefix(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "start", Label: "Start", Category: "Sessions"},
		{ID: "start.dev", Label: "Start Dev", Category: "Sessions"},
		{ID: "status", Label: "Status", Category: "Système"},
	})

	results := r.Search("sta", views.ModeHub)
	assert.Greater(t, len(results), 0)
	// "start" should rank higher than "status" (exact prefix on ID)
	assert.Equal(t, "start", results[0].ID)
}

func TestCommandRegistry_SearchCategory(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "deploy", Label: "Deploy", Category: "Projets"},
		{ID: "status", Label: "Status", Category: "Système"},
	})

	results := r.Search("projet", views.ModeHub)
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

func TestCommand_IsVisibleInMode(t *testing.T) {
	// nil Modes = global, visible everywhere
	cmd := Command{ID: "global"}
	assert.True(t, cmd.IsVisibleInMode(views.ModeHub))
	assert.True(t, cmd.IsVisibleInMode(views.ModeTeam))
	assert.True(t, cmd.IsVisibleInMode(views.ModeProject))

	// Specific modes
	cmd = Command{ID: "project-only", Modes: []views.Mode{views.ModeProject}}
	assert.False(t, cmd.IsVisibleInMode(views.ModeHub))
	assert.False(t, cmd.IsVisibleInMode(views.ModeTeam))
	assert.True(t, cmd.IsVisibleInMode(views.ModeProject))

	// Multi-mode
	cmd = Command{ID: "session", Modes: []views.Mode{views.ModeProject, views.ModeTeam}}
	assert.False(t, cmd.IsVisibleInMode(views.ModeHub))
	assert.True(t, cmd.IsVisibleInMode(views.ModeTeam))
	assert.True(t, cmd.IsVisibleInMode(views.ModeProject))
}

func TestCommandRegistry_SearchRespectsMode(t *testing.T) {
	r := NewCommandRegistry([]Command{
		{ID: "global", Label: "Global"},
		{ID: "team-only", Label: "Team Only", Modes: []views.Mode{views.ModeTeam}},
		{ID: "project-only", Label: "Project Only", Modes: []views.Mode{views.ModeProject}},
	})

	// Hub mode: only global
	results := r.Search("", views.ModeHub)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "global", results[0].ID)

	// Team mode: global + team-only
	results = r.Search("", views.ModeTeam)
	assert.Equal(t, 2, len(results))

	// Project mode: global + project-only
	results = r.Search("", views.ModeProject)
	assert.Equal(t, 2, len(results))
}
