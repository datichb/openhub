package widgets

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func TestColorTag(t *testing.T) {
	assert.Equal(t, "[#ff0000]", ColorTag(tcell.NewRGBColor(255, 0, 0)), "RGB colour")
	assert.Equal(t, "[#f28c50]", ColorTag(theme.Accent), "theme accent")

	t.Setenv("NO_COLOR", "1")
	assert.Equal(t, "", ColorTag(tcell.NewRGBColor(1, 2, 3)), "NO_COLOR: no tag")
}

func TestStepBar(t *testing.T) {
	sb := NewStepBar([]Step{
		{Label: "Config", Status: StepDone},
		{Label: "Identity", Status: StepActive},
		{Label: "Notifs", Status: StepPending},
	})
	require.NotNil(t, sb.TextView, "the bar is a text view")
	text := sb.GetText(true)
	for _, label := range []string{"Config", "Identity", "Notifs"} {
		assert.Contains(t, text, label, "step label shown")
	}

	sb.SetSteps([]Step{{Label: "One", Status: StepPending}, {Label: "Two", Status: StepPending}})
	text = sb.GetText(true)
	assert.Contains(t, text, "One", "new steps shown")
	assert.NotContains(t, text, "Config", "old steps replaced")
	assert.NotContains(t, text, theme.IconDone, "no step done yet")

	sb.UpdateStatus(0, StepDone)
	assert.Contains(t, sb.GetText(true), theme.IconDone, "done icon after UpdateStatus")
}

func TestStatusBar(t *testing.T) {
	sb := NewStatusBar("q quit · h help")
	text := sb.GetText(true)
	assert.Contains(t, text, "q quit", "initial hints")
	assert.Contains(t, text, "h help", "initial hints")

	sb.SetHints("updated hints")
	text = sb.GetText(true)
	assert.Contains(t, text, "updated hints", "new hints")
	assert.NotContains(t, text, "q quit", "old hints replaced")
}
