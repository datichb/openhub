package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func TestColorTag_FormatsCorrectly(t *testing.T) {
	// Red: RGB(255, 0, 0)
	red := tcell.NewRGBColor(255, 0, 0)
	tag := ColorTag(red)
	assert.Equal(t, "[#ff0000]", tag)
}

func TestColorTag_ThemeColors(t *testing.T) {
	tag := ColorTag(theme.Accent)
	// Accent is #89b4fa = Catppuccin Blue
	assert.Equal(t, "[#89b4fa]", tag)
}

func TestNewStepBar_CreatesWidget(t *testing.T) {
	steps := []Step{
		{Label: "Config", Status: StepDone},
		{Label: "Identity", Status: StepActive},
		{Label: "Notifs", Status: StepPending},
	}
	sb := NewStepBar(steps)
	require.NotNil(t, sb)
	require.NotNil(t, sb.TextView)

	text := sb.GetText(true)
	assert.Contains(t, text, "Config")
	assert.Contains(t, text, "Identity")
	assert.Contains(t, text, "Notifs")
}

func TestStepBar_UpdateStatus(t *testing.T) {
	steps := []Step{
		{Label: "One", Status: StepPending},
		{Label: "Two", Status: StepPending},
	}
	sb := NewStepBar(steps)

	sb.UpdateStatus(0, StepDone)
	text := sb.GetText(true)
	// The done icon (●) should appear for "One"
	assert.Contains(t, text, theme.IconDone)
}

func TestStepBar_SetSteps_Replaces(t *testing.T) {
	steps := []Step{
		{Label: "Old", Status: StepPending},
	}
	sb := NewStepBar(steps)

	newSteps := []Step{
		{Label: "New1", Status: StepActive},
		{Label: "New2", Status: StepPending},
	}
	sb.SetSteps(newSteps)

	text := sb.GetText(true)
	assert.Contains(t, text, "New1")
	assert.Contains(t, text, "New2")
	assert.NotContains(t, text, "Old")
}

func TestNewStatusBar_CreatesWidget(t *testing.T) {
	sb := NewStatusBar("q quit · h help")
	require.NotNil(t, sb)
	text := sb.GetText(true)
	assert.Contains(t, text, "q quit")
	assert.Contains(t, text, "h help")
}

func TestStatusBar_SetHints_Updates(t *testing.T) {
	sb := NewStatusBar("initial")
	sb.SetHints("updated hints")
	text := sb.GetText(true)
	assert.Contains(t, text, "updated hints")
	assert.NotContains(t, text, "initial")
}
