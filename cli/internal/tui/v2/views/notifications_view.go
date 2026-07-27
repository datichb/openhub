package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// NotificationEntry is a display-agnostic representation of a past notification.
// It decouples the view from the shell package (avoiding an import cycle).
type NotificationEntry struct {
	// TimeLabel is a pre-formatted time string (e.g. "24/07 14:32:05").
	TimeLabel string
	// Level is the severity: 0=Success, 1=Error, 2=Warning, 3=Info
	// (mirrors shell.ToastLevel ordering).
	Level int
	// Message is the full, untruncated notification text.
	Message string
}

// NotificationsProvider is the minimal interface for in-memory entries.
type NotificationsProvider interface {
	Entries() []NotificationEntry
}

// NotificationsViewConfig holds the configuration for the NotificationsView.
// Separating these from the constructor keeps the views package free of imports
// from the shell package (which imports views — cycle avoidance).
type NotificationsViewConfig struct {
	// FilePath is the path to the persistent JSONL notifications file.
	// When empty, the view only shows the current session's notifications.
	FilePath string
	// ReadLastN reads the last n entries from the persistent file.
	// Signature matches shell.ReadLastN so it can be passed directly.
	ReadLastN func(path string, n int) ([]NotificationEntry, error)
}

// NotificationsView displays the full notification history.
// On every Mount it asynchronously loads the last 50 entries from the
// persistent JSONL file (cross-session history) and displays them.
type NotificationsView struct {
	cfg NotificationsViewConfig
	tv  *tview.TextView
	app *tview.Application
}

var _ View = (*NotificationsView)(nil)

// NewNotificationsView creates the notifications view.
// cfg.ReadLastN may be nil if only in-memory display is desired.
func NewNotificationsView(cfg NotificationsViewConfig) *NotificationsView {
	return &NotificationsView{cfg: cfg}
}

// ID returns the view identifier.
func (v *NotificationsView) ID() string { return "notifications" }

// Title returns the display title.
func (v *NotificationsView) Title() string { return "Notifications" }

// StatusHints returns keybinding hints.
func (v *NotificationsView) StatusHints() string {
	return "j/k scroll · Ctrl+P commandes"
}

// Mount builds and populates the notifications display.
// Content is loaded asynchronously from the persistent file to avoid blocking
// the TUI event loop.
func (v *NotificationsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app

	v.tv = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true).
		SetWordWrap(true).
		SetTextAlign(tview.AlignLeft)
	v.tv.SetBackgroundColor(theme.BgPanel)
	v.tv.SetBorderPadding(1, 0, 2, 2)

	// j/k navigation
	v.tv.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, col := v.tv.GetScrollOffset()
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				v.tv.ScrollTo(row+1, col)
				return nil
			case 'k':
				if row > 0 {
					v.tv.ScrollTo(row-1, col)
				}
				return nil
			}
		}
		return event
	})

	// Show loading placeholder immediately
	muted := theme.ColorTag(theme.TextMutedHex)
	v.tv.SetText(fmt.Sprintf("\n  %sChargement des notifications...%s", muted, theme.TagColor))
	content.AddItem(v.tv, 0, 1, true)

	// Load entries from the persistent file asynchronously.
	// QueueUpdateDraw is called from a goroutine (not from a handler), so no deadlock.
	go func() {
		entries := v.loadEntries()
		app.QueueUpdateDraw(func() {
			if v.tv == nil {
				return // view was unmounted before the goroutine finished
			}
			v.tv.SetText(renderNotifications(entries))
		})
	}()
}

// loadEntries fetches entries from the persistent file (most recent 50).
// Falls back to an empty slice on any error.
func (v *NotificationsView) loadEntries() []NotificationEntry {
	if v.cfg.ReadLastN == nil || v.cfg.FilePath == "" {
		return nil
	}
	entries, _ := v.cfg.ReadLastN(v.cfg.FilePath, 50)
	return entries
}

// Unmount cleans up resources.
func (v *NotificationsView) Unmount() {
	v.tv = nil
	v.app = nil
}

// HandleKey processes view-specific key events.
func (v *NotificationsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	return event
}

// ── Rendering ─────────────────────────────────────────────────────────────────

// renderNotifications formats notification entries for tview dynamic colors.
// No manual word-wrapping — the TextView has SetWrap(true)/SetWordWrap(true).
// Entries are already in most-recent-first order.
func renderNotifications(entries []NotificationEntry) string {
	if len(entries) == 0 {
		muted := theme.ColorTag(theme.TextMutedHex)
		return fmt.Sprintf("\n  %sAucune notification.%s", muted, theme.TagColor)
	}

	var sb strings.Builder
	sb.WriteString("\n")
	for _, e := range entries {
		icon, colorTag := notificationStyle(e.Level)
		timeStr := fmt.Sprintf("%s%s%s", theme.ColorTag(theme.TextMutedHex), e.TimeLabel, theme.TagColor)
		fmt.Fprintf(&sb, "  %s  %s%s%s  %s\n",
			timeStr, colorTag, icon, theme.TagColor, e.Message)
	}
	return sb.String()
}

// notificationStyle returns the icon and tview color tag for a level integer.
// Level mirrors shell.ToastLevel: 0=Success, 1=Error, 2=Warning, 3=Info.
func notificationStyle(level int) (icon, colorTag string) {
	switch level {
	case 0:
		return theme.IconSuccess, theme.ColorTag(theme.SuccessHex)
	case 1:
		return theme.IconError, theme.ColorTag(theme.ErrorHex)
	case 2:
		return theme.IconWarning, theme.ColorTag(theme.WarningHex)
	default:
		return theme.IconArrow, theme.ColorTag(theme.AccentHex)
	}
}
