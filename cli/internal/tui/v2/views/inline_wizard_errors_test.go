package views

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// completion records the calls of OnComplete (made on the event loop).
type completion struct {
	mu    sync.Mutex
	calls []completionCall
}

type completionCall struct {
	completed bool
	err       error
}

func (c *completion) fn(completed bool, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, completionCall{completed, err})
}

func (c *completion) all() []completionCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]completionCall(nil), c.calls...)
}

// failingStep is a step whose OnDone fails with err (once, or always).
func failingStep(id, label string, fails *int, err error) WizardStep {
	s := formStep(label)
	s.ID = id
	s.OnDone = func() error {
		if *fails > 0 {
			*fails--
			return err
		}
		return nil
	}
	return s
}

// waitLoop polls fn on the event loop until it holds.
func waitLoop(t *testing.T, app *tview.Application, fn func() bool) {
	t.Helper()
	require.Eventually(t, func() bool {
		ok := make(chan bool, 1)
		app.QueueUpdate(func() { ok <- fn() })
		select {
		case v := <-ok:
			return v
		case <-time.After(time.Second):
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)
}

func pressEnter(f *tview.Form) {
	f.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
}

// errorRow is the row of buttons of the error screen.
func errorRow(t *testing.T, v *InlineWizardView) *tview.Form {
	t.Helper()
	require.GreaterOrEqual(t, v.stepContentInner.GetItemCount(), 2, "the error screen has a row of buttons")
	row, ok := v.stepContentInner.GetItem(1).(*tview.Form)
	require.True(t, ok)
	return row
}

func buttonLabels(f *tview.Form) []string {
	var out []string
	for i := 0; i < f.GetButtonCount(); i++ {
		out = append(out, strings.Trim(f.GetButton(i).GetLabel(), " ▸◂"))
	}
	return out
}

// A step that failed and that the user skips (Esc → « Passer l'étape ») does
// not take its error to the end of the wizard: OnComplete(true, nil).
func TestInlineWizardSkippingAFailedStepForgetsItsError(t *testing.T) {
	fails := 1
	boom := errors.New("boom")
	failing := failingStep("clone", "Clone", &fails, boom)
	var done completion
	v := NewInlineWizardView(InlineWizardConfig{
		ID:         "wizard.errors.skip",
		Steps:      []WizardStep{failing, formStep("After")},
		OnComplete: done.fn,
	})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() { v.completeStep(v.cfg.Steps[0]) })
	waitLoop(t, app, func() bool { return v.wizardErr != nil })

	onLoop(app, func() {
		v.askQuit()
		require.NotNil(t, v.quitBar)
		v.skipFromPrompt()
	})
	onLoop(app, func() {
		assert.NoError(t, v.wizardErr, "the error of the step the user left is forgotten")
		assert.Equal(t, 1, v.currentStep, "the next step is shown")
		v.advanceAfterDone(v.cfg.Steps[1])
	})
	onLoop(app, func() {
		require.True(t, v.completed)
		assert.Equal(t, []completionCall{{completed: true}}, done.all(), "the wizard ends with no error")
		v.Unmount()
	})
}

// OnSkip runs when a step is skipped, before the next one is chosen: the
// state it sets skips the steps of the group.
func TestInlineWizardOnSkipSkipsTheRestOfTheGroup(t *testing.T) {
	skipped := false
	group := func(label string) WizardStep {
		s := formStep(label)
		s.SkipIf = func() bool { return skipped }
		return s
	}
	first := group("First")
	first.OnSkip = func() { skipped = true }
	var done completion
	v := mountWizard(t, InlineWizardConfig{
		ID:         "wizard.errors.onskip",
		Steps:      []WizardStep{first, group("Second"), group("Third"), formStep("Outside")},
		OnComplete: done.fn,
	})

	v.skipCurrent()
	assert.Equal(t, 3, v.currentStep, "the group is left whole")
	assert.True(t, skipped)
}

// The error screen of a step with ErrorActions has a row of buttons; Enter,
// with the focus where it starts, retries.
func TestInlineWizardErrorActionsRetryOnEnter(t *testing.T) {
	i18ntest.Set(t, "en")
	fails := 1
	failing := failingStep("clone", "Clone", &fails, errors.New("boom"))
	failing.ErrorActions = func(err error) []ErrorAction {
		return []ErrorAction{{Label: "Fix access", Key: 'a', GoToStep: "clone"}, {Label: "Give up", Skip: true}}
	}
	v := NewInlineWizardView(InlineWizardConfig{ID: "wizard.errors.retry", Steps: []WizardStep{failing, formStep("After")}})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() { v.completeStep(v.cfg.Steps[0]) })
	waitLoop(t, app, func() bool { return v.wizardErr != nil })

	onLoop(app, func() {
		row := errorRow(t, v)
		assert.Equal(t, []string{i18n.T("v532w.error.retry"), "Fix access", "Give up"}, buttonLabels(row),
			"retry first, then the actions of the step")
		assert.True(t, row.GetButton(0).HasFocus(), "the focus starts on retry")
		screen := screenText(t, v.stepContentInner, 100, 12)
		assert.Contains(t, screen, "boom", "the error is shown")
		for _, label := range []string{i18n.T("v532w.error.retry"), "Fix access", "Give up"} {
			assert.Contains(t, screen, label, "the button is drawn")
		}
		pressEnter(row)
	})
	onLoop(app, func() {
		assert.NoError(t, v.wizardErr, "retrying draws the step again")
		assert.Equal(t, 0, v.currentStep)
		assert.NotNil(t, v.form, "the form of the step is back")
	})
	onLoop(app, func() { v.Unmount() })
}

// A step puts its own « retry » button where it wants (« Configurer l'accès »
// first on an authentication failure): the focus, and Enter, stay on retry.
func TestInlineWizardErrorActionsKeepRetryUnderEnter(t *testing.T) {
	i18ntest.Set(t, "en")
	fails := 1
	failing := failingStep("clone", "Clone", &fails, errors.New("boom"))
	failing.ErrorActions = func(err error) []ErrorAction {
		return []ErrorAction{{Label: "Fix access", GoToStep: "clone"}, {Label: "Try again", Retry: true}}
	}
	v := NewInlineWizardView(InlineWizardConfig{ID: "wizard.errors.order", Steps: []WizardStep{failing, formStep("After")}})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() { v.completeStep(v.cfg.Steps[0]) })
	waitLoop(t, app, func() bool { return v.wizardErr != nil })

	onLoop(app, func() {
		row := errorRow(t, v)
		assert.Equal(t, []string{"Fix access", "Try again"}, buttonLabels(row))
		assert.True(t, row.GetButton(1).HasFocus(), "Enter retries")
		pressEnter(row)
	})
	onLoop(app, func() {
		assert.NoError(t, v.wizardErr)
		assert.NotNil(t, v.form)
		v.Unmount()
	})
}

// « Configurer l'accès »: back to the access step, which was skipped, shown
// now because the action says so; the steps after it are done again.
func TestInlineWizardErrorActionGoesBackToAStepByID(t *testing.T) {
	i18ntest.Set(t, "en")
	forceAccess := false
	access := formStep("Access")
	access.ID = "access"
	access.SkipIf = func() bool { return !forceAccess }
	fails := 1
	clone := failingStep("clone", "Clone", &fails, errors.New("auth failed"))
	clone.ErrorActions = func(err error) []ErrorAction {
		return []ErrorAction{{Label: "Set up access", Key: 'a', Before: func() { forceAccess = true }, GoToStep: "access"}}
	}
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.errors.goto",
		Steps: []WizardStep{formStep("URL"), access, clone, formStep("After")},
	})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() { v.advanceAfterDone(v.cfg.Steps[0]); v.renderStep(v.currentStep) })
	onLoop(app, func() {
		require.Equal(t, 2, v.currentStep, "the access step is skipped")
		v.completeStep(v.cfg.Steps[2])
	})
	waitLoop(t, app, func() bool { return v.wizardErr != nil })

	onLoop(app, func() {
		row := errorRow(t, v)
		// the hot key of the action triggers it
		row.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))
	})
	onLoop(app, func() {
		assert.NoError(t, v.wizardErr)
		assert.Equal(t, 1, v.currentStep, "back on the access step")
		assert.NotNil(t, v.form, "its form is shown")
		assert.Equal(t, 1, len(v.infoAccum), "only the info of the URL step is kept")
		v.Unmount()
	})
}

// The action Skip skips the failed step even when it is Required, with OnSkip.
func TestInlineWizardErrorActionSkipRunsOnSkip(t *testing.T) {
	i18ntest.Set(t, "en")
	group := false
	fails := 1
	clone := failingStep("clone", "Clone", &fails, errors.New("boom"))
	clone.Required = true
	clone.OnSkip = func() { group = true }
	clone.ErrorActions = func(error) []ErrorAction { return []ErrorAction{{Label: "Skip the team", Skip: true}} }
	rest := formStep("Rest")
	rest.SkipIf = func() bool { return group }
	var done completion
	v := NewInlineWizardView(InlineWizardConfig{
		ID:         "wizard.errors.skipaction",
		Steps:      []WizardStep{clone, rest},
		OnComplete: done.fn,
	})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() { v.completeStep(v.cfg.Steps[0]) })
	waitLoop(t, app, func() bool { return v.wizardErr != nil })

	onLoop(app, func() {
		row := errorRow(t, v)
		row.GetInputCapture()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		pressEnter(row)
	})
	onLoop(app, func() {
		assert.True(t, group, "OnSkip ran")
		assert.True(t, v.completed, "nothing is left to ask")
		assert.Equal(t, []completionCall{{completed: true}}, done.all(), "completed with no error")
		v.Unmount()
	})
}

// Without ErrorActions the error screen is the old one: Enter retries.
func TestInlineWizardErrorScreenWithoutActionsRetriesOnEnter(t *testing.T) {
	fails := 1
	failing := failingStep("clone", "Clone", &fails, errors.New("boom"))
	v := NewInlineWizardView(InlineWizardConfig{ID: "wizard.errors.plain", Steps: []WizardStep{failing}})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() { v.completeStep(v.cfg.Steps[0]) })
	waitLoop(t, app, func() bool { return v.wizardErr != nil })
	onLoop(app, func() {
		assert.Equal(t, 1, v.stepContentInner.GetItemCount(), "no row of buttons")
		tv, ok := v.stepContentInner.GetItem(0).(*tview.TextView)
		require.True(t, ok)
		tv.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
		assert.NoError(t, v.wizardErr)
		v.Unmount()
	})
}

// Back passes over a step that only does something (BackSkip): from the form
// after it, it lands on the form before it; going forward does it again.
func TestInlineWizardBackSkipsAStepWithNothingToEdit(t *testing.T) {
	runs := 0
	probe := WizardStep{ID: "probe", Label: "Probe", BackSkip: true, OnDone: func() error { runs++; return nil }}
	var done completion
	v := NewInlineWizardView(InlineWizardConfig{
		ID:         "wizard.errors.backskip",
		Steps:      []WizardStep{formStep("URL"), probe, formStep("Identity")},
		OnComplete: done.fn,
	})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() { v.completeStep(v.cfg.Steps[0]) })
	waitLoop(t, app, func() bool { return v.currentStep == 2 && !v.processing })
	onLoop(app, func() {
		assert.Equal(t, 1, runs)
		assert.Contains(t, v.statusHintsForStep(2), "ctrl+b", "there is a step to go back to")
		v.goBack()
		assert.Equal(t, 0, v.currentStep, "back lands on the form before the probe")
		assert.Equal(t, widgets.StepPending, v.stepStates[1].Status, "the probe is to be done again")
		assert.Equal(t, 0, len(v.infoAccum), "no info of the steps left behind")
		v.completeStep(v.cfg.Steps[0])
	})
	waitLoop(t, app, func() bool { return v.currentStep == 2 && !v.processing })
	onLoop(app, func() {
		assert.Equal(t, 2, runs, "going forward does the probe again")
		v.Unmount()
	})
}
