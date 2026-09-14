package views

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

func TestInlineWizardView_ImplementsView(t *testing.T) {
	var _ View = (*InlineWizardView)(nil)
}

func TestInlineWizardView_EmptySteps(t *testing.T) {
	completed := false
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.test",
		Title: "Test Wizard",
		Steps: []WizardStep{},
		OnComplete: func(ok bool, err error) {
			completed = ok
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.True(t, completed, "empty wizard should complete immediately")

	v.Unmount()
	assert.Nil(t, v.app)
}

func TestInlineWizardView_AllSkippedSteps(t *testing.T) {
	completed := false
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.skip",
		Title: "Skip Wizard",
		Steps: []WizardStep{
			{Label: "Step 1", Skip: true},
			{Label: "Step 2", Skip: true},
		},
		OnComplete: func(ok bool, err error) {
			completed = ok
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.True(t, completed, "all-skipped wizard should complete immediately")

	v.Unmount()
	assert.Nil(t, v.app)
}

func TestInlineWizardView_MountWithSteps(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.mount",
		Title: "Mount Wizard",
		Steps: []WizardStep{
			{
				Label:    "First Step",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddInputField("Name", "", 0, nil, nil)
					form.AddButton("Next", func() { onDone() })
					return form
				},
			},
			{
				Label: "Second Step",
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddCheckbox("Enable", false, nil)
					form.AddButton("Done", func() { onDone() })
					return form
				},
			},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.Greater(t, content.GetItemCount(), 0)
	assert.Equal(t, "wizard.mount", v.ID())
	assert.Equal(t, "Mount Wizard", v.Title())
	assert.NotEmpty(t, v.StatusHints())

	// Check step state
	require.Len(t, v.stepStates, 2)
	assert.Equal(t, widgets.StepActive, v.stepStates[0].Status)
	assert.Equal(t, widgets.StepPending, v.stepStates[1].Status)
	assert.Equal(t, 0, v.currentStep)

	v.Unmount()
	assert.Nil(t, v.app)
}

func TestInlineWizardView_AbortOnUnmount(t *testing.T) {
	var completedWith *bool
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.abort",
		Title: "Abort Test",
		Steps: []WizardStep{
			{
				Label:    "Step 1",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddButton("Next", func() { onDone() })
					return form
				},
			},
		},
		OnComplete: func(ok bool, _ error) {
			completedWith = &ok
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	// Unmount without completing — should trigger abort callback
	v.Unmount()

	require.NotNil(t, completedWith)
	assert.False(t, *completedWith, "unmount without completion should abort")
}

func TestInlineWizardView_DynamicSkipIf(t *testing.T) {
	skipSecond := true
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.dynamic",
		Title: "Dynamic Skip",
		Steps: []WizardStep{
			{
				Label: "Step 1",
				Skip:  true, // static skip
			},
			{
				Label:  "Step 2",
				SkipIf: func() bool { return skipSecond },
			},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	// Both steps are skippable → should complete immediately
	v.Mount(content, app)
	assert.True(t, v.completed)

	v.Unmount()
}

func TestInlineWizardView_NavigationHelpers(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.nav",
		Title: "Nav Test",
		Steps: []WizardStep{
			{Label: "A"},
			{Label: "B"},
			{Label: "C", Skip: true},
			{Label: "D"},
		},
	})

	// Simulate state setup
	v.stepStates = []widgets.Step{
		{Label: "A", Status: widgets.StepDone},
		{Label: "B", Status: widgets.StepActive},
		{Label: "C", Status: widgets.StepPending},
		{Label: "D", Status: widgets.StepPending},
	}
	v.currentStep = 1

	// findNext from B should find D (C is skipped)
	next := v.findNext(1)
	assert.Equal(t, 3, next)

	// findPrev from B should find A
	prev := v.findPrev(1)
	assert.Equal(t, 0, prev)

	// findPrev from A should find nothing
	prev = v.findPrev(0)
	assert.Equal(t, -1, prev)

	// countVisibleSteps at B
	total, pos := v.countVisibleSteps(1)
	assert.Equal(t, 3, total) // A, B, D (C is skipped)
	assert.Equal(t, 2, pos)   // B is 2nd visible
}
