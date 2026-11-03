package widgets

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runningApp runs a tview application on a simulation screen until the test
// ends.
func runningApp(t *testing.T) *tview.Application {
	t.Helper()
	sc := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sc.Init())
	sc.SetSize(40, 10)
	app := tview.NewApplication().SetScreen(sc)
	app.SetRoot(tview.NewBox(), true)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = app.Run()
	}()
	t.Cleanup(func() {
		app.Stop()
		<-done
	})
	// The loop answers.
	app.QueueUpdate(func() {})
	return app
}

// A Stop followed at once by a Start leaves one animation, not two: the
// animation that was inside QueueUpdateDraw when Stop came reads the channel
// it was started with, not the one of the next Start.
func TestSpinnerStopThenStartRunsOneAnimation(t *testing.T) {
	app := runningApp(t)
	s := NewSpinner("working")
	s.Start(app)
	require.Eventually(t, func() bool { return s.loops.Load() == 1 }, time.Second, time.Millisecond)

	// Hold the event loop long enough for the animation to tick and block in
	// QueueUpdateDraw, then stop and start the spinner from the loop.
	app.QueueUpdate(func() {
		time.Sleep(200 * time.Millisecond)
		s.Stop()
		s.Start(app)
	})
	require.Eventually(t, func() bool { return s.loops.Load() == 1 }, 2*time.Second, 5*time.Millisecond,
		"the first animation ends, the second one runs")
	assert.Never(t, func() bool { return s.loops.Load() != 1 }, 300*time.Millisecond, 10*time.Millisecond)

	s.Stop()
	require.Eventually(t, func() bool { return s.loops.Load() == 0 }, time.Second, 5*time.Millisecond)
}

func TestSpinnerStopIsSafeBeforeStartAndTwice(t *testing.T) {
	app := runningApp(t)
	s := NewSpinner("working")
	s.Stop()
	s.Start(app)
	s.Start(app) // already running: no second animation
	require.Eventually(t, func() bool { return s.loops.Load() == 1 }, time.Second, time.Millisecond)
	s.Stop()
	s.Stop()
	require.Eventually(t, func() bool { return s.loops.Load() == 0 }, time.Second, 5*time.Millisecond)
}
