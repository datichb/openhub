package views

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
)

type toastShell struct {
	mockShell
	toasts []string
}

func (s *toastShell) ShowToastMsg(msg string, _ bool) { s.toasts = append(s.toasts, msg) }

// While a step runs, Ctrl+B and Esc wait for it and say how to leave: going
// back would move the wizard under the step that is still running.
func TestInlineWizardKeysWaitForARunningStep(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	running := WizardStep{Label: "Work", OnDone: func() error { <-release; return nil }}
	v := mountWizard(t, InlineWizardConfig{ID: "wizard.render.running", Steps: []WizardStep{formStep("First"), running, formStep("After")}})
	sh := &toastShell{}
	v.SetShell(sh)
	v.advanceAfterDone(v.cfg.Steps[0])
	v.renderStep(v.currentStep)
	require.Equal(t, 1, v.currentStep)
	require.True(t, v.processing)

	for _, ev := range []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt),
		tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
	} {
		assert.Nil(t, v.HandleKey(ev), "the key is taken")
		assert.Equal(t, 1, v.currentStep, "the wizard stays on the running step")
		assert.Nil(t, v.quitBar, "no quit prompt")
	}
	require.Len(t, sh.toasts, 4, "each key answers")
	assert.Equal(t, i18n.T("tui.modal.hints.processing"), sh.toasts[0])
}

// The summary goes back on every back key, Alt+← included.
func TestInlineWizardSummaryGoesBackOnAltLeft(t *testing.T) {
	v := mountWizard(t, InlineWizardConfig{ID: "wizard.render.summary.back", Steps: []WizardStep{formStep("One")}})
	v.advanceAfterDone(v.cfg.Steps[0])
	v.renderStep(v.currentStep)
	require.True(t, v.completed)
	assert.Nil(t, v.HandleKey(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModAlt)))
	assert.False(t, v.completed, "Alt+← went back from the summary")
}
