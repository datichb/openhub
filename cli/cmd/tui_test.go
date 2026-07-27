package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
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

	assert.True(t, ids["start"], "should have start command")
	assert.True(t, ids["board"], "should have board command")
	assert.True(t, ids["config"], "should have config command")
	assert.True(t, ids["doctor"], "should have doctor command")
	assert.True(t, ids["quit"], "should have quit command")
	assert.True(t, ids["home"], "should have home command")
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
	assert.True(t, categories["Projets"], "should have Projets category")
	assert.True(t, categories["Configuration"], "should have Configuration category")
	assert.True(t, categories["Système"], "should have Système category")
}

func TestBuildViews(t *testing.T) {
	// Create a minimal app for testing
	a := &app.App{Config: &config.Config{}}
	allViews := buildViews(a, shell.NewNotificationStore(50))
	assert.Greater(t, len(allViews), 0, "views should not be empty")

	// First view should be home
	assert.Equal(t, "home", allViews[0].ID())

	// Should have multiple views registered
	assert.GreaterOrEqual(t, len(allViews), 7, "should have at least 7 views")
}

func TestBuildViews_IncludesNotifications(t *testing.T) {
	a := &app.App{Config: &config.Config{}}
	notifStore := shell.NewNotificationStore(50)
	allViews := buildViews(a, notifStore)

	var found bool
	for _, v := range allViews {
		if v.ID() == "notifications" {
			found = true
			break
		}
	}
	assert.True(t, found, "notifications view must be registered in buildViews")
}