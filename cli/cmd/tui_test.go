package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

func TestCanLaunchTUI_NonInteractive(t *testing.T) {
	// In test environment (no TTY), canLaunchTUI should return false.
	t.Setenv("CI", "true")
	result := canLaunchTUI()
	assert.False(t, result, "canLaunchTUI should return false in CI environment")
}

func TestBuildCommands(t *testing.T) {
	a := &app.App{Config: &config.Config{}}
	commands := buildCommands(a)
	assert.Greater(t, len(commands), 0, "commands should not be empty")

	// Should contain core commands
	ids := make(map[string]bool)
	for _, cmd := range commands {
		ids[cmd.ID] = true
	}

	assert.True(t, ids["libre"], "should have libre command (replaces start/quick; coder is its alias)")
	assert.True(t, ids["settings"], "should have settings command")
	assert.True(t, ids["doctor"], "should have doctor command")
	assert.True(t, ids["quit"], "should have quit command")
	assert.True(t, ids["home"], "should have home command")

	// Project-scoped commands (board, deploy, sync, project-config) are conditional
	// on having at least one active project (ADR-032). With nil Projects store,
	// they should NOT be registered.
	assert.False(t, ids["board"], "board should not be registered without projects")
	assert.False(t, ids["deploy"], "deploy should not be registered without projects")

	// Team commands should not be registered without a team configured.
	assert.False(t, ids["team.board"], "team.board should not be registered without team")
	assert.False(t, ids["team-detail"], "team-detail should not be registered without team")
}

func TestBuildCommands_HasCategories(t *testing.T) {
	a := &app.App{Config: &config.Config{}}
	commands := buildCommands(a)

	categories := make(map[string]bool)
	for _, cmd := range commands {
		if cmd.Category != "" {
			categories[cmd.Category] = true
		}
	}

	assert.True(t, categories["Sessions"], "should have Sessions category")
	assert.True(t, categories[i18n.T("tui.category.hub")], "should have Hub category")
	assert.True(t, categories[i18n.T("tui.category.system")], "should have System category")
}

func TestBuildViews(t *testing.T) {
	allViews := buildViews(&app.App{Config: &config.Config{}})
	require.GreaterOrEqual(t, len(allViews), 7, "the shell registers at least 7 views")
	assert.Equal(t, "home", allViews[0].ID(), "the first view is the home")

	ids := make([]string, len(allViews))
	for i, v := range allViews {
		ids[i] = v.ID()
	}
	assert.Contains(t, ids, "notifications", "the notifications view is registered")
}
