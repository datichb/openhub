package views

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
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

	// showSpinner replaces the current step content with the spinner.
	// Called either by the delayed timer or not at all if OnDone is fast.
	spinnerShown := false
	showSpinner := func() {
		if w.app == nil {
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
			w.processing = false
			// Guard: wizard may have been unmounted while OnDone was running.
			if w.app == nil {
				return
			}
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
