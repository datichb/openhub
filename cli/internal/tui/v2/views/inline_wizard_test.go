package views

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

func TestInlineWizardView_ImplementsView(t *testing.T) {
	var _ View = (*InlineWizardView)(nil)
}

func TestInlineWizardView_ImplementsInputCapturing(t *testing.T) {
	var _ InputCapturing = (*InlineWizardView)(nil)
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

func TestInlineWizardView_CapturesInput(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.capture",
		Title: "Capture Test",
		Steps: []WizardStep{
			{
				Label:    "Step 1",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddInputField("Name", "", 0, nil, nil)
					form.AddButton("Next", func() { onDone() })
					return form
				},
			},
		},
	})

	// Before mount: CapturesInput should return false
	assert.False(t, v.CapturesInput(), "should not capture before mount")

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()

	v.Mount(content, app)
	assert.True(t, v.CapturesInput(), "should capture while mounted")

	v.Unmount()
	assert.False(t, v.CapturesInput(), "should not capture after unmount")
}

func TestInlineWizardView_HandleKey_FormPassthrough(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.form.keys",
		Title: "Form Key Test",
		Steps: []WizardStep{
			{
				Label:    "Input Step",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddInputField("Name", "", 0, nil, nil)
					form.AddButton("Next", func() { onDone() })
					return form
				},
			},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()
	v.Mount(content, app)

	// Rune events should pass through (not consumed) so tview delivers to form
	runeEvent := tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)
	result := v.HandleKey(runeEvent)
	assert.NotNil(t, result, "rune should pass through on form step")

	// Enter should pass through
	enterEvent := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	result = v.HandleKey(enterEvent)
	assert.NotNil(t, result, "enter should pass through on form step")

	// Esc should pass through (form's CancelFunc handles it)
	escEvent := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	result = v.HandleKey(escEvent)
	assert.NotNil(t, result, "esc should pass through on form step")

	// Ctrl+B should be consumed (go back)
	ctrlBEvent := tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModNone)
	result = v.HandleKey(ctrlBEvent)
	assert.Nil(t, result, "ctrl+b should be consumed (go back)")

	v.Unmount()
}

func TestInlineWizardView_HandleKey_CustomViewEscRequired(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.cv.esc",
		Title: "CustomView Esc Test",
		Steps: []WizardStep{
			{
				Label:    "Welcome",
				Required: true,
				CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
					tv := tview.NewTextView()
					tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
						if event.Key() == tcell.KeyEnter {
							onDone()
							return nil
						}
						return event
					})
					container.AddItem(tv, 0, 1, true)
					tvApp.SetFocus(tv)
				},
			},
			{
				Label: "Second",
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddButton("Done", func() { onDone() })
					return form
				},
			},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()
	v.Mount(content, app)

	// Esc on a required CustomView step should be consumed (not skip)
	escEvent := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	result := v.HandleKey(escEvent)
	assert.Nil(t, result, "esc should be consumed on required CustomView step")
	assert.Equal(t, 0, v.currentStep, "should NOT advance past required step on Esc")

	v.Unmount()
}

func TestInlineWizardView_HandleKey_CustomViewEscOptional(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.cv.skip",
		Title: "CustomView Skip Test",
		Steps: []WizardStep{
			{
				Label:    "Optional Welcome",
				Required: false,
				CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
					tv := tview.NewTextView()
					container.AddItem(tv, 0, 1, true)
					tvApp.SetFocus(tv)
				},
			},
			{
				Label: "Second",
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddButton("Done", func() { onDone() })
					return form
				},
			},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()
	v.Mount(content, app)

	assert.Equal(t, 0, v.currentStep)

	// First Esc on an optional CustomView step should NOT skip (double-Esc required)
	escEvent := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	result := v.HandleKey(escEvent)
	assert.Nil(t, result, "esc should be consumed on optional CustomView step")
	assert.Equal(t, 0, v.currentStep, "first esc should NOT advance (double-esc required)")
	assert.True(t, v.escPending, "escPending should be set after first Esc")

	// Second Esc should skip to next step
	result = v.HandleKey(escEvent)
	assert.Nil(t, result, "second esc should be consumed")
	assert.Equal(t, 1, v.currentStep, "should advance to next step after double-esc skip")

	v.Unmount()
}

func TestInlineWizardView_SummaryTargetFunc(t *testing.T) {
	projectCreated := false

	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.target",
		Title: "Target Test",
		Steps: []WizardStep{},
		SummaryTargetViewFunc: func() string {
			if projectCreated {
				return "project.mode"
			}
			return "home"
		},
		SummaryTargetLabelFunc: func() string {
			if projectCreated {
				return "Go to project"
			}
			return "Go home"
		},
	})

	// Before project creation
	viewID, label := v.summaryTarget()
	assert.Equal(t, "home", viewID)
	assert.Equal(t, "Go home", label)

	// After project creation
	projectCreated = true
	viewID, label = v.summaryTarget()
	assert.Equal(t, "project.mode", viewID)
	assert.Equal(t, "Go to project", label)
}

// ─────────────────────────────────────────────────────────────────────────────
// N1 — isLastFocusableFormItem with trailing non-focusable TextView
// ─────────────────────────────────────────────────────────────────────────────

func TestInlineWizardView_TabToButtonFormWithTrailingTextView(t *testing.T) {
	t.Run("InputField+TextView: InputField is last focusable", func(t *testing.T) {
		form := tview.NewForm()
		form.AddInputField("Name", "", 0, nil, nil)
		form.AddTextView("hint", "some help text", 0, 1, true, false)

		// Index 0 (InputField) should be treated as last focusable
		// because the trailing TextView is non-interactive.
		assert.True(t, isLastFocusableFormItem(form, 0),
			"InputField at 0 should be last focusable when only a TextView follows")
	})

	t.Run("InputField+DropDown: InputField is NOT last focusable", func(t *testing.T) {
		form := tview.NewForm()
		form.AddInputField("Name", "", 0, nil, nil)
		form.AddDropDown("Region", []string{"us", "eu"}, 0, nil)

		// Index 0 (InputField) is NOT the last focusable because DropDown follows.
		assert.False(t, isLastFocusableFormItem(form, 0),
			"InputField at 0 should NOT be last focusable when a DropDown follows")
	})

	t.Run("nil form returns false", func(t *testing.T) {
		assert.False(t, isLastFocusableFormItem(nil, 0))
	})

	t.Run("negative index returns false", func(t *testing.T) {
		form := tview.NewForm()
		form.AddInputField("Name", "", 0, nil, nil)
		assert.False(t, isLastFocusableFormItem(form, -1))
	})

	t.Run("index out of bounds returns false", func(t *testing.T) {
		form := tview.NewForm()
		form.AddInputField("Name", "", 0, nil, nil)
		assert.False(t, isLastFocusableFormItem(form, 5))
	})

	t.Run("single InputField is last focusable", func(t *testing.T) {
		form := tview.NewForm()
		form.AddInputField("Name", "", 0, nil, nil)
		assert.True(t, isLastFocusableFormItem(form, 0),
			"sole InputField should be last focusable")
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// N2 — Double-Esc on CustomView in a 3-step wizard
// ─────────────────────────────────────────────────────────────────────────────

func TestInlineWizardView_DoubleEscCustomView(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.dblesc",
		Title: "DoubleEsc Test",
		Steps: []WizardStep{
			{
				Label:    "Welcome",
				Required: false, // optional CustomView
				CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
					tv := tview.NewTextView()
					container.AddItem(tv, 0, 1, true)
					tvApp.SetFocus(tv)
				},
			},
			{
				Label:    "Config",
				Required: false,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddInputField("Key", "", 0, nil, nil)
					form.AddButton("Next", func() { onDone() })
					return form
				},
			},
			{
				Label:    "Confirm",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddButton("Done", func() { onDone() })
					return form
				},
			},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()
	v.Mount(content, app)

	require.Equal(t, 0, v.currentStep, "should start at step 0")
	escEvent := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)

	// First Esc: sets escPending, does NOT advance
	result := v.HandleKey(escEvent)
	assert.Nil(t, result, "first esc consumed")
	assert.True(t, v.escPending, "escPending should be true after first Esc")
	assert.Equal(t, 0, v.currentStep, "still on step 0 after first Esc")

	// Non-Esc key after first Esc: resets escPending
	runeEvent := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	v.HandleKey(runeEvent)
	assert.False(t, v.escPending, "escPending should be reset by non-Esc key")
	assert.Equal(t, 0, v.currentStep, "still on step 0 after non-Esc key")

	// Re-arm double-Esc: first Esc again
	v.HandleKey(escEvent)
	assert.True(t, v.escPending, "escPending re-armed")

	// Second Esc: should skip to next step
	v.HandleKey(escEvent)
	assert.False(t, v.escPending, "escPending cleared after skip")
	assert.Equal(t, 1, v.currentStep, "should advance to step 1 after double-Esc")

	v.Unmount()
}

// ─────────────────────────────────────────────────────────────────────────────
// N3 — Ctrl+B from summary screen goes back to last step
// ─────────────────────────────────────────────────────────────────────────────

func TestInlineWizardView_SummaryCtrlB(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.summary.ctrlb",
		Title: "Summary CtrlB Test",
		Steps: []WizardStep{
			{
				Label:    "Step A",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddInputField("Name", "", 0, nil, nil)
					form.AddButton("Next", func() { onDone() })
					return form
				},
			},
			{
				Label:    "Step B",
				Required: true,
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

	require.Equal(t, 0, v.currentStep)

	// Manually advance both steps to reach summary.
	// advanceAfterDone marks the current step done and moves to next.
	step0 := v.cfg.Steps[0]
	v.advanceAfterDone(step0)
	require.Equal(t, 1, v.currentStep, "should be on step 1 after advancing step 0")
	require.False(t, v.completed, "not completed yet")

	// Render step 1 so doRenderStep is usable from handleSummaryKey
	v.doRenderStep(v.currentStep)

	step1 := v.cfg.Steps[1]
	v.advanceAfterDone(step1)
	// After advancing the last step, wizardComplete is called
	require.True(t, v.completed, "should be completed after all steps done")

	// Now simulate Ctrl+B from the summary screen
	ctrlBEvent := tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModNone)
	result := v.HandleKey(ctrlBEvent)
	assert.Nil(t, result, "ctrl+b should be consumed on summary")
	assert.False(t, v.completed, "completed should be false after ctrl+b from summary")
	assert.Equal(t, 1, v.currentStep, "should return to last real (non-hidden) step")

	v.Unmount()
}

// ─────────────────────────────────────────────────────────────────────────────
// N4 — Summary shows skipped steps with the skipped icon
// ─────────────────────────────────────────────────────────────────────────────

func TestInlineWizardView_SummaryShowsSkippedSteps(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.summary.skipped",
		Title: "Skipped Summary Test",
		Steps: []WizardStep{
			{
				Label:    "Setup",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddInputField("Name", "", 0, nil, nil)
					form.AddButton("Next", func() { onDone() })
					return form
				},
			},
			{
				Label:    "Optional Config",
				Required: false,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddCheckbox("Enable", false, nil)
					form.AddButton("Next", func() { onDone() })
					return form
				},
			},
			{
				Label:    "Finish",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddButton("Done", func() { onDone() })
					return form
				},
			},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()
	v.Mount(content, app)

	require.Equal(t, 0, v.currentStep)

	// Complete step 0 (Setup)
	v.advanceAfterDone(v.cfg.Steps[0])
	require.Equal(t, 1, v.currentStep, "should be on step 1")

	// User-skip step 1 (Optional Config) via skipCurrent → marks it StepSkipped
	v.skipCurrent()
	require.Equal(t, 2, v.currentStep, "should advance to step 2 after skip")
	assert.Equal(t, widgets.StepSkipped, v.stepStates[1].Status,
		"step 1 should be StepSkipped after user skip")

	// Render step 2 so the form is wired
	v.doRenderStep(v.currentStep)

	// Complete step 2 to trigger summary (wizardComplete → renderSummaryScreen)
	v.advanceAfterDone(v.cfg.Steps[2])
	require.True(t, v.completed, "wizard should be completed")

	// renderSummaryScreen writes the summary (including skipped steps) into
	// a TextView added to stepContent. Extract the text from the first item
	// in stepContent (the summary TextView).
	require.NotNil(t, v.stepContent, "stepContent should exist")
	require.Greater(t, v.stepContent.GetItemCount(), 0, "stepContent should have items")

	// The first item in stepContent after renderSummaryScreen is the summary TextView.
	summaryPrimitive := v.stepContent.GetItem(0)
	summaryTV, ok := summaryPrimitive.(*tview.TextView)
	require.True(t, ok, "first stepContent item should be a *tview.TextView")

	summaryText := summaryTV.GetText(false)
	assert.True(t, strings.Contains(summaryText, theme.IconSkipped),
		"summary should contain the skipped icon %q for user-skipped step, got: %s",
		theme.IconSkipped, summaryText)
}

// ─────────────────────────────────────────────────────────────────────────────
// N5 — Contextual hints per step
// ─────────────────────────────────────────────────────────────────────────────

func TestInlineWizardView_ContextualHints(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.hints",
		Title: "Hints Test",
		Steps: []WizardStep{
			{
				Label:    "Required Step",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddInputField("Name", "", 0, nil, nil)
					form.AddButton("Next", func() { onDone() })
					return form
				},
			},
			{
				Label:    "Optional Step",
				Required: false,
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

	// ── Step 0: Required, first step (no back, no skip) ──
	hints0 := v.statusHintsForStep(0)

	// Required step should NOT mention esc/skip
	assert.False(t, strings.Contains(strings.ToLower(hints0), "esc"),
		"required step hints should not contain 'esc', got: %s", hints0)
	assert.False(t, strings.Contains(strings.ToLower(hints0), "skip"),
		"required step hints should not contain 'skip', got: %s", hints0)

	// First step has no predecessor → no back hint
	assert.False(t, strings.Contains(strings.ToLower(hints0), "ctrl+b"),
		"first step hints should not contain 'ctrl+b' (no back), got: %s", hints0)

	// Should at least contain submit hint
	assert.True(t, strings.Contains(strings.ToLower(hints0), "ctrl+s"),
		"step hints should contain submit shortcut 'ctrl+s', got: %s", hints0)

	// ── Step 1: Optional, has a predecessor ──
	// First mark step 0 as done so findPrev(1) finds it
	v.stepStates[0].Status = widgets.StepDone
	hints1 := v.statusHintsForStep(1)

	// Optional step should mention skip (esc×2)
	assert.True(t, strings.Contains(hints1, "esc"),
		"optional step hints should contain 'esc' for skip, got: %s", hints1)

	// Should have back since step 0 is done
	assert.True(t, strings.Contains(strings.ToLower(hints1), "ctrl+b"),
		"second step hints should contain 'ctrl+b' (back), got: %s", hints1)

	v.Unmount()
}

// ─────────────────────────────────────────────────────────────────────────────
// Error recovery tests (P2.6)
// ─────────────────────────────────────────────────────────────────────────────

func TestInlineWizardView_ErrorState_IsSet(t *testing.T) {
	onDoneCalled := false
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.err.set",
		Title: "Error Test",
		Steps: []WizardStep{
			{
				Label:    "Step A",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					f := tview.NewForm()
					f.AddButton("Go", func() { onDone() })
					return f
				},
				OnDone: func() error {
					onDoneCalled = true
					return nil
				},
			},
		},
	})
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()
	v.Mount(content, app)

	// Simulate an error state (as runWithSpinner would set it)
	v.wizardErr = assert.AnError
	assert.NotNil(t, v.wizardErr, "wizardErr should be set")
	assert.False(t, v.completed, "wizard should NOT be completed when in error state")
	assert.False(t, onDoneCalled, "OnDone should not have been called during mount")

	v.Unmount()
}

func TestInlineWizardView_ErrorState_RetryClears(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.err.retry",
		Title: "Retry Test",
		Steps: []WizardStep{
			{
				Label:    "Step A",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					f := tview.NewForm()
					f.AddButton("Go", func() { onDone() })
					return f
				},
				OnDone: func() error {
					return nil
				},
			},
		},
	})
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()
	v.Mount(content, app)

	// Set error state and then trigger retry by clearing it (as Enter key handler does)
	v.wizardErr = assert.AnError
	assert.NotNil(t, v.wizardErr)

	// Simulate retry: clear error and re-render
	v.wizardErr = nil
	v.renderStep(v.currentStep)
	assert.Nil(t, v.wizardErr, "wizardErr should be nil after retry")
	assert.Equal(t, 0, v.currentStep, "should still be on step 0 after retry")

	v.Unmount()
}

func TestInlineWizardView_ErrorState_GoBack(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.err.back",
		Title: "GoBack Test",
		Steps: []WizardStep{
			{
				Label:    "Step A",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					f := tview.NewForm()
					f.AddButton("Go", func() { onDone() })
					return f
				},
				OnDone: func() error {
					return nil
				},
			},
			{
				Label:    "Step B",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					f := tview.NewForm()
					f.AddButton("Go", func() { onDone() })
					return f
				},
				OnDone: func() error {
					return nil
				},
			},
		},
	})
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()
	v.Mount(content, app)

	// Advance to step 1
	v.advanceAfterDone(v.cfg.Steps[0])
	require.Equal(t, 1, v.currentStep, "should be on step 1")

	// Simulate error on step 1, then go back
	v.wizardErr = assert.AnError
	v.goBack()
	assert.Equal(t, 0, v.currentStep, "should be back on step 0 after goBack")
	// Note: goBack() does not clear wizardErr by itself — the errView's
	// InputCapture handler does that before calling goBack(). Here we verify
	// navigation works even with wizardErr set.

	v.Unmount()
}
