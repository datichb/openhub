package limits

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Queued sessions (maximum of working sessions, memory cap): the session is
// created in the tool, and its first prompt waits in
// ~/.oh/sessions/<id>/queued.json until the daemon finds a free slot.

// QueueFileName is the per-session file of a queued first prompt.
const QueueFileName = "queued.json"

// Queue priorities (lower first; same priority: first queued first).
const (
	PriorityInteractive = 0
	PriorityHeadless    = 1
)

// Queued is a first prompt waiting for a slot.
type Queued struct {
	Prompt   string    `json:"prompt"`
	Priority int       `json:"priority"`
	QueuedAt time.Time `json:"queued_at"`
	// Scope is the scope of the maximum of working sessions (ActiveScope).
	Scope     string `json:"scope"`
	MaxActive int    `json:"max_active,omitempty"`
	MemoryMB  int    `json:"memory_mb,omitempty"`
}

// SaveQueued writes the queued prompt of a session (0600).
func SaveQueued(sessionsDir, sessionID string, q Queued) error {
	p := filepath.Join(sessionsDir, sessionID, QueueFileName)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(q)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// LoadQueued reads the queued prompt of a session (ok false when none).
func LoadQueued(sessionsDir, sessionID string) (Queued, bool, error) {
	var q Queued
	data, err := os.ReadFile(filepath.Join(sessionsDir, sessionID, QueueFileName))
	if errors.Is(err, os.ErrNotExist) {
		return q, false, nil
	}
	if err != nil {
		return q, false, err
	}
	if err := json.Unmarshal(data, &q); err != nil {
		return q, false, err
	}
	return q, true, nil
}

// RemoveQueued removes the queued prompt of a session.
func RemoveQueued(sessionsDir, sessionID string) error {
	err := os.Remove(filepath.Join(sessionsDir, sessionID, QueueFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Less orders two queued prompts.
func (q Queued) Less(o Queued) bool {
	if q.Priority != o.Priority {
		return q.Priority < o.Priority
	}
	return q.QueuedAt.Before(o.QueuedAt)
}
