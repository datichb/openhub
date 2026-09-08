// Package shell — TUI-aware slog handler that redirects logs to the toast/notification
// system instead of writing to stderr (which corrupts the tview terminal).
package shell

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// TUILogHandler implements slog.Handler by routing log records to the TUI
// notification system. Warn and Error produce visual toasts; Info and Debug
// are persisted silently to the NotificationStore for later review in the
// Notifications view.
type TUILogHandler struct {
	shell      *Shell
	notifStore *NotificationStore
	level      slog.Level
	groups     []string    // accumulated slog group prefixes
	attrs      []slog.Attr // pre-attached attributes
}

// NewTUILogHandler creates a handler that routes slog records to the TUI.
// The shell reference is used for visual toasts (QueueUpdateDraw-safe).
// The notifStore receives all records for persistence.
func NewTUILogHandler(shell *Shell, notifStore *NotificationStore, level slog.Level) *TUILogHandler {
	return &TUILogHandler{
		shell:      shell,
		notifStore: notifStore,
		level:      level,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *TUILogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle processes a log record: persists to NotificationStore and optionally
// shows a visual toast for Warn/Error levels.
func (h *TUILogHandler) Handle(_ context.Context, r slog.Record) error {
	msg := h.formatMessage(r)

	level := slogToToastLevel(r.Level)

	// Always persist to notification store (thread-safe via mutex)
	if h.notifStore != nil {
		h.notifStore.Add(level, msg)
	}

	// Visual toast only for Warn and above — must go through QueueUpdateDraw
	// because Handle() can be called from any goroutine.
	// Guard: only enqueue when the tview event loop is running.
	// CRITICAL: use a goroutine to avoid deadlock when Handle() is called
	// from within a tview event handler (SetSelectedFunc, InputCapture, Mount).
	// QueueUpdateDraw is synchronous — it blocks until the event loop drains
	// the callback. If we're already ON the event loop, that's a deadlock.
	// The goroutine blocks independently; the event loop processes it on the next cycle.
	if r.Level >= slog.LevelWarn && h.shell != nil && h.shell.IsRunning() {
		toastMsg := msg
		go h.shell.App().QueueUpdateDraw(func() {
			h.shell.ShowToast(toastMsg, level)
		})
	}

	return nil
}

// WithAttrs returns a new handler with the given attributes pre-attached.
func (h *TUILogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TUILogHandler{
		shell:      h.shell,
		notifStore: h.notifStore,
		level:      h.level,
		groups:     h.groups,
		attrs:      append(sliceClone(h.attrs), attrs...),
	}
}

// WithGroup returns a new handler with the given group name appended.
func (h *TUILogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &TUILogHandler{
		shell:      h.shell,
		notifStore: h.notifStore,
		level:      h.level,
		groups:     append(sliceClone(h.groups), name),
		attrs:      h.attrs,
	}
}

// formatMessage builds a human-readable message from the slog record,
// including group prefix and key attributes for context.
func (h *TUILogHandler) formatMessage(r slog.Record) string {
	var b strings.Builder

	// Group prefix: "tracker.sync: " or "teamstate: "
	if len(h.groups) > 0 {
		b.WriteString(strings.Join(h.groups, "."))
		b.WriteString(": ")
	}

	b.WriteString(r.Message)

	// Append pre-attached attrs
	for _, a := range h.attrs {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
	}

	// Append record-level attrs (limited to first 3 for readability)
	count := 0
	r.Attrs(func(a slog.Attr) bool {
		if count >= 3 {
			return false
		}
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
		count++
		return true
	})

	return b.String()
}

// slogToToastLevel maps slog levels to toast levels.
func slogToToastLevel(level slog.Level) ToastLevel {
	switch {
	case level >= slog.LevelError:
		return ToastError
	case level >= slog.LevelWarn:
		return ToastWarning
	case level >= slog.LevelInfo:
		return ToastInfo
	default:
		return ToastInfo
	}
}

// sliceClone returns a shallow copy of a slice.
func sliceClone[T any](s []T) []T {
	if s == nil {
		return nil
	}
	c := make([]T, len(s))
	copy(c, s)
	return c
}
