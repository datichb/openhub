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
		if !w.shouldSkip(i) && !w.cfg.Steps[i].BackSkip && (w.stepStates[i].Status == widgets.StepDone || w.stepStates[i].Status == widgets.StepActive) {
			return i
		}
	}
	return -1
}

func (w *InlineWizardView) skipCurrent() {
	// Skipping a step that failed forgets its error: the wizard ends with
	// OnComplete(true, nil), not with the error of a step the user left.
	w.wizardErr = nil
	w.syncStepBar(w.currentStep, widgets.StepSkipped)
	if on := w.cfg.Steps[w.currentStep].OnSkip; on != nil {
		on()
	}

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
	// Reset current to pending, and the steps passed over (BackSkip)
	w.syncStepBar(w.currentStep, widgets.StepPending)
	for i := prev + 1; i < w.currentStep; i++ {
		if w.cfg.Steps[i].BackSkip && w.stepStates[i].Status == widgets.StepDone {
			w.syncStepBar(i, widgets.StepPending)
		}
	}

	// Reset previous to active
	w.syncStepBar(prev, widgets.StepActive)

	// Drop the info of the previous step and of the steps passed over
	kept := w.infoAccum[:0]
	for _, e := range w.infoAccum {
		if e.stepIdx < prev {
			kept = append(kept, e)
		}
	}
	w.infoAccum = kept
	// Going back leaves the error of a step that failed behind it.
	w.wizardErr = nil
	w.currentStep = prev
	w.renderInfoPanel()
	w.doRenderStep(prev)
}

// goToStep goes back to the step of that ID: the steps after it, the current
// one included, are pending again and their info is dropped. It reports false
// when no earlier step has that ID.
func (w *InlineWizardView) goToStep(id string) bool {
	target := -1
	for i := 0; i <= w.currentStep && i < len(w.cfg.Steps); i++ {
		if w.cfg.Steps[i].ID == id {
			target = i
			break
		}
	}
	if target == -1 {
		return false
	}
	for i := target + 1; i <= w.currentStep; i++ {
		w.syncStepBar(i, widgets.StepPending)
	}
	w.syncStepBar(target, widgets.StepActive)
	kept := w.infoAccum[:0]
	for _, e := range w.infoAccum {
		if e.stepIdx < target {
			kept = append(kept, e)
		}
	}
	w.infoAccum = kept
	w.wizardErr = nil
	w.currentStep = target
	w.renderInfoPanel()
	w.doRenderStep(target)
	return true
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
