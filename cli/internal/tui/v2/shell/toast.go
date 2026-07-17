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

func (s *Shell) showToast(msg string, level ToastLevel, duration time.Duration) {
	icon, borderColor := toastStyle(level)

	// Truncate long messages
	if len(msg) > 50 {
		msg = msg[:47] + "..."
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
	if toastWidth > 60 {
		toastWidth = 60
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
