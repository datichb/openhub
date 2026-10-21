package layout

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func TestBuild(t *testing.T) {
	result := Build(Config{ProjectName: "TestProject", Command: "board", StatusHints: "q quit"})
	require.NotNil(t, result, "result")
	assert.NotNil(t, result.Root, "root")
	assert.NotNil(t, result.Content, "content")
	assert.NotNil(t, result.App, "application")

	require.NotNil(t, result.Sidebar, "sidebar")
	assert.Equal(t, 1, result.Sidebar.GetItemCount(), "the sidebar lists the command")
	main, _ := result.Sidebar.GetItemText(0)
	assert.Equal(t, "board", main, "sidebar item")

	require.NotNil(t, result.StatusBar, "status bar")
	text := result.StatusBar.GetText(true)
	assert.Contains(t, text, "q quit", "hints of the view")
	assert.Contains(t, text, "ctrl+n menu", "global hint")
	assert.Contains(t, text, "ctrl+c quit", "global hint")
}

func TestBuildGlobalKeys(t *testing.T) {
	quits := 0
	result := Build(Config{ProjectName: "Test", Command: "cmd", OnQuit: func() { quits++ }})
	root, ok := result.Root.(interface {
		GetInputCapture() func(*tcell.EventKey) *tcell.EventKey
	})
	require.True(t, ok, "the root exposes its input capture")
	capture := root.GetInputCapture()
	require.NotNil(t, capture, "global key handler")

	assert.Nil(t, capture(tcell.NewEventKey(tcell.KeyCtrlN, 0, tcell.ModNone)), "Ctrl+N is consumed")
	assert.Equal(t, theme.BorderFocus, result.Sidebar.GetBorderColor(), "Ctrl+N focuses the sidebar")
	assert.Nil(t, capture(tcell.NewEventKey(tcell.KeyCtrlN, 0, tcell.ModNone)), "Ctrl+N is consumed")
	assert.Equal(t, theme.BorderNormal, result.Sidebar.GetBorderColor(), "a second Ctrl+N goes back to the content")

	assert.Zero(t, quits, "OnQuit is not called before Ctrl+C")
	assert.Nil(t, capture(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone)), "Ctrl+C is consumed")
	assert.Equal(t, 1, quits, "Ctrl+C calls OnQuit")

	other := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	assert.Equal(t, other, capture(other), "other keys go on to the view")
}
