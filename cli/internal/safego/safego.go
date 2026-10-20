// Package safego provides panic-safe goroutine launchers.
//
// All goroutines spawned via Go() are wrapped in a deferred recover that logs
// the panic with a full stack trace. This prevents a single panicking goroutine
// from silently dying (background refresh, network call, git operation) and
// leaving the application in an inconsistent state with no diagnostic output.
//
// Supervise() is for the loops a process cannot live without (the session
// watchers of the daemon, its periodic jobs): after a panic the loop is run
// again, after a pause that grows while it keeps failing.
package safego

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"
)

// Go launches fn in a new goroutine with panic recovery.
// If fn panics, the panic is recovered, logged via slog.Error with a full
// stack trace, and the goroutine exits cleanly. The rest of the application
// continues running.
func Go(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered panic in goroutine",
					"panic", r,
					"stack", string(debug.Stack()),
				)
			}
		}()
		fn()
	}()
}

// Default pauses between two runs of a supervised loop.
const (
	DefaultRestartMin = time.Second
	DefaultRestartMax = 30 * time.Second
)

// healthyRun is how long a run must last for the pause to start over: a
// loop that ran that long was working, its panic is not a crash loop.
const healthyRun = time.Minute

// Supervise runs fn in the calling goroutine until it returns normally or
// ctx is done. When fn panics, the panic is recovered and logged (with the
// name of the loop and a full stack trace), then fn is run again after a
// pause: DefaultRestartMin, doubled at each panic in a row up to
// DefaultRestartMax. The caller picks the goroutine: safego.Go(func() {
// safego.Supervise(...) }) when it must not block.
//
// A restarted loop starts from scratch: fn must rebuild its state (the
// watchers resynchronize at each connection).
func Supervise(ctx context.Context, name string, fn func(ctx context.Context)) {
	SuperviseBackoff(ctx, name, DefaultRestartMin, DefaultRestartMax, fn)
}

// SuperviseBackoff is Supervise with the bounds of the pause between runs.
func SuperviseBackoff(ctx context.Context, name string, lo, hi time.Duration, fn func(ctx context.Context)) {
	pause := lo
	for ctx.Err() == nil {
		start := time.Now()
		if !run(ctx, name, fn) {
			return
		}
		if time.Since(start) >= healthyRun {
			pause = lo
		}
		slog.Warn("restarting after a panic", "loop", name, "pause", pause)
		t := time.NewTimer(pause)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
		pause = min(2*pause, hi)
	}
}

// run calls fn and reports whether it panicked.
func run(ctx context.Context, name string, fn func(ctx context.Context)) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			slog.Error("recovered panic in supervised loop",
				"loop", name,
				"panic", r,
				"stack", string(debug.Stack()),
			)
		}
	}()
	fn(ctx)
	return false
}
