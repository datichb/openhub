package widgets

import (
	"fmt"
	"sync"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/v2/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Spinner — animated activity indicator
// ─────────────────────────────────────────────────────────────────────────────

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner is an animated text view that shows a spinning indicator with a message.
// It is safe to call Start/Stop/SetMessage from any goroutine.
// Note: a Spinner can be started multiple times (after a Reset), but Stop is safe
// to call multiple times without panic.
type Spinner struct {
	*tview.TextView
	mu       sync.Mutex
	message  string
	frame    int
	stop     chan struct{}
	stopOnce sync.Once
	running  bool
}

// NewSpinner creates a spinner with the given message. Call Start() to begin animation.
func NewSpinner(message string) *Spinner {
	tv := tview.NewTextView().
		SetDynamicColors(true)
	tv.SetBackgroundColor(theme.BgPanel)

	s := &Spinner{
		TextView: tv,
		message:  message,
		stop:     make(chan struct{}),
	}
	s.renderFrame()
	return s
}

// Start begins the spinner animation. Must be called after the tview.Application
// is running. Pass the app for QueueUpdateDraw. Safe to call multiple times
// (no-op if already running).
func (s *Spinner) Start(app *tview.Application) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	// Reset stop channel and once for reuse
	s.stop = make(chan struct{})
	s.stopOnce = sync.Once{}
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				s.mu.Lock()
				s.frame = (s.frame + 1) % len(spinnerFrames)
				s.mu.Unlock()
				app.QueueUpdateDraw(func() {
					s.mu.Lock()
					s.renderFrame()
					s.mu.Unlock()
				})
			}
		}
	}()
}

// Stop halts the spinner animation. Safe to call multiple times (no panic).
func (s *Spinner) Stop() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		close(s.stop)
	})
}

// SetMessage updates the spinner message text. Safe to call from any goroutine.
func (s *Spinner) SetMessage(msg string) {
	s.mu.Lock()
	s.message = msg
	s.renderFrame()
	s.mu.Unlock()
}

// renderFrame updates the text view with the current frame and message.
// Caller must hold s.mu.
func (s *Spinner) renderFrame() {
	frame := spinnerFrames[s.frame]
	s.SetText(fmt.Sprintf("  %s%s[-] %s%s[-]",
		colorTag(theme.Accent), frame,
		colorTag(theme.FgSecondary), s.message))
}
