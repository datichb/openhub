package views

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The OnDone of a step started in one mount answers after the wizard was
// unmounted and mounted again: its result belongs to the old mount, and the
// new one keeps its step.
func TestInlineWizardOnDoneOfAPreviousMountIsDropped(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	first := formStep("First")
	first.OnDone = func() error {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return nil
	}
	v := NewInlineWizardView(InlineWizardConfig{
		ID:    "wizard.lifecycle.ondone",
		Steps: []WizardStep{first, formStep("Second")},
	})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() { v.completeStep(v.cfg.Steps[0]) })
	<-started
	onLoop(app, func() {
		v.Unmount()
		v.Mount(content, app) // mounted again while the first OnDone runs
	})
	close(release)
	// The answer of the old OnDone is queued on the loop: wait for it to run.
	time.Sleep(50 * time.Millisecond)
	onLoop(app, func() {
		assert.Equal(t, 0, v.currentStep, "the old result does not advance the new mount")
		assert.False(t, v.processing)
		assert.Nil(t, v.wizardErr)
	})
	onLoop(app, func() { v.Unmount() })
}

// The same answer, an error this time, does not show on the new mount.
func TestInlineWizardErrorOfAPreviousMountIsDropped(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	first := formStep("First")
	first.OnDone = func() error {
		close(started)
		<-release
		return assert.AnError
	}
	v := NewInlineWizardView(InlineWizardConfig{ID: "wizard.lifecycle.err", Steps: []WizardStep{first, formStep("Second")}})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() { v.completeStep(v.cfg.Steps[0]) })
	<-started
	onLoop(app, func() { v.Unmount(); v.Mount(content, app) })
	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(app, func() { assert.NoError(t, v.wizardErr) })
	onLoop(app, func() { v.Unmount() })
}

// A spinner timer that fires after the OnDone ended does not put the spinner
// over the next step.
func TestInlineWizardLateSpinnerDoesNotCoverTheNextStep(t *testing.T) {
	first := formStep("First")
	first.OnDone = func() error { return nil }
	v := NewInlineWizardView(InlineWizardConfig{ID: "wizard.lifecycle.late", Steps: []WizardStep{first, formStep("Second")}})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() {
		v.completeStep(v.cfg.Steps[0])
		time.Sleep(1200 * time.Millisecond) // the loop is held: the 1 s timer is queued behind the answer of OnDone
	})
	time.Sleep(100 * time.Millisecond)
	onLoop(app, func() {
		assert.Equal(t, 1, v.currentStep, "the wizard went on to the second step")
		assert.NotNil(t, v.form, "the second step shows its form, not the spinner")
	})
	onLoop(app, func() { v.Unmount() })
}

// The timers of a step problem and of a marked field read the application
// they were started with, not the field the loop clears at Unmount (-race).
func TestInlineWizardTimersDoNotReadTheAppOfAnUnmountedWizard(t *testing.T) {
	old := stepProblemHold
	stepProblemHold = time.Millisecond
	t.Cleanup(func() { stepProblemHold = old })
	step := formStep("First", "Name")
	v := NewInlineWizardView(InlineWizardConfig{ID: "wizard.lifecycle.timers", Steps: []WizardStep{step}})
	content := tview.NewFlex()
	app := runApp(t, content)
	for i := 0; i < 30; i++ {
		onLoop(app, func() {
			v.Mount(content, app)
			v.showStepProblem("a problem")
			v.markFirstInput(v.form)
		})
		time.Sleep(2 * time.Millisecond)
		onLoop(app, func() { v.Unmount() })
	}
}

// Notify may be called from any goroutine while the loop mounts and
// unmounts the wizard (-race).
func TestInlineWizardNotifyDuringMountAndUnmount(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{ID: "wizard.lifecycle.notify", Steps: []WizardStep{formStep("First")}})
	content := tview.NewFlex()
	app := runApp(t, content)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				v.Notify("hello", false)
				time.Sleep(2 * time.Millisecond)
			}
		}
	}()
	for i := 0; i < 10; i++ {
		onLoop(app, func() { v.Mount(content, app) })
		time.Sleep(time.Millisecond)
		onLoop(app, func() { v.Unmount() })
	}
	close(stop)
	<-done
	require.NotNil(t, v)
}
