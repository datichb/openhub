package tracker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SyncState persists the last-sync timestamp locally so that the next sync
// can pass updated_after to the tracker API and fetch only changed issues.
//
// Stored in ~/.oh/sync-state.json (never committed to the team-state git repo —
// each member has their own sync cadence).
type SyncState struct {
	// LastSyncAt maps "trackerType/projectID" to the timestamp of the last
	// successful sync for that project.
	LastSyncAt map[string]time.Time `json:"last_sync_at"`
}

// syncStateKey returns the map key for a (type, projectID) pair.
func syncStateKey(t Type, projectID string) string {
	return fmt.Sprintf("%s/%s", t, projectID)
}

// LoadSyncState reads the sync state from disk.
// Returns an empty state (not an error) if the file does not exist yet.
func LoadSyncState(stateDir string) (*SyncState, error) {
	path := filepath.Join(stateDir, "sync-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SyncState{LastSyncAt: make(map[string]time.Time)}, nil
		}
		return nil, fmt.Errorf("reading sync state: %w", err)
	}
	var s SyncState
	if err := json.Unmarshal(data, &s); err != nil {
		// Corrupt file — start fresh (don't block syncs).
		return &SyncState{LastSyncAt: make(map[string]time.Time)}, nil
	}
	if s.LastSyncAt == nil {
		s.LastSyncAt = make(map[string]time.Time)
	}
	return &s, nil
}

// Save writes the sync state to disk atomically.
func (s *SyncState) Save(stateDir string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling sync state: %w", err)
	}
	path := filepath.Join(stateDir, "sync-state.json")
	// Write to temp then rename for atomicity.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("writing sync state: %w", err)
	}
	return os.Rename(tmp, path)
}

// LastSync returns the last sync time for (type, projectID), or zero time if unknown.
func (s *SyncState) LastSync(t Type, projectID string) time.Time {
	return s.LastSyncAt[syncStateKey(t, projectID)]
}

// SetLastSync records the sync time for (type, projectID).
func (s *SyncState) SetLastSync(t Type, projectID string, at time.Time) {
	s.LastSyncAt[syncStateKey(t, projectID)] = at
}
