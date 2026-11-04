package views

import (
	"fmt"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// The OnDone of a step runs off the event loop, behind a delayed spinner; an
// error stays on the step (Ctrl+B back, Enter retry).

func (w *InlineWizardView) runWithSpinner(step WizardStep, afterDone func()) {
	// Fast path: no async work — transition directly without goroutine.
	if step.OnDone == nil {
		afterDone()
		return
	}
	w.processing = true

	// Block input on the current step content while OnDone executes.
	// The current content stays visible (no flash) until either OnDone
	// completes (direct transition) or the spinner delay expires.
	prevCapture := w.stepContent.GetInputCapture()
	w.stepContent.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return nil // absorb all keys during processing
	})

	msg := i18n.T("wizard.processing")
	if step.Processing != "" {
		msg = step.Processing
	}
	w.spinner.SetMessage(msg)

	// Capture app reference before launching goroutine to avoid nil
	// dereference if Unmount() runs before QueueUpdateDraw fires.
	tvApp := w.app

	// What this run does to the wizard only counts while the wizard is still
	// in the mount that started it: after an Unmount, and even after a new
	// Mount, the result of this OnDone belongs to nobody.
	gen, finished := w.gen, false
	current := func() bool { return w.app != nil && w.gen == gen }

	// showSpinner replaces the current step content with the spinner.
	// Called either by the delayed timer or not at all if OnDone is fast.
	spinnerShown := false
	showSpinner := func() {
		if !current() || finished {
			return
		}
		spinnerShown = true
		w.stepContent.SetInputCapture(prevCapture)
		w.showSpinnerContent()
	}

	// Delayed spinner: only show after 1s if OnDone hasn't finished yet.
	// This keeps the previous step content visible during short operations,
	// eliminating the flash that occurred when the spinner appeared briefly
	// for fast OnDone callbacks.
	spinnerTimer := safego.AfterFunc(1*time.Second, func() {
		tvApp.QueueUpdateDraw(func() {
			showSpinner()
		})
	})
	w.trackTimer(spinnerTimer)

	safego.Go(func() {
		var err error
		if step.OnDone != nil {
			err = step.OnDone()
		}
		spinnerTimer.Stop()
		tvApp.QueueUpdateDraw(func() {
			finished = true
			// Guard: wizard may have been unmounted (and mounted again)
			// while OnDone was running. Mount reset processing.
			if !current() {
				return
			}
			w.processing = false
			if spinnerShown {
				w.spinner.Stop()
			}
			// Restore input capture.
			w.stepContent.SetInputCapture(prevCapture)

			if err != nil {
				w.showStepError(err)
				return
			}
			afterDone()
		})
	})
}

func (w *InlineWizardView) showSpinnerContent() {
	w.stepContentInner.Clear()
	w.stepContentInner.AddItem(w.spinner.TextView, 3, 0, false)
	bgFill := tview.NewBox()
	bgFill.SetBackgroundColor(theme.BgPanel)
	w.stepContentInner.AddItem(bgFill, 0, 1, false)
	w.spinner.Start(w.app)
}

// showStepError shows the error of an OnDone in the step content but does NOT
// mark the step as completed. The user can press Ctrl+B to go back and fix the
// issue, or Enter to retry the current step.
func (w *InlineWizardView) showStepError(err error) {
	w.wizardErr = err
	w.stepContentInner.Clear()
	if w.currentStep >= 0 && w.currentStep < len(w.cfg.Steps) {
		if build := w.cfg.Steps[w.currentStep].ErrorActions; build != nil {
			if actions := withRetry(build(err)); len(actions) > 0 {
				w.showStepErrorActions(err, actions)
				return
			}
		}
	}
	errView := tview.NewTextView().SetDynamicColors(true)
	errView.SetBackgroundColor(theme.BgPanel)
	errView.SetText(fmt.Sprintf("  %s%s %s[-]\n\n  %s%s[-]\n\n  %sCtrl+B[-] %s  •  %sEnter[-] %s",
		theme.ColorTagOf(theme.Error), theme.IconError,
		i18n.T("wizard.error"),
		theme.ColorTagOf(theme.FgSecondary), err.Error(),
		theme.ColorTagOf(theme.ActiveMode.Primary), i18n.T("wizard.error.back"),
		theme.ColorTagOf(theme.ActiveMode.Primary), i18n.T("wizard.error.retry")))
	errView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyCtrlB:
			w.goBack()
			return nil
		case event.Key() == tcell.KeyEnter:
			// Retry: re-render the current step
			w.wizardErr = nil
			w.renderStep(w.currentStep)
			return nil
		}
		return event
	})
	w.stepContentInner.AddItem(errView, 0, 1, true)
	w.app.SetFocus(errView)
}

// withRetry puts a retry action first when the list has none.
func withRetry(actions []ErrorAction) []ErrorAction {
	for _, a := range actions {
		if a.Retry {
			return actions
		}
	}
	return append([]ErrorAction{{Label: i18n.T("v532w.error.retry"), Retry: true}}, actions...)
}

// showStepErrorActions is the error screen with a row of buttons: the focus
// starts on the retry button, Tab and the arrows move along the row, a letter
// triggers its action, Ctrl+B goes back.
func (w *InlineWizardView) showStepErrorActions(err error, actions []ErrorAction) {
	errView := tview.NewTextView().SetDynamicColors(true)
	errView.SetBackgroundColor(theme.BgPanel)
	errView.SetText(fmt.Sprintf("  %s%s %s[-]\n\n  %s%s[-]\n\n  %sCtrl+B[-] %s  •  %sTab/←→[-] %s  •  %sEnter[-] %s",
		theme.ColorTagOf(theme.Error), theme.IconError,
		i18n.T("wizard.error"),
		theme.ColorTagOf(theme.FgSecondary), err.Error(),
		theme.ColorTagOf(theme.ActiveMode.Primary), i18n.T("wizard.error.back"),
		theme.ColorTagOf(theme.ActiveMode.Primary), i18n.T("v532w.error.choose"),
		theme.ColorTagOf(theme.ActiveMode.Primary), i18n.T("v532w.error.validate")))

	buttons := tview.NewForm().SetHorizontal(true)
	buttons.SetBackgroundColor(theme.BgPanel)
	buttons.SetBorderPadding(0, 0, 2, 0)
	buttons.SetButtonsAlign(tview.AlignLeft)
	retryIdx := 0
	for i, a := range actions {
		if a.Retry {
			retryIdx = i
		}
		a := a
		buttons.AddButton("  "+a.Label+"  ", func() { w.runErrorAction(a) })
	}
	widgets.StyleFormButtons(buttons, retryIdx)
	buttons.SetFocus(retryIdx)
	buttons.SetCancelFunc(w.askQuit)
	buttons.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		n := buttons.GetButtonCount()
		cur := retryIdx
		for i := 0; i < n; i++ {
			if buttons.GetButton(i).HasFocus() {
				cur = i
			}
		}
		move := func(delta int) *tcell.EventKey {
			buttons.SetFocus(((cur+delta)%n + n) % n)
			w.app.SetFocus(buttons)
			return nil
		}
		switch event.Key() {
		case tcell.KeyCtrlB:
			w.goBack()
			return nil
		case tcell.KeyLeft, tcell.KeyBacktab:
			return move(-1)
		case tcell.KeyRight, tcell.KeyTab:
			return move(1)
		case tcell.KeyRune:
			for _, a := range actions {
				if a.Key != 0 && unicode.ToLower(event.Rune()) == unicode.ToLower(a.Key) {
					w.runErrorAction(a)
					return nil
				}
			}
		}
		return event
	})

	w.stepContentInner.AddItem(errView, 0, 1, false)
	w.stepContentInner.AddItem(buttons, 3, 0, true)
	w.app.SetFocus(buttons)
}

// runErrorAction does what a button of the error screen says.
func (w *InlineWizardView) runErrorAction(a ErrorAction) {
	if a.Before != nil {
		a.Before()
	}
	switch {
	case a.GoToStep != "":
		if !w.goToStep(a.GoToStep) {
			w.wizardErr = nil
			w.renderStep(w.currentStep)
		}
	case a.Skip:
		w.skipCurrent()
		if !w.completed && w.doRenderStep != nil {
			w.doRenderStep(w.currentStep)
		}
	case a.Quit:
		w.quitWizard()
	default:
		w.wizardErr = nil
		w.renderStep(w.currentStep)
	}
}
