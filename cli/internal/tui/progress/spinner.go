// Package progress provides a lightweight CLI spinner for long-running operations.
// It writes to stderr so it does not pollute piped stdout output.
// Usage:
//
//	s := progress.NewSpinner("Création du worktree...")
//	s.Start()
//	defer s.Stop()
//	// ... do work ...
//
// Or with a scoped helper:
//
//	err := progress.Run("Téléchargement...", func() error {
//	    return doSomethingSlow()
//	})
package progress

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// frames is the braille spinner animation sequence (matches the TUI widget).
var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const tickRate = 80 * time.Millisecond

// Spinner is a terminal spinner that animates on stderr while work is in progress.
type Spinner struct {
	mu      sync.Mutex
	msg     string
	running bool
	done    chan struct{}
}

// NewSpinner creates a new Spinner with the given message.
func NewSpinner(msg string) *Spinner {
	return &Spinner{msg: msg}
}

// SetMessage updates the spinner label (safe to call while running).
func (s *Spinner) SetMessage(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.mu.Unlock()
}

// Start begins the spinner animation in a background goroutine.
// It is a no-op if the terminal is not interactive (e.g. piped output).
func (s *Spinner) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.done = make(chan struct{})
	s.mu.Unlock()

	go func() {
		i := 0
		for {
			select {
			case <-s.done:
				// Clear the spinner line before returning.
				fmt.Fprint(os.Stderr, "\r\033[K")
				return
			case <-time.After(tickRate):
				s.mu.Lock()
				msg := s.msg
				s.mu.Unlock()
				fmt.Fprintf(os.Stderr, "\r  %s  %s", frames[i%len(frames)], msg)
				i++
			}
		}
	}()
}

// Stop halts the spinner and clears the line. Safe to call multiple times.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.done)
	// Brief pause so the goroutine has time to clear the line.
	time.Sleep(tickRate)
}

// Run is a convenience wrapper that starts a spinner, runs fn, stops the spinner
// and returns fn's error. The spinner is always stopped even if fn panics.
func Run(msg string, fn func() error) error {
	s := NewSpinner(msg)
	s.Start()
	defer s.Stop()
	return fn()
}
