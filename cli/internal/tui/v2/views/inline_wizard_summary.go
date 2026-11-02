package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// Wizard completion and summary screen.

func (w *InlineWizardView) wizardComplete() {
	w.completed = true
	w.renderSummaryScreen()
	w.fireComplete()
}

func (w *InlineWizardView) fireComplete() {
	if w.cfg.OnComplete != nil {
		w.cfg.OnComplete(w.completed && !w.aborted, w.wizardErr)
	}
}

// summaryTarget returns the target view ID and label for the summary screen,
// preferring the Func callbacks (evaluated at render time) over static fields.
func (w *InlineWizardView) summaryTarget() (viewID, label string) {
	if w.cfg.SummaryTargetViewFunc != nil {
		viewID = w.cfg.SummaryTargetViewFunc()
	} else {
		viewID = w.cfg.SummaryTargetView
	}
	if w.cfg.SummaryTargetLabelFunc != nil {
		label = w.cfg.SummaryTargetLabelFunc()
	} else {
		label = w.cfg.SummaryTargetLabel
	}
	return
}

func (w *InlineWizardView) renderSummaryScreen() {
	if w.stepContentInner == nil {
		return
	}

	// Mark all remaining as done in the step bar
	for i := range w.stepStates {
		if w.stepStates[i].Status == widgets.StepActive {
			w.syncStepBar(i, widgets.StepDone)
		}
	}

	// Update header
	w.stepHeader.SetText(fmt.Sprintf("  %s%s %s[-]",
		theme.ColorTagOf(theme.Success), theme.IconSuccess,
		i18n.T("wizard.complete")))

	// Build summary content
	w.stepContentInner.Clear()

	// ── Recap text ──
	summary := tview.NewTextView().SetDynamicColors(true)
	summary.SetBackgroundColor(theme.BgPanel)

	var b strings.Builder
	b.WriteString("\n")

	for _, si := range w.infoAccum {
		fmt.Fprintf(&b, "  %s%s[-] %s%s[-]\n",
			theme.ColorTagOf(theme.Success), theme.IconDone,
			theme.ColorTagOf(theme.FgPrimary), si.label)
		for _, f := range si.fields {
			if f.Value != "" {
				if strings.Contains(f.Value, "\n") {
					// Multi-line value: label on its own line, content indented below.
					fmt.Fprintf(&b, "      %s%s:[-]\n",
						theme.ColorTagOf(theme.FgSecondary), f.Label)
					for _, line := range strings.Split(f.Value, "\n") {
						fmt.Fprintf(&b, "        %s\n", line)
					}
				} else {
					fmt.Fprintf(&b, "      %s%s:[-] %s\n",
						theme.ColorTagOf(theme.FgSecondary), f.Label, f.Value)
				}
			}
		}
		b.WriteString("\n")
	}

	// Show skipped steps so the user knows what was not configured.
	for i, ss := range w.stepStates {
		if ss.Status != widgets.StepSkipped {
			continue
		}
		step := w.cfg.Steps[i]
		if step.SidebarHidden {
			continue
		}
		fmt.Fprintf(&b, "  %s%s %s[-]\n",
			theme.ColorTagOf(theme.FgMuted), theme.IconSkipped, step.Label)
	}

	if w.cfg.SummaryNote != nil {
		if note := w.cfg.SummaryNote(); note != "" {
			for _, line := range strings.Split(note, "\n") {
				fmt.Fprintf(&b, "  %s%s[-]\n", theme.ColorTagOf(theme.FgSecondary), line)
			}
		}
	}

	summary.SetText(b.String())
	w.stepContentInner.AddItem(summary, 0, 1, false)

	// ── Action button ──
	targetView, targetLabel := w.summaryTarget()
	buttonLabel := targetLabel
	if buttonLabel == "" {
		buttonLabel = i18n.T("wizard.summary.start")
	}
	summaryForm := NewStyledButtonForm()

	summaryForm.AddButton(buttonLabel, func() {
		switch {
		case w.shell == nil:
		case w.cfg.SummaryOnAction != nil:
			w.cfg.SummaryOnAction(w.shell)
		case targetView != "":
			w.shell.NavigateTo(targetView)
		default:
			w.shell.PopView()
		}
	})

	w.stepContentInner.AddItem(summaryForm, 3, 0, true)
	w.app.SetFocus(summaryForm)

	// Update hints
	w.hintsBar.SetHints("enter " + i18n.T("wizard.hint.submit") + " · ctrl+b " + i18n.T("wizard.hint.back"))

	// Update info panel to show final state
	w.renderInfoPanel()
}

func (w *InlineWizardView) handleSummaryKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter, tcell.KeyTab, tcell.KeyBacktab:
		// Let tview deliver to the focused form button.
		return event
	case tcell.KeyEscape:
		// The wizard is done: back to the previous view.
		if w.shell != nil {
			w.shell.PopView()
		}
		return nil
	case tcell.KeyCtrlB, tcell.KeyPgUp:
		// Allow going back from summary to review/edit the last step.
		w.completed = false
		w.wizardErr = nil
		// Find the last completed step to return to.
		lastDone := -1
		for i := len(w.stepStates) - 1; i >= 0; i-- {
			if w.stepStates[i].Status == widgets.StepDone && !w.cfg.Steps[i].SidebarHidden {
				lastDone = i
				break
			}
		}
		if lastDone >= 0 {
			w.syncStepBar(lastDone, widgets.StepActive)
			w.currentStep = lastDone
			if len(w.infoAccum) > 0 {
				w.infoAccum = w.infoAccum[:len(w.infoAccum)-1]
			}
			w.renderInfoPanel()
			w.doRenderStep(lastDone)
		}
		return nil
	}
	// Block everything else (Esc, runes, etc.).
	return nil
}
