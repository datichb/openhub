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

	pageName := fmt.Sprintf("toast-%d", time.Now().UnixNano())
	s.activeToasts++
	s.activeToastIDs = append(s.activeToastIDs, pageName)
	s.pages.AddPage(pageName, grid, true, true)

	// Auto-dismiss after duration
	time.AfterFunc(duration, func() {
		s.app.QueueUpdateDraw(func() {
			s.dismissToast(pageName)
		})
	})
}

// dismissToast removes a toast by page name and updates tracking state.
// Must be called on the tview event loop (inside QueueUpdateDraw or a handler).
func (s *Shell) dismissToast(pageName string) {
	s.pages.RemovePage(pageName)
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
