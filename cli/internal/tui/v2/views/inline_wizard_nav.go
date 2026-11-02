package views

import (
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// Step navigation of the inline wizard: which step is next or previous, skip,
// back, and what a finished step leaves in the info panel.

func (w *InlineWizardView) shouldSkip(idx int) bool {
	s := w.cfg.Steps[idx]
	if s.SkipIf != nil {
		return s.SkipIf()
	}
	return s.Skip
}

func (w *InlineWizardView) countVisibleSteps(idx int) (total, position int) {
	pos := 0
	for i := range w.cfg.Steps {
		if w.cfg.Steps[i].Skip {
			continue
		}
		if w.cfg.Steps[i].SkipIf != nil && w.cfg.Steps[i].SkipIf() {
			continue
		}
		total++
		if i < idx {
			pos++
		} else if i == idx {
			pos++
			position = pos
		}
	}
	if position == 0 {
		position = total
	}
	return total, position
}

func (w *InlineWizardView) findNext(from int) int {
	for i := from + 1; i < len(w.cfg.Steps); i++ {
		if !w.shouldSkip(i) && w.stepStates[i].Status == widgets.StepPending {
			return i
		}
	}
	return -1
}

func (w *InlineWizardView) findPrev(from int) int {
	for i := from - 1; i >= 0; i-- {
		if !w.shouldSkip(i) && (w.stepStates[i].Status == widgets.StepDone || w.stepStates[i].Status == widgets.StepActive) {
			return i
		}
	}
	return -1
}

func (w *InlineWizardView) skipCurrent() {
	w.syncStepBar(w.currentStep, widgets.StepSkipped)

	next := w.findNext(w.currentStep)
	if next == -1 {
		w.wizardComplete()
		return
	}
	w.currentStep = next
	w.syncStepBar(next, widgets.StepActive)
	w.renderInfoPanel()
}

func (w *InlineWizardView) goBack() {
	prev := w.findPrev(w.currentStep)
	if prev == -1 {
		return
	}
	// Reset current to pending
	w.syncStepBar(w.currentStep, widgets.StepPending)

	// Reset previous to active
	w.syncStepBar(prev, widgets.StepActive)

	// Pop last info entry
	if len(w.infoAccum) > 0 {
		w.infoAccum = w.infoAccum[:len(w.infoAccum)-1]
	}
	// Going back leaves the error of a step that failed behind it.
	w.wizardErr = nil
	w.currentStep = prev
	w.renderInfoPanel()
	w.doRenderStep(prev)
}

func (w *InlineWizardView) advanceAfterDone(step WizardStep) {
	// Collect info fields
	if step.InfoFields != nil {
		fields := step.InfoFields()
		w.infoAccum = append(w.infoAccum, inlineStepInfoEntry{stepIdx: w.currentStep, label: step.Label, fields: fields})
	} else {
		w.infoAccum = append(w.infoAccum, inlineStepInfoEntry{stepIdx: w.currentStep, label: step.Label, fields: nil})
	}

	w.syncStepBar(w.currentStep, widgets.StepDone)

	next := w.findNext(w.currentStep)
	if next == -1 {
		w.wizardComplete()
		return
	}
	w.currentStep = next
	w.syncStepBar(next, widgets.StepActive)
	w.renderInfoPanel()
}
