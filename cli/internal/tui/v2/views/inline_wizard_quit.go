package views

import (
	"fmt"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// « Quit the wizard? » (N19): Esc asks, in place of the hints bar.

// askQuit is Esc: « Quitter l'assistant ? [Quitter] [Passer l'étape] (when
// the step is optional) [Continuer] », in place of the hints bar. The focus
// starts on Continuer.
func (w *InlineWizardView) askQuit() {
	if w.app == nil || w.quitBar != nil || w.hintsHost == nil || w.hintsBar == nil {
		return
	}
	step := w.cfg.Steps[w.currentStep]
	w.quitBack = w.app.GetFocus()

	question := fmt.Sprintf("%s%s %s[-] ", theme.ColorTagOf(theme.Warning), theme.IconWarning, i18n.T("tui.wizard.quit.title"))
	label := tview.NewTextView().SetDynamicColors(true).SetText(question)
	label.SetBackgroundColor(theme.BgPanel)

	buttons := tview.NewForm().SetHorizontal(true)
	buttons.SetBackgroundColor(theme.BgPanel)
	buttons.SetBorderPadding(0, 0, 0, 0)
	buttons.SetButtonsAlign(tview.AlignLeft)
	buttons.AddButton(i18n.T("tui.wizard.quit.quit"), w.quitWizard)
	if !step.Required {
		buttons.AddButton(i18n.T("tui.wizard.quit.skip"), w.skipFromPrompt)
	}
	buttons.AddButton(i18n.T("tui.wizard.quit.continue"), w.closeQuitPrompt)
	widgets.StyleFormButtons(buttons, buttons.GetButtonCount()-1) // « Continuer »
	buttons.SetCancelFunc(w.closeQuitPrompt)
	buttons.SetFocus(buttons.GetButtonCount() - 1)

	bar := tview.NewFlex().
		AddItem(label, tview.TaggedStringWidth(question)+1, 0, false).
		AddItem(buttons, 0, 1, true)
	bar.SetBackgroundColor(theme.BgPanel)
	w.hintsHost.ResizeItem(w.hintsBar.TextView, 0, 0)
	w.hintsHost.AddItem(bar, 1, 0, true)
	w.quitBar = bar
	w.app.SetFocus(buttons)
}

// moveQuitFocus moves the focus between the buttons of the quit prompt
// (←/→, Tab/Shift+Tab), wrapping around.
func (w *InlineWizardView) moveQuitFocus(delta int) {
	if w.quitBar == nil || w.app == nil {
		return
	}
	buttons, ok := w.quitBar.GetItem(1).(*tview.Form)
	if !ok || buttons.GetButtonCount() == 0 {
		return
	}
	n := buttons.GetButtonCount()
	cur := 0
	for i := 0; i < n; i++ {
		if buttons.GetButton(i).HasFocus() {
			cur = i
		}
	}
	next := ((cur+delta)%n + n) % n
	buttons.SetFocus(next)
	w.app.SetFocus(buttons)
}

// closeQuitPrompt is « Continuer » (or Esc on the prompt): back to the step.
func (w *InlineWizardView) closeQuitPrompt() {
	if w.quitBar == nil {
		return
	}
	if w.hintsHost != nil {
		w.hintsHost.RemoveItem(w.quitBar)
		if w.hintsBar != nil {
			w.hintsHost.ResizeItem(w.hintsBar.TextView, 1, 0)
		}
	}
	w.quitBar = nil
	back := w.quitBack
	w.quitBack = nil
	if w.app != nil && back != nil {
		w.app.SetFocus(back)
	}
}

// quitWizard is « Quitter »: OnComplete(false, nil), then the wizard is
// popped.
func (w *InlineWizardView) quitWizard() {
	w.closeQuitPrompt()
	if !w.completed && !w.aborted {
		w.aborted = true
		w.fireComplete()
	}
	if w.shell != nil {
		w.shell.PopView()
	}
}

// skipFromPrompt is « Passer l'étape » (optional steps only).
func (w *InlineWizardView) skipFromPrompt() {
	w.closeQuitPrompt()
	if w.currentStep < 0 || w.currentStep >= len(w.cfg.Steps) || w.cfg.Steps[w.currentStep].Required {
		return
	}
	w.skipCurrent()
	if !w.completed && w.doRenderStep != nil {
		w.doRenderStep(w.currentStep)
	}
}

// formDropDownOpen reports whether a drop-down list of the form is open.
func formDropDownOpen(form *tview.Form) bool {
	if form == nil {
		return false
	}
	for i := 0; i < form.GetFormItemCount(); i++ {
		if dd, ok := form.GetFormItem(i).(*tview.DropDown); ok && dd.IsOpen() {
			return true
		}
	}
	return false
}
