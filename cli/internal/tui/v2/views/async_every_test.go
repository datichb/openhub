package views

import (
	"context"
	"github.com/rivo/tview"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The ticks stop with the view (done) and with the shell (ctx): once either
// is over, nothing more is queued on an event loop that may be gone.
func TestTickOnLoopStopsWithTheViewOrTheShell(t *testing.T) {
	for name, stop := range map[string]func(cancel context.CancelFunc, done chan struct{}){
		"the view is left":   func(_ context.CancelFunc, done chan struct{}) { close(done) },
		"the shell is ended": func(cancel context.CancelFunc, _ chan struct{}) { cancel() },
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			var queued atomic.Int32
			ended := make(chan struct{})
			go func() {
				defer close(ended)
				tickOnLoop(ctx, time.Millisecond, done, func(fn func()) { queued.Add(1); fn() }, func() {})
			}()
			require.Eventually(t, func() bool { return queued.Load() >= 2 }, 5*time.Second, time.Millisecond, "ticks are queued while the view lives")
			stop(cancel, done)
			select {
			case <-ended:
			case <-time.After(5 * time.Second):
				t.Fatal("the loop goes on after the end")
			}
			n := queued.Load()
			time.Sleep(20 * time.Millisecond)
			assert.Equal(t, n, queued.Load(), "nothing queued after the end")
		})
	}
}

// A shell already ended queues nothing at all, even when a tick is ready.
func TestTickOnLoopWithAnEndedShellQueuesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var queued atomic.Int32
	tickOnLoop(ctx, time.Millisecond, make(chan struct{}), func(func()) { queued.Add(1) }, func() {})
	assert.Zero(t, queued.Load())
}

// The end of an action queues its result on the loop only while the shell lives.
func TestQueueIfLiveSkipsAnEndedShell(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	queueIfLive(ctx, tview.NewApplication(), func() { t.Fatal("queued after the end") }) // would block for ever if it queued
	queueIfLive(context.Background(), nil, func() { t.Fatal("no application") })

	app := runApp(t, tview.NewFlex())
	ran := make(chan struct{})
	go queueIfLive(context.Background(), app, func() { close(ran) })
	select {
	case <-ran:
	case <-time.After(3 * time.Second):
		t.Fatal("not queued while the shell lives")
	}
}
