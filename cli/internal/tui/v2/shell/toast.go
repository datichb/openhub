package shell

import (
	"fmt"
	"log"
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

// toastMaxLen returns the maximum display length for a toast message based on
// its severity level. Errors and warnings get more characters so diagnostics
// are legible without opening the Notifications view.
func toastMaxLen(level ToastLevel) int {
	switch level {
	case ToastError, ToastWarning:
		return 80
	default:
		return 50
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

	// Log errors to stderr so they survive TUI sessions and can be redirected.
	if level == ToastError {
		log.Printf("[ERROR] %s", msg)
	}

	// Truncate for display only — errors/warnings get more room.
	// NOTE: this modifies the local variable msg but does NOT affect the
	// already-persisted full message above.
	maxLen := toastMaxLen(level)
	if len(msg) > maxLen {
		msg = msg[:maxLen-3] + "..."
	}

	toast := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf(" %s %s ", icon, msg))
	toast.SetBackgroundColor(theme.BgElement)
	toast.SetBorder(true)
	toast.SetBorderColor(borderColor)

	toastWidth := len(msg) + 8
	if toastWidth < 20 {
		toastWidth = 20
	}
	if toastWidth > 90 {
		toastWidth = 90
	}
	toastHeight := 3

	// Position at top-right using a Grid
	grid := tview.NewGrid().
		SetColumns(0, toastWidth, 2).
		SetRows(1, toastHeight, 0)
	grid.AddItem(toast, 1, 1, 1, 1, 0, 0, false)

	pageName := fmt.Sprintf("toast-%d", time.Now().UnixNano())
	s.pages.AddPage(pageName, grid, true, true)

	// Auto-dismiss after duration
	time.AfterFunc(duration, func() {
		s.app.QueueUpdateDraw(func() {
			s.pages.RemovePage(pageName)
		})
	})
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
