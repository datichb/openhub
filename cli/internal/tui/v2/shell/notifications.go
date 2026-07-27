package shell

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ── In-memory store ───────────────────────────────────────────────────────────

// Notification represents a single toast event captured by the NotificationStore.
// The Message field always holds the full, untruncated text.
type Notification struct {
	Time    time.Time
	Level   ToastLevel
	Message string
}

// NotificationStore is a bounded, thread-safe ring buffer of Notifications.
// It holds the current session's toasts only. Cross-session history is persisted
// in the JSONL file and read lazily by the NotificationsView at Mount time.
type NotificationStore struct {
	mu    sync.Mutex
	items []Notification
	max   int
}

// NewNotificationStore creates a store that retains at most max entries.
func NewNotificationStore(max int) *NotificationStore {
	if max <= 0 {
		max = 50
	}
	return &NotificationStore{
		items: make([]Notification, 0, max),
		max:   max,
	}
}

// Add appends a notification. When capacity is reached the oldest entry is evicted.
func (ns *NotificationStore) Add(level ToastLevel, msg string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	n := Notification{Time: time.Now(), Level: level, Message: msg}
	if len(ns.items) >= ns.max {
		copy(ns.items, ns.items[1:])
		ns.items[len(ns.items)-1] = n
	} else {
		ns.items = append(ns.items, n)
	}
}

// All returns a snapshot of all stored notifications, most recent first.
func (ns *NotificationStore) All() []Notification {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	out := make([]Notification, len(ns.items))
	for i, n := range ns.items {
		out[len(ns.items)-1-i] = n
	}
	return out
}

// Len returns the current number of stored notifications.
func (ns *NotificationStore) Len() int {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	return len(ns.items)
}

// Entries implements views.NotificationsProvider.
// Returns the current session's notifications in most-recent-first order.
func (ns *NotificationStore) Entries() []views.NotificationEntry {
	all := ns.All()
	out := make([]views.NotificationEntry, len(all))
	for i, n := range all {
		out[i] = views.NotificationEntry{
			TimeLabel: n.Time.Format("15:04:05"),
			Level:     int(n.Level),
			Message:   n.Message,
		}
	}
	return out
}

// ── Persistence ───────────────────────────────────────────────────────────────

// persistedNotif is the on-disk JSON representation of a notification.
// Short keys minimise file size.
type persistedNotif struct {
	Time    time.Time `json:"t"`
	Level   int       `json:"l"`
	Message string    `json:"m"`
}

// NotificationsFilePath returns the default path for the persistent JSONL file.
// The file lives alongside hub.toml in ~/.oh/.
func NotificationsFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".oh", "notifications.jsonl")
}

// AppendToFile appends a single notification as a JSONL line to path.
// Creates the file and any missing parent directories if absent.
// Uses O_APPEND so concurrent calls do not corrupt each other.
func AppendToFile(path string, level ToastLevel, msg string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("notifications: mkdir %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("notifications: open %s: %w", path, err)
	}
	defer f.Close()

	entry := persistedNotif{Time: time.Now(), Level: int(level), Message: msg}
	if err := json.NewEncoder(f).Encode(entry); err != nil {
		return fmt.Errorf("notifications: encode: %w", err)
	}
	return nil
}

// ReadLastN reads the last n notifications from the JSONL file at path.
// Returns entries in most-recent-first order (most recent entry first).
// Returns an empty slice and nil error if the file does not exist.
func ReadLastN(path string, n int) ([]views.NotificationEntry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notifications: open %s: %w", path, err)
	}
	defer f.Close()

	// Read all lines into a buffer — max file size is ~500 * ~200B = ~100KB
	var lines []persistedNotif
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var pn persistedNotif
		if err := json.Unmarshal(line, &pn); err != nil {
			continue // skip malformed lines gracefully
		}
		lines = append(lines, pn)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("notifications: read %s: %w", path, err)
	}

	// Take the last n entries
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	// Return most-recent-first
	out := make([]views.NotificationEntry, len(lines))
	for i, pn := range lines {
		out[len(lines)-1-i] = views.NotificationEntry{
			TimeLabel: pn.Time.Local().Format("02/01 15:04:05"),
			Level:     pn.Level,
			Message:   pn.Message,
		}
	}
	return out, nil
}

// RotateFile truncates the JSONL file to keep at most maxLines entries and
// removes entries older than maxAge. The rewrite is atomic (tmp file + rename).
// No-op if the file does not exist.
func RotateFile(path string, maxLines int, maxAge time.Duration) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("notifications: rotate open %s: %w", path, err)
	}

	cutoff := time.Now().Add(-maxAge)
	var lines []string

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		var pn persistedNotif
		if err := json.Unmarshal([]byte(line), &pn); err != nil {
			continue // drop malformed lines
		}
		if pn.Time.After(cutoff) {
			lines = append(lines, line)
		}
	}
	f.Close()
	if err := sc.Err(); err != nil {
		return fmt.Errorf("notifications: rotate scan %s: %w", path, err)
	}

	// Apply maxLines cap — keep the last maxLines entries
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	// Atomic rewrite: write to a temp file then rename
	tmp := path + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("notifications: rotate create tmp: %w", err)
	}
	w := bufio.NewWriter(out)
	for _, line := range lines {
		_, _ = fmt.Fprintln(w, line)
	}
	if err := w.Flush(); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("notifications: rotate flush: %w", err)
	}
	out.Close()

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("notifications: rotate rename: %w", err)
	}
	return nil
}
