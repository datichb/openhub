package views

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type loaded struct {
	mu   sync.Mutex
	vals []string
	errs []error
}

func (l *loaded) apply(v string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.vals, l.errs = append(l.vals, v), append(l.errs, err)
}

func (l *loaded) snapshot() ([]string, []error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.vals...), append([]error(nil), l.errs...)
}

// drain waits for the event loop to have run what was queued before.
func drain(app *tview.Application) { onLoop(app, func() {}) }

// A result that comes back after Unmount and a new Mount is ignored; the one
// of the new mount is applied.
func TestLoad_StaleResultAfterRemountIsIgnored(t *testing.T) {
	app := runApp(t, tview.NewFlex())
	var slot loadSlot
	var l loaded

	release := make(chan struct{})
	started := make(chan struct{})
	load(app, context.Background(), &slot, func(context.Context) (string, error) {
		close(started)
		<-release
		return "old", nil
	}, l.apply)
	<-started

	slot.Invalidate() // Unmount
	load(app, context.Background(), &slot, func(context.Context) (string, error) { return "new", nil }, l.apply)

	require.Eventually(t, func() bool { v, _ := l.snapshot(); return len(v) == 1 }, 2*time.Second, 5*time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	drain(app)

	vals, _ := l.snapshot()
	assert.Equal(t, []string{"new"}, vals, "the old result is dropped")
}

// Unmount alone is enough: nothing is applied afterwards.
func TestLoad_InvalidateDropsTheResult(t *testing.T) {
	app := runApp(t, tview.NewFlex())
	var slot loadSlot
	var l loaded
	release, done := make(chan struct{}), make(chan struct{})
	load(app, context.Background(), &slot, func(context.Context) (string, error) {
		<-release
		defer close(done)
		return "late", nil
	}, l.apply)
	slot.Invalidate()
	close(release)
	<-done
	time.Sleep(50 * time.Millisecond)
	drain(app)
	vals, _ := l.snapshot()
	assert.Empty(t, vals)
}

// Two loads in a row (two quick `r`): the last one wins, even when the first
// answers after it.
func TestLoad_LatestLoadWins(t *testing.T) {
	app := runApp(t, tview.NewFlex())
	var slot loadSlot
	var l loaded
	release, started := make(chan struct{}), make(chan struct{})
	load(app, context.Background(), &slot, func(context.Context) (string, error) {
		close(started)
		<-release
		return "first", nil
	}, l.apply)
	<-started
	load(app, context.Background(), &slot, func(context.Context) (string, error) { return "second", nil }, l.apply)
	require.Eventually(t, func() bool { v, _ := l.snapshot(); return len(v) == 1 }, 2*time.Second, 5*time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	drain(app)
	vals, _ := l.snapshot()
	assert.Equal(t, []string{"second"}, vals)
}

// A cancelled context drops the result too.
func TestLoad_CancelledContextDropsTheResult(t *testing.T) {
	app := runApp(t, tview.NewFlex())
	var slot loadSlot
	var l loaded
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	load(app, ctx, &slot, func(context.Context) (string, error) {
		cancel()
		defer close(done)
		return "x", nil
	}, l.apply)
	<-done
	time.Sleep(50 * time.Millisecond)
	drain(app)
	vals, _ := l.snapshot()
	assert.Empty(t, vals)
}

// An error of fetch reaches apply, which runs on the event loop.
func TestLoad_ErrorReachesApply(t *testing.T) {
	app := runApp(t, tview.NewFlex())
	var slot loadSlot
	var l loaded
	boom := errors.New("boom")
	onLoop(app, func() {
		load(app, context.Background(), &slot, func(context.Context) (string, error) { return "partial", boom }, func(v string, err error) {
			l.apply(v, err)
		})
	})
	require.Eventually(t, func() bool { v, _ := l.snapshot(); return len(v) == 1 }, 2*time.Second, 5*time.Millisecond)
	vals, errs := l.snapshot()
	assert.Equal(t, []string{"partial"}, vals)
	assert.ErrorIs(t, errs[0], boom)
}

// A panic of fetch is recovered and reported to apply as an error: the view
// leaves its loading state and the process lives.
func TestLoad_PanicInFetchIsRecovered(t *testing.T) {
	app := runApp(t, tview.NewFlex())
	var slot loadSlot
	var l loaded
	load(app, context.Background(), &slot, func(context.Context) (string, error) { panic("kaboom") }, l.apply)
	require.Eventually(t, func() bool { v, _ := l.snapshot(); return len(v) == 1 }, 2*time.Second, 5*time.Millisecond)
	vals, errs := l.snapshot()
	assert.Equal(t, []string{""}, vals)
	require.Error(t, errs[0])
	assert.Contains(t, errs[0].Error(), "kaboom")
}

// apply runs on the event loop, fetch does not.
func TestLoad_FetchOffTheLoopApplyOnIt(t *testing.T) {
	app := runApp(t, tview.NewFlex())
	var slot loadSlot
	// The loop is blocked while fetch has to finish: it would deadlock if
	// fetch ran on the loop.
	block, fetched := make(chan struct{}), make(chan struct{})
	applied := make(chan struct{})
	app.QueueUpdate(func() {
		load(app, context.Background(), &slot, func(context.Context) (int, error) {
			close(fetched)
			return 1, nil
		}, func(int, error) { close(applied) })
		<-fetched
		close(block)
	})
	<-block
	select {
	case <-applied:
	case <-time.After(2 * time.Second):
		t.Fatal("apply not run")
	}
}

// Without an application (tests without a loop) everything runs inline.
func TestLoad_NilAppRunsInline(t *testing.T) {
	var slot loadSlot
	var l loaded
	load(nil, context.Background(), &slot, func(context.Context) (string, error) { return "inline", nil }, l.apply)
	vals, _ := l.snapshot()
	assert.Equal(t, []string{"inline"}, vals)
}
