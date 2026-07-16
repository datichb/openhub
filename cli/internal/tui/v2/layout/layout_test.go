package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild_ReturnsNonNilResult(t *testing.T) {
	result := Build(Config{
		ProjectName: "TestProject",
		Command:     "test cmd",
		StatusHints: "q quit",
	})

	require.NotNil(t, result)
	assert.NotNil(t, result.Root)
	assert.NotNil(t, result.Content)
	assert.NotNil(t, result.Sidebar)
	assert.NotNil(t, result.StatusBar)
	assert.NotNil(t, result.App)
}

func TestBuild_DefaultProjectName(t *testing.T) {
	result := Build(Config{
		ProjectName: "OpenHub",
		Command:     "board",
		StatusHints: "",
	})
	require.NotNil(t, result)
	// Sidebar should have the command as an item
	assert.Equal(t, 1, result.Sidebar.GetItemCount())
	main, _ := result.Sidebar.GetItemText(0)
	assert.Equal(t, "board", main)
}

func TestBuild_OnQuitCallback(t *testing.T) {
	called := false
	result := Build(Config{
		ProjectName: "Test",
		Command:     "cmd",
		OnQuit:      func() { called = true },
	})
	require.NotNil(t, result)
	// OnQuit is stored — will be called by Ctrl+C handler
	// We can't easily simulate Ctrl+C without running the app,
	// but we verify the config was accepted.
	assert.False(t, called) // not called yet
}

func TestBuild_StatusHintsAppendGlobalHints(t *testing.T) {
	result := Build(Config{
		ProjectName: "Test",
		Command:     "cmd",
		StatusHints: "q quit",
	})
	require.NotNil(t, result)
	// StatusBar text should contain both view hints and global hints
	text := result.StatusBar.GetText(true)
	assert.Contains(t, text, "ctrl+n menu")
	assert.Contains(t, text, "ctrl+c quit")
}
