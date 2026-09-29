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

// ─────────────────────────────────────────────────────────────────────────────
// E2E full flow test (R20)
// ─────────────────────────────────────────────────────────────────────────────

// TestInlineWizardView_E2E_FullFlow exercises a complete wizard lifecycle:
// advance, skip, goBack, error recovery, and completion across 8 synthetic steps.
func TestInlineWizardView_E2E_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E wizard test in short mode")
	}

	// Mutable state captured by step closures
	var (
		step1Value    string
		step5Value    string
		step6Attempts int
		completed     bool
		completedErr  error
	)

	skipStep2 := true // dynamic skip flag

	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.e2e",
		Title: "E2E Test",
		Steps: []WizardStep{
			// ── Step 0: CustomView (intro gate) ──
			{
				ID:       "intro",
				Label:    "Welcome",
				Required: true,
				CustomView: func(_ *tview.Application, container *tview.Flex, onDone func()) {
					tv := tview.NewTextView().SetText("Welcome to the wizard!")
					container.AddItem(tv, 0, 1, false)
				},
			},

			// ── Step 1: Form (required, captures input) ──
			{
				ID:       "form_required",
				Label:    "Required Form",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddInputField("Name", "", 0, nil, func(text string) { step1Value = text })
					form.AddButton("Next", func() { onDone() })
					return form
				},
				OnDone: func() error { return nil },
				InfoFields: func() []InfoField {
					return []InfoField{{Label: "Name", Value: step1Value}}
				},
			},

			// ── Step 2: Dynamically skipped ──
			{
				ID:     "skipped_dynamic",
				Label:  "Skipped Step",
				SkipIf: func() bool { return skipStep2 },
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddButton("Should not render", func() { onDone() })
					return form
				},
			},

			// ── Step 3: Optional (will be user-skipped via skipCurrent) ──
			{
				ID:    "optional",
				Label: "Optional Step",
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddInputField("Optional", "", 0, nil, nil)
					form.AddButton("Next", func() { onDone() })
					return form
				},
			},

			// ── Step 4: Processing-only (no Form, just OnDone) ──
			{
				ID:         "processing",
				Label:      "Processing",
				Processing: "Working...",
				OnDone: func() error {
					return nil
				},
				InfoFields: func() []InfoField {
					return []InfoField{{Label: "Status", Value: "done"}}
				},
			},

			// ── Step 5: Form with validation ──
			{
				ID:       "validated",
				Label:    "Validated Step",
				Required: true,
				Validate: func() string {
					if step5Value == "" {
						return "value is required"
					}
					return ""
				},
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddInputField("Value", step5Value, 0, nil, func(text string) { step5Value = text })
					form.AddButton("Next", func() { onDone() })
					return form
				},
				OnDone: func() error { return nil },
				InfoFields: func() []InfoField {
					return []InfoField{{Label: "Value", Value: step5Value}}
				},
			},

			// ── Step 6: Form with OnDone that fails once then succeeds ──
			{
				ID:       "retry",
				Label:    "Retry Step",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddButton("Submit", func() { onDone() })
					return form
				},
				OnDone: func() error {
					step6Attempts++
					if step6Attempts == 1 {
						return assert.AnError
					}
					return nil
				},
			},

			// ── Step 7: Final step ──
			{
				ID:       "final",
				Label:    "Summary",
				Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					form := tview.NewForm()
					form.AddButton("Finish", func() { onDone() })
					return form
				},
				OnDone: func() error { return nil },
			},
		},
		OnComplete: func(ok bool, err error) {
			completed = ok
			completedErr = err
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()
	v.Mount(content, app)

	// ── Verify initial state ──
	require.Equal(t, 0, v.currentStep, "should start on step 0 (intro)")
	require.Len(t, v.stepStates, 8, "should have 8 step states")
	assert.Equal(t, widgets.StepActive, v.stepStates[0].Status)

	// ── Step 0 (CustomView): advance ──
	v.advanceAfterDone(v.cfg.Steps[0])
	assert.Equal(t, 1, v.currentStep, "should advance to step 1")
	assert.Equal(t, widgets.StepDone, v.stepStates[0].Status)

	// ── Step 1 (Form required): simulate input and advance ──
	step1Value = "Alice"
	v.advanceAfterDone(v.cfg.Steps[1])
	// findNext skips step 2 (SkipIf=true) and returns step 3
	assert.Equal(t, 3, v.currentStep, "should skip step 2 (SkipIf=true) and land on step 3")
	assert.Equal(t, widgets.StepDone, v.stepStates[1].Status)
	// Note: stepStates[2] stays StepPending (not StepSkipped) because the engine
	// only marks skipped status when doRenderStep is called, not during findNext.

	// ── Step 3 (Optional): user-skip via skipCurrent ──
	v.skipCurrent()
	assert.Equal(t, 4, v.currentStep, "should advance to step 4 after skip")
	assert.Equal(t, widgets.StepSkipped, v.stepStates[3].Status)

	// ── Step 4 (Processing-only): advance ──
	v.advanceAfterDone(v.cfg.Steps[4])
	assert.Equal(t, 5, v.currentStep, "should advance to step 5")
	assert.Equal(t, widgets.StepDone, v.stepStates[4].Status)

	// ── Step 5 (Validated): test validation failure then success ──
	step5Value = ""
	if v.cfg.Steps[5].Validate != nil {
		assert.NotEmpty(t, v.cfg.Steps[5].Validate(), "validation should fail when empty")
	}
	step5Value = "important-data"
	if v.cfg.Steps[5].Validate != nil {
		assert.Empty(t, v.cfg.Steps[5].Validate(), "validation should pass when set")
	}
	v.advanceAfterDone(v.cfg.Steps[5])
	assert.Equal(t, 6, v.currentStep, "should advance to step 6")
	assert.Equal(t, widgets.StepDone, v.stepStates[5].Status)

	// ── Step 6 (Retry): first attempt fails ──
	err := v.cfg.Steps[6].OnDone()
	assert.Error(t, err, "first attempt should fail")
	assert.Equal(t, 1, step6Attempts)
	v.wizardErr = err

	// Retry: clear error and re-run
	v.wizardErr = nil
	err = v.cfg.Steps[6].OnDone()
	assert.NoError(t, err, "second attempt should succeed")
	v.advanceAfterDone(v.cfg.Steps[6])
	assert.Equal(t, 7, v.currentStep, "should advance to step 7")
	assert.Equal(t, widgets.StepDone, v.stepStates[6].Status)

	// ── Test goBack from step 7 ──
	v.goBack()
	assert.Equal(t, 6, v.currentStep, "should go back to step 6")
	v.advanceAfterDone(v.cfg.Steps[6])
	assert.Equal(t, 7, v.currentStep)

	// ── Step 7 (Final): complete ──
	v.advanceAfterDone(v.cfg.Steps[7])
	assert.True(t, v.completed, "wizard should be completed")
	assert.True(t, completed, "OnComplete called with true")
	assert.NoError(t, completedErr)

	// ── Final step states ──
	assert.Equal(t, widgets.StepDone, v.stepStates[0].Status, "step 0: done")
	assert.Equal(t, widgets.StepDone, v.stepStates[1].Status, "step 1: done")
	// step 2: skipped via SkipIf — status depends on engine rendering
	assert.Equal(t, widgets.StepSkipped, v.stepStates[3].Status, "step 3: user-skipped")
	assert.Equal(t, widgets.StepDone, v.stepStates[4].Status, "step 4: done")
	assert.Equal(t, widgets.StepDone, v.stepStates[5].Status, "step 5: done")
	assert.Equal(t, widgets.StepDone, v.stepStates[6].Status, "step 6: done")
	assert.Equal(t, widgets.StepDone, v.stepStates[7].Status, "step 7: done")

	// ── Verify info accumulator ──
	assert.GreaterOrEqual(t, len(v.infoAccum), 3, "should have info from steps with InfoFields")

	v.Unmount()
}

// TestInlineWizardView_E2E_HandleKey exercises InputCapture key events
// (Ctrl+B for back) across multiple steps. Note: Esc handling for Form steps
// is delegated to tview's form CancelFunc and cannot be tested without
// Application.Run(); only CustomView Esc is handled by HandleKey directly.
func TestInlineWizardView_E2E_HandleKey(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.keys",
		Title: "Key Test",
		Steps: []WizardStep{
			{
				ID:       "cv_required",
				Label:    "CV Required",
				Required: true,
				CustomView: func(_ *tview.Application, container *tview.Flex, onDone func()) {
					tv := tview.NewTextView().SetText("Intro")
					container.AddItem(tv, 0, 1, false)
				},
			},
			{
				ID:    "cv_optional",
				Label: "CV Optional",
				CustomView: func(_ *tview.Application, container *tview.Flex, onDone func()) {
					tv := tview.NewTextView().SetText("Optional")
					container.AddItem(tv, 0, 1, false)
				},
			},
			{
				ID:    "form_step",
				Label: "Form Step",
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					f := tview.NewForm()
					f.AddButton("Finish", func() { onDone() })
					return f
				},
				OnDone: func() error { return nil },
			},
		},
	})

	content := tview.NewFlex().SetDirection(tview.FlexRow)
	app := tview.NewApplication()
	v.Mount(content, app)

	assert.Equal(t, 0, v.currentStep)

	escEvent := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	ctrlBEvent := tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModCtrl)

	// ── Esc on Required CustomView: NOT skipped, no escPending ──
	result := v.HandleKey(escEvent)
	assert.Nil(t, result, "Esc on required CustomView should be consumed")
	assert.Equal(t, 0, v.currentStep, "Required CustomView not skipped")
	assert.False(t, v.escPending, "Required CustomView should not set escPending")

	// Advance step 0
	v.advanceAfterDone(v.cfg.Steps[0])
	assert.Equal(t, 1, v.currentStep)

	// ── Single Esc on Optional CustomView: sets escPending ──
	v.escPending = false
	result = v.HandleKey(escEvent)
	assert.Nil(t, result, "Esc should be consumed")
	assert.True(t, v.escPending, "first Esc should set escPending on optional CustomView")
	assert.Equal(t, 1, v.currentStep, "should not skip on single Esc")

	// ── Double Esc on Optional CustomView: skips ──
	result = v.HandleKey(escEvent)
	assert.Nil(t, result)
	assert.Equal(t, 2, v.currentStep, "double Esc should skip optional CustomView")
	assert.Equal(t, widgets.StepSkipped, v.stepStates[1].Status)

	// ── Ctrl+B on Form step: goes back to step 1 ──
	result = v.HandleKey(ctrlBEvent)
	assert.Nil(t, result, "Ctrl+B should be consumed")
	// goBack finds prev non-skipped done step — step 1 was skipped, step 0 is done
	assert.Equal(t, 0, v.currentStep, "Ctrl+B should go back past skipped step")

	v.Unmount()
}

// ─────────────────────────────────────────────────────────────────────────────
// stripTviewTags
// ─────────────────────────────────────────────────────────────────────────────

func TestStripTviewTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"no tags", "hello world", "hello world"},
		{"color tag", "[#ff0000]red[-]", "red"},
		{"bold tag", "[::b]bold[::-]", "bold"},
		{"reset tag", "[-:-:-]text", "text"},
		{"mixed", "[#a6e3a1]success[-] and [#7f849c]muted[-]", "success and muted"},
		{"nested-like", "[#ff0000][::b]both[::-][-]", "both"},
		{"brackets in text", "no [tags] here", "no  here"}, // brackets without # : - get stripped too
		{"empty tag", "[]empty", "empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, stripTviewTags(tc.input))
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// isDropDownFocused
// ─────────────────────────────────────────────────────────────────────────────

func TestIsDropDownFocused_NilForm(t *testing.T) {
	assert.False(t, isDropDownFocused(nil))
}

func TestIsDropDownFocused_WithInputField(t *testing.T) {
	form := tview.NewForm()
	form.AddInputField("Name", "", 0, nil, nil)
	form.AddDropDown("Choice", []string{"a", "b"}, 0, nil)
	form.SetFocus(0) // focus on InputField
	assert.False(t, isDropDownFocused(form), "InputField focused should return false")
}

func TestIsDropDownFocused_WithDropDown(t *testing.T) {
	form := tview.NewForm()
	form.AddInputField("Name", "", 0, nil, nil)
	form.AddDropDown("Choice", []string{"a", "b"}, 0, nil)
	form.SetFocus(1) // focus on DropDown
	assert.True(t, isDropDownFocused(form), "DropDown focused should return true")
}

// ─────────────────────────────────────────────────────────────────────────────
// fixFormLabelFocus
// ─────────────────────────────────────────────────────────────────────────────

func TestFixFormLabelFocus_AppliesColors(t *testing.T) {
	form := tview.NewForm()
	form.AddInputField("Name", "", 0, nil, nil)
	form.AddInputField("Email", "", 0, nil, nil)

	// Without real tview app focus, GetFocusedItemIndex returns -1.
	// All labels should get the non-focused (secondary) style.
	fixFormLabelFocus(form)

	nameLabel := form.GetFormItem(0).GetLabel()
	emailLabel := form.GetFormItem(1).GetLabel()

	// Both should have secondary color tags (non-focused).
	assert.Contains(t, nameLabel, theme.TextSecondaryHex, "label should contain secondary color")
	assert.Contains(t, emailLabel, theme.TextSecondaryHex, "label should contain secondary color")

	// The raw text should be preserved inside the tags.
	assert.Contains(t, nameLabel, "Name", "label text should be preserved")
	assert.Contains(t, emailLabel, "Email", "label text should be preserved")
}

func TestFixFormLabelFocus_NilForm(t *testing.T) {
	// Should not panic.
	fixFormLabelFocus(nil)
}

// ─────────────────────────────────────────────────────────────────────────────
// Exported wrappers
// ─────────────────────────────────────────────────────────────────────────────

func TestFixFormDropDownStyles_Export_NilSafety(t *testing.T) {
	FixFormDropDownStyles(nil) // should not panic
}

func TestFixFormLabelFocus_Export_NilSafety(t *testing.T) {
	FixFormLabelFocus(nil) // should not panic
}

func TestIsLastFocusableFormItem_Export(t *testing.T) {
	form := tview.NewForm()
	form.AddInputField("A", "", 0, nil, nil)
	form.AddInputField("B", "", 0, nil, nil)

	assert.False(t, IsLastFocusableFormItem(form, 0), "first item is not last")
	assert.True(t, IsLastFocusableFormItem(form, 1), "second item is last")
	assert.False(t, IsLastFocusableFormItem(nil, 0), "nil form returns false")
}
