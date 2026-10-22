package views

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync/atomic"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/safego"
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
