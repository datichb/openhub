package views

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// Tests for RunInlineWizardStandalone and standaloneShellStub
// ─────────────────────────────────────────────────────────────────────────────

func TestStandaloneShellStub_ImplementsShellAccess(t *testing.T) {
	var _ ShellAccess = (*standaloneShellStub)(nil)
}

func TestRunInlineWizardStandalone_EmptySteps(t *testing.T) {
	callbackFired := false
	wizard := NewInlineWizardView(InlineWizardConfig{
		ID:    "test.empty",
		Title: "Empty",
		Steps: []WizardStep{},
		OnComplete: func(ok bool, err error) {
			callbackFired = true
			assert.True(t, ok)
			assert.NoError(t, err)
		},
	})

	result := RunInlineWizardStandalone(wizard)
	assert.True(t, result.Completed)
	assert.False(t, result.Aborted)
	assert.NoError(t, result.Err)
	assert.True(t, callbackFired, "OnComplete should fire for empty wizard")
}

func TestRunInlineWizardStandalone_AllSkipped(t *testing.T) {
	completedOk := false
	wizard := NewInlineWizardView(InlineWizardConfig{
		ID:    "test.allskip",
		Title: "All Skipped",
		Steps: []WizardStep{
			{Label: "A", Skip: true},
			{Label: "B", SkipIf: func() bool { return true }},
		},
		OnComplete: func(ok bool, err error) {
			completedOk = ok
		},
	})

	result := RunInlineWizardStandalone(wizard)
	assert.True(t, result.Completed, "all-skipped wizard should complete")
	assert.True(t, completedOk, "OnComplete should report completed=true")
}

func TestRunInlineWizardStandalone_OnCompleteCallbackForwarded(t *testing.T) {
	// All-skipped path exercises the OnComplete forwarding without TTY.
	originalCalled := false

	wizard := NewInlineWizardView(InlineWizardConfig{
		ID:    "test.callback",
		Title: "Callback",
		Steps: []WizardStep{
			{Label: "Skipped", Skip: true},
		},
		OnComplete: func(ok bool, err error) {
			originalCalled = true
		},
	})

	RunInlineWizardStandalone(wizard)
	assert.True(t, originalCalled, "original OnComplete should be forwarded")
}

func TestRunInlineWizardStandalone_NilOnComplete(t *testing.T) {
	// Wizard with no OnComplete callback should not panic.
	wizard := NewInlineWizardView(InlineWizardConfig{
		ID:    "test.nil-callback",
		Title: "No callback",
		Steps: []WizardStep{
			{Label: "Skipped", Skip: true},
		},
	})

	require.NotPanics(t, func() {
		RunInlineWizardStandalone(wizard)
	})
}

func TestStandaloneShellStub_SafeMethods(t *testing.T) {
	app := tview.NewApplication()
	stub := &standaloneShellStub{app: app}

	// These methods should not panic.
	require.NotPanics(t, func() {
		stub.SetOmnibarVisible(false) // no-op
		stub.SetOmnibarVisible(true)  // no-op
		stub.PushView(nil)            // no-op
		_ = stub.Context()            // returns background context
	})
}

func TestStandaloneShellStub_UnsupportedMethodsPanic(t *testing.T) {
	stub := &standaloneShellStub{app: tview.NewApplication()}

	assert.Panics(t, func() { stub.ShowInputModal("", "", nil) })
	assert.Panics(t, func() { stub.ShowPasswordModal("", nil) })
	assert.Panics(t, func() { stub.ShowSelectModal("", nil, "", nil) })
	assert.Panics(t, func() { stub.ShowMultiSelectModal("", nil, nil, nil) })
	assert.Panics(t, func() { stub.ShowScrollableModal("", "", nil) })
	assert.Panics(t, func() { stub.ShowToastMsg("", false) })
	assert.Panics(t, func() { stub.ShowInlineForm(InlineFormConfig{}) })
	assert.Panics(t, func() { stub.SetProjectMode(nil) })
	assert.Panics(t, func() { stub.SetActiveProject(nil) })
	assert.Panics(t, func() { _ = stub.ActiveProject() })
	assert.Panics(t, func() { stub.SetMode("") })
	assert.Panics(t, func() { _ = stub.Mode() })
	assert.Panics(t, func() { stub.SetActiveTeam(nil) })
	assert.Panics(t, func() { _ = stub.ActiveTeam() })
}
