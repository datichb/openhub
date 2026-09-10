package views

import (
	"sync"
	"time"

	"github.com/rivo/tview"
)

// AutoSaver provides debounced auto-save for configuration views.
// After each mutation, call Schedule() to reset the debounce timer.
// When the timer fires, saveFn is invoked via app.QueueUpdateDraw
// to ensure thread-safe TUI updates.
//
// Usage:
//
//	as := NewAutoSaver(500*time.Millisecond, app, func() { v.save() })
//	// In each field setter:
//	f.Set = func(val string) { /* mutate */ ; as.Schedule() }
//	// On view unmount:
//	as.Flush()  // save any pending changes immediately
//	as.Cancel() // stop the timer
type AutoSaver struct {
	mu      sync.Mutex
	timer   *time.Timer
	delay   time.Duration
	saveFn  func()
	app     *tview.Application
	pending bool
}

// NewAutoSaver creates an auto-saver with the given debounce delay.
// saveFn is the function that persists the current config state (e.g. config.Save, ProjectStore.Update).
// app is used for QueueUpdateDraw to ensure TUI-safe save execution.
func NewAutoSaver(delay time.Duration, app *tview.Application, saveFn func()) *AutoSaver {
	return &AutoSaver{
		delay:  delay,
		saveFn: saveFn,
		app:    app,
	}
}

// Schedule resets the debounce timer. If no further Schedule() calls happen
// within the delay window, saveFn is invoked automatically.
// Safe to call from any goroutine.
func (as *AutoSaver) Schedule() {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.pending = true
	if as.timer != nil {
		as.timer.Stop()
	}
	as.timer = time.AfterFunc(as.delay, func() {
		as.mu.Lock()
		pending := as.pending
		as.pending = false
		as.mu.Unlock()
		if !pending {
			return
		}
		if as.app != nil {
			as.app.QueueUpdateDraw(func() {
				as.saveFn()
			})
		} else {
			as.saveFn()
		}
	})
}

// Flush saves immediately if there are pending changes, cancelling any
// debounce timer. Call this on view unmount or navigation away.
func (as *AutoSaver) Flush() {
	as.mu.Lock()
	if as.timer != nil {
		as.timer.Stop()
		as.timer = nil
	}
	pending := as.pending
	as.pending = false
	as.mu.Unlock()

	if pending && as.saveFn != nil {
		as.saveFn()
	}
}

// Cancel stops any pending auto-save without executing it.
// Use this when discarding changes (e.g. undo to clean state).
func (as *AutoSaver) Cancel() {
	as.mu.Lock()
	defer as.mu.Unlock()
	if as.timer != nil {
		as.timer.Stop()
		as.timer = nil
	}
	as.pending = false
}

// IsPending reports whether there is a scheduled save that hasn't fired yet.
func (as *AutoSaver) IsPending() bool {
	as.mu.Lock()
	defer as.mu.Unlock()
	return as.pending
}

// SetApp updates the tview application reference. Useful when the app
// reference is not available at construction time.
func (as *AutoSaver) SetApp(app *tview.Application) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.app = app
}
