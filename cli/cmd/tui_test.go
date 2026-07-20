package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
)

func TestCanLaunchTUI_NonInteractive(t *testing.T) {
	// In test environment (no TTY), canLaunchTUI should return false.
	t.Setenv("CI", "true")
	result := canLaunchTUI()
	assert.False(t, result, "canLaunchTUI should return false in CI environment")
}

func TestBuildMenuItems(t *testing.T) {
	a := &app.App{Config: &config.Config{}}
	items := buildMenuItems(a)
	assert.Greater(t, len(items), 0, "menu items should not be empty")

	// First item should be Home
	assert.Equal(t, "home", items[0].ID)
	assert.Equal(t, "home", items[0].ViewID)

	// Sessions should have children
	sessions := items[1]
	assert.Equal(t, "sessions", sessions.ID)
	assert.True(t, sessions.IsCategory())
	assert.Greater(t, len(sessions.Children), 0)
}

func TestBuildViews(t *testing.T) {
	// Create a minimal app for testing
	a := &app.App{Config: &config.Config{}}
	allViews := buildViews(a)
	assert.Greater(t, len(allViews), 0, "views should not be empty")

	// First view should be home
	assert.Equal(t, "home", allViews[0].ID())

	// Should have multiple views registered
	assert.GreaterOrEqual(t, len(allViews), 7, "should have at least 7 views")
}
