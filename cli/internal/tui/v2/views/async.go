package views

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// loadSlot numbers the loads of a view: only the latest one may be applied.
// A load starts a new generation (the previous in-flight result is dropped)
// and Unmount invalidates the one in flight. The zero value is ready.
type loadSlot struct{ gen atomic.Uint64 }

// Invalidate drops the load in flight (call it in Unmount).
func (s *loadSlot) Invalidate() { s.gen.Add(1) }

// load runs fetch off the event loop and applies its result on it.
//
// Whatever fetch needs from the view (mode, period, selection…) must be
// captured on the loop before the call: fetch never touches the view. A
// panic in fetch is recovered (logged) and given to apply as an error.
// apply runs on the loop (QueueUpdateDraw), and only while this load is the
// latest of the slot and ctx is alive; otherwise the result is dropped.
//
// With a nil app (tests without an event loop) fetch and apply run inline.
func load[T any](app *tview.Application, ctx context.Context, slot *loadSlot, fetch func(context.Context) (T, error), apply func(T, error)) {
	if ctx == nil {
		ctx = context.Background()
	}
	gen := slot.gen.Add(1)
	current := func() bool { return slot.gen.Load() == gen && ctx.Err() == nil }
	if app == nil {
		v, err := safeFetch(ctx, fetch)
		if current() {
			apply(v, err)
		}
		return
	}
	safego.Go(func() {
		v, err := safeFetch(ctx, fetch)
		if !current() {
			return
		}
		app.QueueUpdateDraw(func() {
			if current() {
				apply(v, err)
			}
		})
	})
}

// safeFetch calls fetch and turns a panic into an error.
func safeFetch[T any](ctx context.Context, fetch func(context.Context) (T, error)) (v T, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("recovered panic in view load", "panic", r, "stack", string(debug.Stack()))
			var zero T
			v, err = zero, fmt.Errorf("panic: %v", r)
		}
	}()
	return fetch(ctx)
}

// syncLoad pulls the team-state of repo (syncAsync: the slow-pull toast, the
// failure warning), then reads with fetch off the event loop and applies the
// result on it, as load does. The latest request of the slot wins: a newer
// one, or an Unmount, drops this one at any step.
func syncLoad[T any](app *tview.Application, ctx context.Context, slot *loadSlot, repo teamstate.TeamStateWriter, shell ShellAccess, fetch func(context.Context) (T, error), apply func(T, error)) {
	gen := slot.gen.Add(1)
	syncAsync(app, repo, shell, func(error) {
		if slot.gen.Load() != gen {
			return
		}
		load(app, ctx, slot, fetch, apply)
	})
}

// everyOnLoop calls fn on the event loop of app every rate until done is
// closed. fn is where a view starts a read (load): the timer itself reads
// nothing.
func everyOnLoop(app *tview.Application, rate time.Duration, done <-chan struct{}, fn func()) {
	safego.Go(func() {
		ticker := time.NewTicker(rate)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				app.QueueUpdate(fn)
			}
		}
	})
}

// persist runs a write (work) off the event loop and gives its result to
// done on it. A write is an action, not a read: unlike load it is neither
// dropped when a newer one starts nor by an Unmount, since the user asked for
// it, and its context is not the one of the shell, which ends with the
// application (the write must finish). done runs even if the view was
// unmounted meanwhile and checks it before drawing. A panic in work is
// recovered (logged) and given to done as an error. With a nil app (tests
// without an event loop) work and done run inline.
func persist(app *tview.Application, work func(context.Context) error, done func(error)) {
	run := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered panic in view write", "panic", r, "stack", string(debug.Stack()))
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		return work(context.Background())
	}
	if app == nil {
		done(run())
		return
	}
	safego.Go(func() {
		err := run()
		app.QueueUpdateDraw(func() { done(err) })
	})
}
