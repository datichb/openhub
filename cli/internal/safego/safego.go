// Package safego provides panic-safe goroutine launchers.
//
// All goroutines spawned via Go() are wrapped in a deferred recover that logs
// the panic with a full stack trace. This prevents a single panicking goroutine
// from silently dying (background refresh, network call, git operation) and
// leaving the application in an inconsistent state with no diagnostic output.
package safego

import (
	"log/slog"
	"runtime/debug"
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
