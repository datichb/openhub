package widgets

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Spinner — animated activity indicator
// ─────────────────────────────────────────────────────────────────────────────

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner is an animated text view that shows a spinning indicator with a message.
// It is safe to call Start/Stop/SetMessage from any goroutine.
// A Spinner can be started again after a Stop, and Stop is safe to call
// multiple times (and before Start).
type Spinner struct {
	*tview.TextView
	mu      sync.Mutex
	message string
	frame   int
	// stop is closed by Stop and nil while the spinner is not running. Each
	// animation reads the channel it was started with: a Stop followed at
	// once by a Start does not leave the first animation running on the
	// channel of the second.
	stop  chan struct{}
	loops atomic.Int32 // running animations (tests)
}

// NewSpinner creates a spinner with the given message. Call Start() to begin animation.
func NewSpinner(message string) *Spinner {
	tv := tview.NewTextView().
		SetDynamicColors(true)
	tv.SetBackgroundColor(theme.BgPanel)

	s := &Spinner{
		TextView: tv,
		message:  message,
	}
	s.renderFrame()
	return s
}

// Start begins the spinner animation. Must be called after the tview.Application
// is running. Pass the app for QueueUpdateDraw. Safe to call multiple times
// (no-op if already running).
func (s *Spinner) Start(app *tview.Application) {
	s.mu.Lock()
	if s.stop != nil {
		s.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	s.stop = stop
	s.loops.Add(1)
	s.mu.Unlock()

	safego.Go(func() {
		defer s.loops.Add(-1)
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				// Stopped while the tick was ready: QueueUpdateDraw waits for
				// the event loop, which may be gone.
				if stopped(stop) {
					return
				}
				s.mu.Lock()
				s.frame = (s.frame + 1) % len(spinnerFrames)
				s.mu.Unlock()
				app.QueueUpdateDraw(func() {
					if stopped(stop) {
						return
					}
					s.mu.Lock()
					s.renderFrame()
					s.mu.Unlock()
				})
			}
		}
	})
}

func stopped(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

// Stop halts the spinner animation. Safe to call multiple times (no panic).
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stop != nil {
		close(s.stop)
		s.stop = nil
	}
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
		theme.ColorTagOf(theme.ActiveMode.Primary), frame,
		theme.ColorTagOf(theme.FgSecondary), s.message))
}
