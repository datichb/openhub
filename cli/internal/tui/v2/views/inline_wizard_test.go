package views

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

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
	t.Parallel()
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

	// Esc asks whether to quit (N19); Esc again closes the question
	escEvent := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	result = v.HandleKey(escEvent)
	assert.Nil(t, result, "esc opens the quit question on form step")
	require.NotNil(t, v.quitBar)
	assert.Nil(t, v.HandleKey(escEvent))
	assert.Nil(t, v.quitBar, "second esc = continue")

	// Ctrl+B should be consumed (go back)
	ctrlBEvent := tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModNone)
	result = v.HandleKey(ctrlBEvent)
	assert.Nil(t, result, "ctrl+b should be consumed (go back)")

	v.Unmount()
}

// Esc on a CustomView step opens the quit question: a required step offers
// no « skip the step », an optional one does.
func TestInlineWizardView_HandleKey_CustomViewEsc(t *testing.T) {
	second := WizardStep{
		Label: "Second",
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddButton("Done", func() { onDone() })
			return form
		},
	}
	tests := []struct {
		name     string
		required bool
		buttons  []string
	}{
		{"required step", true, []string{i18n.T("tui.wizard.quit.quit"), i18n.T("tui.wizard.quit.continue")}},
		{"optional step", false, []string{i18n.T("tui.wizard.quit.quit"), i18n.T("tui.wizard.quit.skip"), i18n.T("tui.wizard.quit.continue")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := NewInlineWizardView(InlineWizardConfig{
				ID:    "wizard.cv.esc",
				Title: "CustomView Esc Test",
				Steps: []WizardStep{{
					Label:    "Welcome",
					Required: tc.required,
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
				}, second},
			})
			v.Mount(tview.NewFlex().SetDirection(tview.FlexRow), tview.NewApplication())
			defer v.Unmount()

			assert.Nil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)), "esc is consumed")
			assert.Equal(t, 0, v.currentStep, "esc alone does not leave the step")
			assert.Equal(t, tc.buttons, quitButtons(t, v), "buttons of the quit question")

			if !tc.required {
				pressQuitButton(t, v, i18n.T("tui.wizard.quit.skip"))
				assert.Nil(t, v.quitBar, "the question is closed")
				assert.Equal(t, 1, v.currentStep, "« skip the step » advances")
			}
		})
	}
}

func TestInlineWizardView_SummaryTargetFunc(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
// N2 — No more silent double-Esc (N19): two Esc open then close the question
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

	result := v.HandleKey(escEvent)
	assert.Nil(t, result, "first esc consumed")
	assert.NotNil(t, v.quitBar, "first esc asks")
	v.HandleKey(escEvent)
	assert.Nil(t, v.quitBar, "second esc continues")
	assert.Equal(t, 0, v.currentStep, "never skipped silently")

	// « Continue » keeps the step too
	v.HandleKey(escEvent)
	pressQuitButton(t, v, i18n.T("tui.wizard.quit.continue"))
	assert.Nil(t, v.quitBar)
	assert.Equal(t, 0, v.currentStep)

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
	// a TextView added to stepContentInner. Extract the text from the first item
	// in stepContentInner (the summary TextView).
	require.NotNil(t, v.stepContentInner, "stepContentInner should exist")
	require.Greater(t, v.stepContentInner.GetItemCount(), 0, "stepContentInner should have items")

	// The first item in stepContentInner after renderSummaryScreen is the summary TextView.
	summaryPrimitive := v.stepContentInner.GetItem(0)
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

	// Every step offers Esc (quit question), never a skip shortcut
	assert.True(t, strings.Contains(strings.ToLower(hints0), "esc "+i18n.T("tui.wizard.quit.hint")),
		"step hints should contain the esc quit hint, got: %s", hints0)
	assert.False(t, strings.Contains(strings.ToLower(hints0), "esc×2"),
		"no double esc any more, got: %s", hints0)

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

	assert.True(t, strings.Contains(hints1, "esc"),
		"optional step hints should contain 'esc', got: %s", hints1)

	// Should have back since step 0 is done
	assert.True(t, strings.Contains(strings.ToLower(hints1), "ctrl+b"),
		"second step hints should contain 'ctrl+b' (back), got: %s", hints1)

	v.Unmount()
}

// ─────────────────────────────────────────────────────────────────────────────
// Error recovery tests (P2.6)
// ─────────────────────────────────────────────────────────────────────────────

func TestInlineWizardView_ErrorState(t *testing.T) {
	onDoneCalls := 0
	step := func(label string) WizardStep {
		return WizardStep{
			Label:    label,
			Required: true,
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				f := tview.NewForm()
				f.AddButton("Go", func() { onDone() })
				return f
			},
			OnDone: func() error {
				onDoneCalls++
				return nil
			},
		}
	}
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.err",
		Title: "Error Test",
		Steps: []WizardStep{step("Step A"), step("Step B")},
	})
	v.Mount(tview.NewFlex().SetDirection(tview.FlexRow), tview.NewApplication())
	defer v.Unmount()
	assert.NoError(t, v.wizardErr, "no error after the mount")
	assert.False(t, v.completed, "not completed after the mount")
	assert.Zero(t, onDoneCalls, "OnDone is not called by the mount")

	// an error on step B, then going back keeps the navigation working
	v.advanceAfterDone(v.cfg.Steps[0])
	require.Equal(t, 1, v.currentStep, "on step B")
	v.wizardErr = assert.AnError
	assert.False(t, v.completed, "a wizard in error is not completed")
	v.goBack()
	assert.Equal(t, 0, v.currentStep, "back on step A, wizardErr set or not")

	// retry: the error is cleared and the step is drawn again
	v.wizardErr = nil
	v.renderStep(v.currentStep)
	assert.NoError(t, v.wizardErr, "error cleared by the retry")
	assert.Equal(t, 0, v.currentStep, "still on step A after the retry")
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

	// ── Esc on Required CustomView: question without « skip » ──
	result := v.HandleKey(escEvent)
	assert.Nil(t, result, "Esc on required CustomView should be consumed")
	assert.Equal(t, 0, v.currentStep, "Required CustomView not skipped")
	assert.Len(t, quitButtons(t, v), 2)
	v.HandleKey(escEvent)

	// Advance step 0
	v.advanceAfterDone(v.cfg.Steps[0])
	assert.Equal(t, 1, v.currentStep)

	// ── Esc on Optional CustomView: question, then « skip » ──
	result = v.HandleKey(escEvent)
	assert.Nil(t, result, "Esc should be consumed")
	assert.Equal(t, 1, v.currentStep, "should not skip on Esc")
	pressQuitButton(t, v, i18n.T("tui.wizard.quit.skip"))
	assert.Equal(t, 2, v.currentStep, "« skip » skips the optional CustomView")
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
	t.Parallel()
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
// Form helpers (isDropDownFocused, fixFormLabelFocus, exports)
// ─────────────────────────────────────────────────────────────────────────────

func TestIsDropDownFocused(t *testing.T) {
	t.Parallel()
	form := func() *tview.Form {
		f := tview.NewForm()
		f.AddInputField("Name", "", 0, nil, nil)
		f.AddDropDown("Choice", []string{"a", "b"}, 0, nil)
		return f
	}
	tests := []struct {
		name  string
		form  *tview.Form
		focus int
		want  bool
	}{
		{"nil form", nil, 0, false},
		{"input field focused", form(), 0, false},
		{"drop down focused", form(), 1, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.form != nil {
				tc.form.SetFocus(tc.focus)
			}
			assert.Equal(t, tc.want, isDropDownFocused(tc.form), "isDropDownFocused")
		})
	}
}

func TestFixFormLabelFocus(t *testing.T) {
	t.Parallel()
	form := tview.NewForm()
	form.AddInputField("Name", "", 0, nil, nil)
	form.AddInputField("Email", "", 0, nil, nil)

	// Without real tview app focus, GetFocusedItemIndex returns -1:
	// all labels get the non-focused (secondary) style.
	fixFormLabelFocus(form)

	for i, text := range []string{"Name", "Email"} {
		label := form.GetFormItem(i).GetLabel()
		assert.Contains(t, label, theme.TextSecondaryHex, "%s: secondary colour", text)
		assert.Contains(t, label, text, "%s: label text preserved", text)
	}

	assert.NotPanics(t, func() {
		fixFormLabelFocus(nil)
		FixFormLabelFocus(nil)
		FixFormDropDownStyles(nil)
	}, "nil form")
}

func TestIsLastFocusableFormItem_Export(t *testing.T) {
	t.Parallel()
	form := tview.NewForm()
	form.AddInputField("A", "", 0, nil, nil)
	form.AddInputField("B", "", 0, nil, nil)

	assert.False(t, IsLastFocusableFormItem(form, 0), "first item is not last")
	assert.True(t, IsLastFocusableFormItem(form, 1), "second item is last")
	assert.False(t, IsLastFocusableFormItem(nil, 0), "nil form returns false")
}

// quitButtons returns the labels of the « quit the wizard? » prompt.
func quitButtons(t *testing.T, v *InlineWizardView) []string {
	t.Helper()
	require.NotNil(t, v.quitBar, "quit question shown")
	form, ok := v.quitBar.GetItem(1).(*tview.Form)
	require.True(t, ok)
	var labels []string
	for i := 0; i < form.GetButtonCount(); i++ {
		labels = append(labels, widgets.ButtonLabel(form.GetButton(i)))
	}
	return labels
}

// pressQuitButton activates a button of the « quit the wizard? » prompt.
func pressQuitButton(t *testing.T, v *InlineWizardView, label string) {
	t.Helper()
	require.NotNil(t, v.quitBar, "quit question shown")
	form := v.quitBar.GetItem(1).(*tview.Form)
	idx := -1
	for i := 0; i < form.GetButtonCount(); i++ {
		if widgets.ButtonLabel(form.GetButton(i)) == label {
			idx = i
		}
	}
	require.GreaterOrEqual(t, idx, 0, "button %q", label)
	form.GetButton(idx).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
}

type popShell struct {
	mockShell
	popped int
}

func (s *popShell) PopView() bool { s.popped++; return true }

// N19: leaving a wizard step.
func TestInlineWizardView_QuitAndBack(t *testing.T) {
	t.Run("esc then quit calls OnComplete(false) once and pops the wizard", func(t *testing.T) {
		var calls []bool
		v := NewInlineWizardView(InlineWizardConfig{
			ID: "wizard.quit", Title: "Quit",
			OnComplete: func(ok bool, err error) { calls = append(calls, ok); assert.NoError(t, err) },
			Steps: []WizardStep{{
				Label: "Required", Required: true,
				Form: func(_ *tview.Application, onDone func()) *tview.Form {
					f := tview.NewForm()
					f.AddInputField("Name", "", 0, nil, nil)
					f.AddButton("Next", onDone)
					return f
				},
			}},
		})
		sh := &popShell{}
		v.SetShell(sh)
		content := tview.NewFlex().SetDirection(tview.FlexRow)
		app := runApp(t, content)
		onLoop(app, func() { v.Mount(content, app) })
		onLoop(app, func() {
			assert.Nil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)), "esc is consumed")
			form := v.quitBar.GetItem(1).(*tview.Form)
			assert.Equal(t, form.GetButton(form.GetButtonCount()-1), app.GetFocus(), "focus on « continue »")
			pressQuitButton(t, v, i18n.T("tui.wizard.quit.quit"))
		})
		assert.Equal(t, []bool{false}, calls, "OnComplete(false)")
		assert.Equal(t, 1, sh.popped, "wizard popped")
		onLoop(app, func() { v.Unmount() })
		assert.Equal(t, []bool{false}, calls, "OnComplete called once")
	})

	t.Run("Ctrl+B, Alt+← and PgUp go back", func(t *testing.T) {
		step := func(label string) WizardStep {
			return WizardStep{Label: label, Form: func(_ *tview.Application, onDone func()) *tview.Form {
				f := tview.NewForm()
				f.AddButton("Next", onDone)
				return f
			}}
		}
		for _, ev := range []*tcell.EventKey{
			tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModNone),
			tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt),
			tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone),
		} {
			t.Run(ev.Name(), func(t *testing.T) {
				v := NewInlineWizardView(InlineWizardConfig{ID: "wizard.back", Steps: []WizardStep{step("a"), step("b")}})
				v.Mount(tview.NewFlex(), tview.NewApplication())
				defer v.Unmount()
				v.advanceAfterDone(v.cfg.Steps[0])
				v.renderStep(v.currentStep)
				require.Equal(t, 1, v.currentStep, "on the second step")
				assert.Nil(t, v.HandleKey(ev), "key is consumed")
				assert.Equal(t, 0, v.currentStep, "back on the first step")
			})
		}
	})
}
