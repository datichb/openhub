package views

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// Step rendering: one function per kind of step (skipped, custom view, form,
// processing only).

// renderStep draws step idx: a step that asks to be skipped moves on to the
// next one, the others get their area cleared and filled by their own kind
// (custom view, form, or no widget at all).
func (w *InlineWizardView) renderStep(idx int) {
	w.refreshLabels()

	// Inject Rerender before reading the step so the Form callback can use it.
	w.cfg.Steps[idx].Rerender = func() { w.doRenderStep(w.currentStep) }

	step := w.cfg.Steps[idx]

	if w.shouldSkip(idx) {
		w.renderNextAfterSkip(idx)
		return
	}
	w.prepareStepArea(idx, step)

	switch {
	case step.CustomView != nil:
		w.renderCustomStep(step)
	case step.Form != nil:
		w.renderFormStep(step)
	default:
		// No form, no CustomView: processing-only step.
		w.completeStep(step)
	}
}

// refreshLabels re-evaluates the locale-dependent labels (RefreshLabels) and
// syncs the step bar and the groups with them.
func (w *InlineWizardView) refreshLabels() {
	if w.cfg.RefreshLabels == nil {
		return
	}
	w.cfg.RefreshLabels()
	for i, s := range w.cfg.Steps {
		if i < len(w.stepStates) {
			w.stepStates[i].Label = s.Label
		}
	}
	if len(w.cfg.Groups) > 0 {
		for gi, g := range w.cfg.Groups {
			if gi < len(w.groupStates) {
				w.groupStates[gi].Label = g.Label
			}
		}
	}
}

// renderNextAfterSkip marks the skipped step idx done and draws the next one,
// or completes the wizard when there is none.
func (w *InlineWizardView) renderNextAfterSkip(idx int) {
	w.syncStepBar(idx, widgets.StepDone)
	next := w.findNext(idx)
	if next == -1 {
		w.wizardComplete()
		return
	}
	w.currentStep = next
	w.syncStepBar(next, widgets.StepActive)
	w.renderInfoPanel()
	w.doRenderStep(next)
}

// prepareStepArea clears the area of step idx and sets what every kind of
// step shares: the step counter, the sidebar, Ctrl+B and the hints.
func (w *InlineWizardView) prepareStepArea(idx int, step WizardStep) {
	// Clear step content (inner only — outer keeps its wrapper/DrawFunc)
	w.stepContentInner.Clear()

	// Step counter header — classic mode only (grouped mode uses sidebar)
	if len(w.cfg.Groups) == 0 {
		totalSteps, stepPos := w.countVisibleSteps(idx)
		w.stepHeader.SetText(fmt.Sprintf("  %s%s %d/%d — %s[-]",
			theme.ColorTagOf(theme.ActiveMode.Primary), theme.IconActive, stepPos, totalSteps, step.Label))
	}

	// Refresh sidebar in grouped mode
	w.renderInfoPanel()

	// Wire Ctrl+B on the step content container
	w.stepContent.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlB {
			w.goBack()
			return nil
		}
		return event
	})

	w.form = nil
	w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))

	if step.Form != nil && step.CustomView != nil {
		slog.Warn("wizard step has both Form and CustomView set; CustomView takes precedence",
			"step", step.Label, "idx", idx)
	}
}

// completeStep runs the OnDone of a validated step, then draws the next one.
func (w *InlineWizardView) completeStep(step WizardStep) {
	w.runWithSpinner(step, func() {
		w.advanceAfterDone(step)
		if !w.completed {
			w.doRenderStep(w.currentStep)
		}
	})
}

// showStepProblem puts the message of a failed Validate in the hints bar for a
// few seconds.
func (w *InlineWizardView) showStepProblem(msg string) {
	w.hintsBar.SetHints(fmt.Sprintf("%s%s %s[-]",
		theme.ColorTagOf(theme.Error), theme.IconWarning, msg))
	w.trackTimer(safego.AfterFunc(3*time.Second, func() {
		if w.app != nil {
			w.app.QueueUpdateDraw(func() {
				if w.hintsBar != nil {
					w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
				}
			})
		}
	}))
}

// renderCustomStep gives the area to the custom view of the step; onDone
// validates the step like the form path does.
func (w *InlineWizardView) renderCustomStep(step WizardStep) {
	onDone := func() {
		if step.Validate != nil {
			if errMsg := step.Validate(); errMsg != "" {
				w.showStepProblem(errMsg)
				return
			}
		}
		w.completeStep(step)
	}
	step.CustomView(w.app, w.stepContentInner, onDone)
}

// renderFormStep builds the form of the step, themes it and places it: alone
// in the classic layout, in a page with its button in the grouped one.
func (w *InlineWizardView) renderFormStep(step WizardStep) {
	var form *tview.Form // pre-declare so onDone closure can reference it

	onDone := func() {
		if step.Validate != nil {
			if errMsg := step.Validate(); errMsg != "" {
				w.showStepProblem(errMsg)
				w.markFirstInput(form)
				return
			}
		}
		w.completeStep(step)
	}

	form = step.Form(w.app, onDone)
	if form == nil {
		return
	}
	w.themeForm(form)
	w.form = form

	// Ctrl+S and the button of the engine go through the main button
	// of the step, like a press on it: its handler may set the state
	// the step needs (the services ticked, the policies chosen).
	submit := primaryButtonAction(form, onDone)

	// Ctrl+S = submit, Ctrl+B = back
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Arrow keys → Tab/Backtab (except on DropDowns)
		if remapped := remapArrowToTab(form, event); remapped != nil {
			defer w.updateDropdownHints(form)
			return remapped
		}
		if event.Key() == tcell.KeyCtrlS {
			submit()
			return nil
		}
		if event.Key() == tcell.KeyCtrlB {
			w.goBack()
			return nil
		}
		// Update hints after any navigation key (Tab, Backtab, Enter)
		// that may shift focus to or from a DropDown.
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyEnter {
			defer w.updateDropdownHints(form)
		}
		return event
	})

	if len(w.cfg.Groups) > 0 {
		w.placeGroupedForm(form, step, submit)
		return
	}
	w.stepContentInner.AddItem(form, 0, 1, true)
	w.app.SetFocus(form)
}

// markFirstInput puts the error icon on the label of the first field of the
// form for a few seconds.
func (w *InlineWizardView) markFirstInput(form *tview.Form) {
	if form == nil || form.GetFormItemCount() == 0 {
		return
	}
	input, ok := form.GetFormItem(0).(*tview.InputField)
	if !ok {
		return
	}
	origLabel := input.GetLabel()
	input.SetLabel(fmt.Sprintf("%s%s[-] %s",
		theme.ColorTagOf(theme.Error), theme.IconError, origLabel))
	w.trackTimer(safego.AfterFunc(3*time.Second, func() {
		if w.app != nil {
			w.app.QueueUpdateDraw(func() { input.SetLabel(origLabel) })
		}
	}))
}

// themeForm applies the theme to a step form.
func (w *InlineWizardView) themeForm(form *tview.Form) {
	form.SetBackgroundColor(theme.BgPanel)
	form.SetFieldBackgroundColor(theme.BgElement)
	form.SetFieldTextColor(theme.FgPrimary)
	form.SetLabelColor(theme.FgPrimary)
	widgets.StyleFormButtons(form, 0)
	form.SetBorder(false)

	// Fix DropDown popup list colors (tview bakes them at construction
	// time from tview.Styles, which produces invisible text in our theme).
	fixFormDropDownStyles(form)

	// Apply initial label focus hierarchy (bold on focused, muted on rest).
	fixFormLabelFocus(form)

	// Esc asks whether to quit the wizard (N19). When a DropDown has
	// focus, the Escape closing its popup also reaches CancelFunc
	// (tview forwards it via finishedFunc): swallow it.
	form.SetCancelFunc(func() {
		if isDropDownFocused(form) {
			w.updateDropdownHints(form)
			return
		}
		w.askQuit()
	})
}

// placeGroupedForm places the form of a step in the grouped layout. The
// buttons leave the form for a button form at a fixed position (same as
// intros), unless the terminal is too short: then they stay inline.
func (w *InlineWizardView) placeGroupedForm(form *tview.Form, step WizardStep, submit func()) {
	// shell overhead: omnibar(5) + border_padding(1) + hints(1)
	minimalLayout := wizardTermHeight()-7 < 20
	if minimalLayout {
		w.placeMinimalForm(form, submit)
		return
	}
	form.ClearButtons()
	w.placeFormPage(form, step, submit)
}

// placeMinimalForm is the form alone with its inline buttons.
func (w *InlineWizardView) placeMinimalForm(form *tview.Form, submit func()) {
	w.stepContentInner.AddItem(form, 0, 1, true)

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlS {
			submit()
			return nil
		}
		if event.Key() == tcell.KeyCtrlB {
			w.goBack()
			return nil
		}
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyEnter {
			defer w.updateDropdownHints(form)
		}
		return event
	})
}

// formPageWidth is the width of the form in the page: from its labels, 60 to
// 80 columns, within the width available.
func formPageWidth(form *tview.Form) int {
	formMaxWidth := 60
	for i := 0; i < form.GetFormItemCount(); i++ {
		label := form.GetFormItem(i).GetLabel()
		w2 := tview.TaggedStringWidth(label) + 40
		if w2 > formMaxWidth {
			formMaxWidth = w2
		}
	}
	if formMaxWidth > 80 {
		formMaxWidth = 80
	}
	// Cap at available width to prevent clipping on small terminals.
	if innerW := StepContentInnerWidth(); innerW > 0 && formMaxWidth > innerW {
		formMaxWidth = innerW
	}
	return formMaxWidth
}

// placeFormPage assembles the page of a step of the grouped layout: the badge,
// the form and a button form at a fixed position.
func (w *InlineWizardView) placeFormPage(form *tview.Form, step WizardStep, submit func()) {
	formMaxWidth := formPageWidth(form)

	form.SetBorderPadding(1, 1, 2, 2) // match styleWizardForm for visual consistency

	// Separate button form at fixed position.
	buttonForm := NewStyledButtonForm()
	buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", submit)

	// Assemble the page layout via BuildWizardPage.
	BuildWizardPage(nil, w.stepContentInner, WizardPageLayout{
		Badge:           step.Label,
		Content:         form,
		ContentMaxWidth: formMaxWidth,
		Buttons:         buttonForm,
		// FocusTarget: nil — we handle focus below.
	})

	// Install navigation: arrow remap + cross-section.
	SetupFormNavigation(form)
	SetupCrossSectionNav(CrossSectionNavConfig{
		App:     w.app,
		Content: form,
		Buttons: buttonForm,
	})
	w.composePageCaptures(form, buttonForm, submit)

	// FocusButtonAfterRender: use direct reference instead of scanning.
	w.app.SetFocus(form)
	if w.cfg.FocusButtonAfterRender != nil && *w.cfg.FocusButtonAfterRender {
		*w.cfg.FocusButtonAfterRender = false
		w.app.SetFocus(buttonForm)
	}
}

// composePageCaptures puts the engine keys (Ctrl+S, Ctrl+B) ON TOP of the
// navigation captures of the form and of its button form.
func (w *InlineWizardView) composePageCaptures(form, buttonForm *tview.Form, submit func()) {
	navFormCapture := form.GetInputCapture()
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlS {
			submit()
			return nil
		}
		if event.Key() == tcell.KeyCtrlB {
			w.goBack()
			return nil
		}
		// Delegate to navigation capture.
		var result *tcell.EventKey
		if navFormCapture != nil {
			result = navFormCapture(event)
		} else {
			result = event
		}
		// Update hints after navigation keys.
		if result != nil && (event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyEnter ||
			event.Key() == tcell.KeyDown || event.Key() == tcell.KeyUp) {
			w.updateDropdownHints(form)
		}
		return result
	})

	navBtnCapture := buttonForm.GetInputCapture()
	buttonForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlS {
			submit()
			return nil
		}
		if navBtnCapture != nil {
			return navBtnCapture(event)
		}
		return event
	})
}

// wizardTermHeight is the height of the terminal (50 when it is unknown); a
// variable so that the tests choose the layout.
var wizardTermHeight = func() int {
	_, termH, _ := term.GetSize(int(os.Stdout.Fd()))
	if termH <= 0 {
		termH = 50 // safe fallback
	}
	return termH
}

// primaryButtonAction returns the action of the first button of the form: a
// press on it, so that its handler runs. The button stays valid when the
// form gives it up (grouped layout). Without a button, it is fallback.
func primaryButtonAction(form *tview.Form, fallback func()) func() {
	if form.GetButtonCount() == 0 {
		return fallback
	}
	button := form.GetButton(0)
	return func() {
		button.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	}
}
