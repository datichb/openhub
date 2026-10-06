package shell

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ToastLevel defines the visual style of a toast notification.
type ToastLevel int

const (
	// ToastSuccess shows a success toast (Jade border, checkmark icon).
	ToastSuccess ToastLevel = iota
	// ToastError shows an error toast (Ruby border, cross icon).
	ToastError
	// ToastWarning shows a warning toast (Amber border, exclamation icon).
	ToastWarning
	// ToastInfo shows an info toast (Azure border, arrow icon).
	ToastInfo
)

// Toast duration constants — how long each level stays visible.
const (
	ToastDurationSuccess = 4 * time.Second  // was 2.5s
	ToastDurationInfo    = 4 * time.Second  // was 2.5s
	ToastDurationWarning = 8 * time.Second  // new
	ToastDurationError   = 10 * time.Second // was 6s
)

// ToastDurationForLevel returns the auto-dismiss duration for a given toast level.
func ToastDurationForLevel(level ToastLevel) time.Duration {
	switch level {
	case ToastError:
		return ToastDurationError
	case ToastWarning:
		return ToastDurationWarning
	case ToastSuccess:
		return ToastDurationSuccess
	default:
		return ToastDurationInfo
	}
}

// toastMaxLen returns the maximum display length for a toast message based on
// its severity level. Errors and warnings get more characters so diagnostics
// are legible without opening the Notifications view.
func toastMaxLen(level ToastLevel) int {
	switch level {
	case ToastError, ToastWarning:
		return 120 // was 80
	default:
		return 80 // was 50
	}
}

// showToast displays a toast notification. Thread-safe: can be called from any
// goroutine. When called from outside the tview event loop, widget mutations
// are scheduled via QueueUpdateDraw. When called from the event loop (key/mouse
// handlers, QueueUpdateDraw callbacks), widget mutations run inline.
//
// The message is always persisted to the in-memory notification store and the
// JSONL file, regardless of whether the visual toast is displayed.
func (s *Shell) showToast(msg string, level ToastLevel, duration time.Duration) {
	icon, borderColor := toastStyle(level)

	// Persist the FULL message before any truncation:
	// 1. In-memory store (current session, 50 entries max) — synchronous, safe.
	s.notifications.Add(level, msg)
	// 2. Persistent JSONL file — asynchronous.
	//    Pass msg as an explicit goroutine argument to capture its current VALUE,
	//    not the closure variable. Without this, the goroutine may read msg after
	//    it has been reassigned below to the display-truncated version.
	go func(fullMsg string) {
		_ = AppendToFile(NotificationsFilePath(), level, fullMsg)
	}(msg)

	// Truncate for display only — errors/warnings get more room.
	// NOTE: this modifies the local variable msg but does NOT affect the
	// already-persisted full message above.
	maxLen := toastMaxLen(level)
	runes := []rune(msg)
	if len(runes) > maxLen {
		msg = string(runes[:maxLen-3]) + "..."
	}

	// renderToast performs all tview widget mutations. MUST run on the event
	// loop — either called inline (from a handler / QueueUpdateDraw callback)
	// or scheduled via QueueUpdateDraw from a goroutine.
	renderToast := func() {
		toast := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter).
			SetText(fmt.Sprintf(" %s %s ", icon, msg))
		toast.SetBackgroundColor(theme.BgElement)
		toast.SetBorder(true)
		toast.SetBorderColor(borderColor)

		toastWidth := len([]rune(msg)) + 8
		if toastWidth < 20 {
			toastWidth = 20
		}
		if toastWidth > 120 {
			toastWidth = 120
		}
		toastHeight := 3

		// Position at top-right using a Grid, offset vertically by the number
		// of currently active toasts so they stack instead of overlapping.
		// Cap at 5 visible toasts to avoid overflowing off-screen on small terminals.
		if s.activeToasts >= 5 {
			return // silently drop — the message is already persisted to notifications
		}
		topRow := 1 + (s.activeToasts * (toastHeight + 1))
		grid := tview.NewGrid().
			SetColumns(0, toastWidth, 2).
			SetRows(topRow, toastHeight, 0)
		grid.AddItem(toast, 1, 1, 1, 1, 0, 0, false)
		// Defense-in-depth: if focus somehow lands on the toast grid, do not let
		// it consume any key events (arrow keys were causing the toast to "move"
		// because tview's Grid navigates between its cells on arrow presses).
		// Returning nil from InputCapture swallows the event at the widget level;
		// the global key handler (Shell.globalKeyHandler) has already processed
		// the event before tview delegates it to the focused primitive, so nothing
		// is lost.
		grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			return nil
		})

		pageName := fmt.Sprintf("toast-%d", time.Now().UnixNano())
		s.activeToasts++
		s.activeToastIDs = append(s.activeToastIDs, pageName)
		prev := s.app.GetFocus()
		s.pages.AddPage(pageName, grid, true, true)
		// Pages.AddPage re-delegates focus to the last visible page (the toast),
		// stealing it from whatever widget the user was interacting with.
		// Restore focus immediately.
		s.restoreFocusAfterToast(prev)

		// Auto-dismiss after duration
		time.AfterFunc(duration, func() {
			s.app.QueueUpdateDraw(func() {
				s.dismissToast(pageName)
			})
		})
	}

	// Schedule the rendering on the event loop. Using a goroutine wrapper
	// ensures QueueUpdateDraw is never called from inside the event loop
	// (which would deadlock on the unbuffered done-channel). When showToast
	// is called from a handler, the goroutine adds a negligible scheduling
	// delay but avoids the need to know caller context.
	go func() {
		s.app.QueueUpdateDraw(func() {
			renderToast()
		})
	}()
}

// dismissToast removes a toast by page name and updates tracking state.
// Must be called on the tview event loop (inside QueueUpdateDraw or a handler).
func (s *Shell) dismissToast(pageName string) {
	prev := s.app.GetFocus()
	s.pages.RemovePage(pageName)
	// Pages.RemovePage re-delegates focus to the last visible page (possibly
	// another toast or the suggestions overlay). Restore focus properly.
	s.restoreFocusAfterToast(prev)
	if s.activeToasts > 0 {
		s.activeToasts--
	}
	// Remove from tracked IDs
	for i, id := range s.activeToastIDs {
		if id == pageName {
			s.activeToastIDs = append(s.activeToastIDs[:i], s.activeToastIDs[i+1:]...)
			break
		}
	}
}

// DismissOldestToast removes the oldest visible toast (manual dismiss via 'd' key).
// Returns true if a toast was dismissed, false if no toasts are visible.
// Must be called on the tview event loop.
func (s *Shell) DismissOldestToast() bool {
	if len(s.activeToastIDs) == 0 {
		return false
	}
	oldest := s.activeToastIDs[0]
	s.dismissToast(oldest)
	return true
}

// restoreFocusAfterToast restores focus to the appropriate widget after a
// toast page is added or removed. tview.Pages.AddPage / RemovePage re-delegate
// focus to the topmost visible page (the toast Grid), stealing it from
// whichever widget the user was interacting with. This helper restores focus:
//   - to the widget that had it when a modal (form, sub-selector) is open,
//   - to the omnibar input when the omnibar is active,
//   - to the content area (active view) otherwise.
func (s *Shell) restoreFocusAfterToast(prev tview.Primitive) {
	if prev != nil && (s.pages.HasPage("inline-overlay") || s.pages.HasPage("sub-overlay")) {
		s.app.SetFocus(prev)
		return
	}
	if s.omnibar.IsActive() {
		s.omnibar.RestoreFocus()
		return
	}
	s.app.SetFocus(s.content)
}

func toastStyle(level ToastLevel) (string, tcell.Color) {
	switch level {
	case ToastSuccess:
		return theme.IconSuccess, theme.Success
	case ToastError:
		return theme.IconError, theme.Error
	case ToastWarning:
		return theme.IconWarning, theme.Warning
	case ToastInfo:
		return theme.IconArrow, theme.Accent
	default:
		return theme.IconDot, theme.Accent
	}
}
