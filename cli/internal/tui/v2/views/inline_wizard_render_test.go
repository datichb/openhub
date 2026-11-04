package views

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// These tests pin what renderStep does for each kind of step (skip, custom
// view, form, processing-only), before and after its split into one function
// per kind.

func formStep(label string, fields ...string) WizardStep {
	return WizardStep{
		Label: label,
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			for _, f := range fields {
				form.AddInputField(f, "", 0, nil, nil)
			}
			form.AddButton("Next", func() { onDone() })
			return form
		},
	}
}

func mountWizard(t *testing.T, cfg InlineWizardConfig) *InlineWizardView {
	t.Helper()
	v := NewInlineWizardView(cfg)
	v.Mount(tview.NewFlex().SetDirection(tview.FlexRow), tview.NewApplication())
	t.Cleanup(v.Unmount)
	return v
}

// A step that asks to be skipped when it is reached is marked done and the
// next one is drawn; with no next step the wizard completes.
func TestInlineWizardRenderStepSkipsADynamicallySkippedStep(t *testing.T) {
	skip := false
	second := formStep("Second")
	second.SkipIf = func() bool { return skip }
	v := mountWizard(t, InlineWizardConfig{
		ID:    "wizard.render.skip",
		Steps: []WizardStep{formStep("First"), second, formStep("Third")},
	})

	v.advanceAfterDone(v.cfg.Steps[0])
	require.Equal(t, 1, v.currentStep)
	skip = true
	v.renderStep(v.currentStep)
	assert.Equal(t, 2, v.currentStep, "the skipped step is left behind")
	assert.Equal(t, widgets.StepDone, v.stepStates[1].Status)
	assert.Equal(t, widgets.StepActive, v.stepStates[2].Status)
	assert.False(t, v.completed)

	// the last step skipped: nothing left to draw, the wizard completes
	lastSkip := false
	last := formStep("Last")
	last.SkipIf = func() bool { return lastSkip }
	v2 := mountWizard(t, InlineWizardConfig{ID: "wizard.render.skip.last", Steps: []WizardStep{formStep("One"), last}})
	v2.advanceAfterDone(v2.cfg.Steps[0])
	lastSkip = true
	v2.renderStep(1)
	assert.True(t, v2.completed, "no step left after the skipped one")
}

// RefreshLabels runs before each render and its labels reach the step bar and
// the groups.
func TestInlineWizardRenderStepRefreshesTheLabels(t *testing.T) {
	var cfg InlineWizardConfig
	label := "Old"
	cfg = InlineWizardConfig{
		ID:     "wizard.render.labels",
		Steps:  []WizardStep{formStep("Old"), formStep("Old 2")},
		Groups: []StepGroup{{Label: "Group old", StartIdx: 0}},
		RefreshLabels: func() {
			cfg.Steps[0].Label = label
			cfg.Groups[0].Label = "Group " + label
		},
	}
	v := mountWizard(t, cfg)
	label = "New"
	v.renderStep(0)
	assert.Equal(t, "New", v.stepStates[0].Label)
	assert.Equal(t, "Group New", v.groupStates[0].Label)
}

func hintsText(v *InlineWizardView) string { return v.hintsBar.GetText(true) }

// A custom view that calls onDone with a failing Validate stays on its step
// and shows the message in the hints bar; once valid it moves on.
func TestInlineWizardRenderStepCustomViewValidation(t *testing.T) {
	problem := "pick one"
	var done func()
	step := WizardStep{
		Label:    "Custom",
		Validate: func() string { return problem },
		CustomView: func(tvApp *tview.Application, container *tview.Flex, onDone func()) {
			done = onDone
			tv := tview.NewTextView()
			container.AddItem(tv, 0, 1, true)
			tvApp.SetFocus(tv)
		},
	}
	v := mountWizard(t, InlineWizardConfig{ID: "wizard.render.custom", Steps: []WizardStep{step, formStep("After")}})
	require.NotNil(t, done)

	done()
	assert.Equal(t, 0, v.currentStep, "invalid: the step is kept")
	assert.Contains(t, hintsText(v), theme.IconWarning+" pick one")

	problem = ""
	done()
	assert.Equal(t, 1, v.currentStep, "valid: next step")
}

// A form step with a failing Validate marks its first input with the error
// icon and shows the message; the label comes back when the step is valid.
func TestInlineWizardRenderStepFormValidation(t *testing.T) {
	problem := "name required"
	step := formStep("Form", "Name")
	step.Validate = func() string { return problem }
	v := mountWizard(t, InlineWizardConfig{ID: "wizard.render.form", Steps: []WizardStep{step, formStep("After")}})
	require.NotNil(t, v.form)

	input, ok := v.form.GetFormItem(0).(*tview.InputField)
	require.True(t, ok)
	v.form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	assert.Equal(t, 0, v.currentStep, "invalid: the step is kept")
	assert.Contains(t, hintsText(v), theme.IconWarning+" name required")
	assert.Contains(t, input.GetLabel(), theme.IconError, "the first input carries the error mark")
	assert.Contains(t, input.GetLabel(), "Name")

	problem = ""
	v.form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	assert.Equal(t, 1, v.currentStep, "valid: next step")
}

// Ctrl+S on a form step goes through its first button, like a press on it.
func TestInlineWizardRenderStepCtrlSPressesTheButton(t *testing.T) {
	pressed := 0
	step := WizardStep{
		Label: "Form",
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddButton("Go", func() { pressed++; onDone() })
			return form
		},
	}
	v := mountWizard(t, InlineWizardConfig{ID: "wizard.render.ctrls", Steps: []WizardStep{step, formStep("After")}})
	v.form.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone), func(tview.Primitive) {})
	assert.Equal(t, 1, pressed)
	assert.Equal(t, 1, v.currentStep)
}

// A step with both a Form and a CustomView draws the custom view.
func TestInlineWizardRenderStepCustomViewWinsOverForm(t *testing.T) {
	formCalled := false
	step := WizardStep{
		Label: "Both",
		Form: func(_ *tview.Application, _ func()) *tview.Form {
			formCalled = true
			return tview.NewForm()
		},
		CustomView: func(tvApp *tview.Application, container *tview.Flex, _ func()) {
			tv := tview.NewTextView().SetText("custom")
			container.AddItem(tv, 0, 1, true)
		},
	}
	v := mountWizard(t, InlineWizardConfig{ID: "wizard.render.both", Steps: []WizardStep{step}})
	assert.False(t, formCalled)
	assert.Nil(t, v.form)
	assert.Equal(t, 1, v.stepContentInner.GetItemCount())
}

// A step with no form and no custom view only runs its OnDone.
func TestInlineWizardRenderStepProcessingOnly(t *testing.T) {
	step := WizardStep{Label: "Work", OnDone: func() error { return nil }}
	v := mountWizard(t, InlineWizardConfig{ID: "wizard.render.work", Steps: []WizardStep{step, formStep("After")}})
	assert.True(t, v.processing, "OnDone runs in the background")
	assert.Nil(t, v.form)
}

// The step header counts the visible steps in the classic layout only.
func TestInlineWizardRenderStepHeader(t *testing.T) {
	i18ntest.Set(t, "en")
	v := mountWizard(t, InlineWizardConfig{ID: "wizard.render.header", Steps: []WizardStep{formStep("Alpha"), formStep("Beta")}})
	assert.Contains(t, v.stepHeader.GetText(true), "Step 1/2 — Alpha")
	assert.NotContains(t, v.stepHeader.GetText(true), "◆", "the counter is written out: no diamond stuck to the number")
	v.advanceAfterDone(v.cfg.Steps[0])
	v.renderStep(v.currentStep)
	assert.Contains(t, v.stepHeader.GetText(true), "Step 2/2 — Beta")

	i18ntest.Set(t, "fr")
	v.renderStep(v.currentStep)
	assert.Contains(t, v.stepHeader.GetText(true), "Étape 2/2 — Beta")

	g := mountWizard(t, InlineWizardConfig{
		ID:     "wizard.render.header.grouped",
		Steps:  []WizardStep{formStep("Alpha")},
		Groups: []StepGroup{{Label: "G", StartIdx: 0}},
	})
	assert.Empty(t, g.stepHeader.GetText(true), "grouped: the sidebar replaces the header")
}

// In the grouped layout the buttons leave the form for a button form of the
// page, unless the terminal is too short: then they stay in the form.
func TestInlineWizardRenderStepGroupedButtons(t *testing.T) {
	cfg := func(id string) InlineWizardConfig {
		return InlineWizardConfig{ID: id, Steps: []WizardStep{formStep("Alpha", "Name")}, Groups: []StepGroup{{Label: "G", StartIdx: 0}}}
	}
	prev := wizardTermHeight
	t.Cleanup(func() { wizardTermHeight = prev })

	wizardTermHeight = func() int { return 50 }
	v := mountWizard(t, cfg("wizard.render.grouped.tall"))
	assert.Zero(t, v.form.GetButtonCount(), "tall terminal: the buttons are on the page")

	wizardTermHeight = func() int { return 20 }
	v = mountWizard(t, cfg("wizard.render.grouped.short"))
	assert.Equal(t, 1, v.form.GetButtonCount(), "short terminal: the buttons stay in the form")
}

// FocusButtonAfterRender lands on the submit button once, then resets.
func TestInlineWizardRenderStepFocusButtonAfterRender(t *testing.T) {
	flag := true
	prev := wizardTermHeight
	wizardTermHeight = func() int { return 50 }
	t.Cleanup(func() { wizardTermHeight = prev })
	app := tview.NewApplication()
	v := NewInlineWizardView(InlineWizardConfig{
		ID:                     "wizard.render.focus",
		Steps:                  []WizardStep{formStep("Alpha", "Name")},
		Groups:                 []StepGroup{{Label: "G", StartIdx: 0}},
		FocusButtonAfterRender: &flag,
	})
	v.Mount(tview.NewFlex().SetDirection(tview.FlexRow), app)
	t.Cleanup(v.Unmount)
	assert.False(t, flag, "the flag is consumed")
	_, isButton := app.GetFocus().(*tview.Button)
	assert.True(t, isButton, "the submit button has the focus, not the input of the step")
}

func TestInlineWizardRenderStepFocusesTheFormByDefault(t *testing.T) {
	prev := wizardTermHeight
	wizardTermHeight = func() int { return 50 }
	t.Cleanup(func() { wizardTermHeight = prev })
	app := tview.NewApplication()
	v := NewInlineWizardView(InlineWizardConfig{
		ID:     "wizard.render.focus.default",
		Steps:  []WizardStep{formStep("Alpha", "Name")},
		Groups: []StepGroup{{Label: "G", StartIdx: 0}},
	})
	v.Mount(tview.NewFlex().SetDirection(tview.FlexRow), app)
	t.Cleanup(v.Unmount)
	_, isInput := app.GetFocus().(*tview.InputField)
	assert.True(t, isInput, "the first input of the step has the focus")
}
