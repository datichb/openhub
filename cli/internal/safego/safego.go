// Package safego provides panic-safe goroutine launchers.
//
// All goroutines spawned via Go() are wrapped in a deferred recover that logs
// the panic with a full stack trace and writes the crash report
// (internal/crashlog, the same file as the panic of the main goroutine). This prevents a single panicking goroutine
// from silently dying (background refresh, network call, git operation) and
// leaving the application in an inconsistent state with no diagnostic output.
//
// Supervise() is for the loops a process cannot live without (the session
// watchers of the daemon, its periodic jobs): after a panic the loop is run
// again, after a pause that grows while it keeps failing.
package safego

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/datichb/openhub/cli/internal/crashlog"
)

// Go launches fn in a new goroutine with panic recovery.
// If fn panics, the panic is recovered, logged via slog.Error with a full
// stack trace and written to a crash file (internal/crashlog: the log shows
// the path, so does the toast of the TUI), and the goroutine exits cleanly.
// The rest of the application continues running.
func Go(fn func()) {
	go Wrap(fn)()
}

// Wrap returns fn with the recovery of Go, for the launchers that start the
// goroutine themselves: sync.WaitGroup.Go(safego.Wrap(fn)).
func Wrap(fn func()) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil {
				logPanic("recovered panic in goroutine", r)
			}
		}()
		fn()
	}
}

// PanicError is the error a recovered panic becomes (WrapErr).
type PanicError struct {
	Value     any    // the value given to panic
	Stack     []byte // the stack, read in the recover
	CrashFile string // the crash report, empty when it could not be written
}

func (e *PanicError) Error() string {
	msg := fmt.Sprintf("panic: %v", e.Value)
	if e.CrashFile != "" {
		msg += " (crash report: " + e.CrashFile + ")"
	}
	return msg + "\n" + string(e.Stack)
}

// Unwrap lets errors.Is and errors.As see an error given to panic.
func (e *PanicError) Unwrap() error {
	err, _ := e.Value.(error)
	return err
}

// WrapErr returns fn with its panic recovered and returned as a *PanicError
// (value, stack and crash report), for the launchers that collect an error:
// errgroup.Group.Go(safego.WrapErr(fn)). The panic is neither logged nor
// shown: the caller of Wait gets it as any other error, and an error is not
// to be swallowed, so it must handle it. The crash file is written all the
// same, the stack of the error being a text the user does not read.
func WrapErr(fn func() error) func() error {
	return func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				perr := &PanicError{Value: r, Stack: stack}
				if path, werr := crashlog.Write(r, stack, time.Now()); werr == nil {
					perr.CrashFile = path
				}
				err = perr
			}
		}()
		return fn()
	}
}

// logPanic logs a recovered panic (message, value, crash file, stack). The
// stack comes last: the toast of the TUI keeps the first attributes and
// leaves the stack out.
func logPanic(msg string, r any, attrs ...any) {
	stack := debug.Stack()
	args := append([]any{}, attrs...)
	args = append(args, "panic", r)
	if path, err := crashlog.Write(r, stack, time.Now()); err == nil {
		args = append(args, "crash_file", path)
	} else if !errors.Is(err, crashlog.ErrDisabled) {
		args = append(args, "crash_file_error", err.Error())
	}
	args = append(args, "stack", string(stack))
	slog.Error(msg, args...)
}

// AfterFunc is time.AfterFunc with the recovery of Go: fn runs in a goroutine
// of its own after d, and a panic is logged instead of ending the process. The
// returned timer stops it as time.AfterFunc's does.
func AfterFunc(d time.Duration, fn func()) *time.Timer {
	return time.AfterFunc(d, func() { Go(fn) })
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
			logPanic("recovered panic in supervised loop", r, "loop", name)
		}
	}()
	fn(ctx)
	return false
}
