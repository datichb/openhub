package tracker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/datichb/openhub/cli/internal/fsutil"
)

// SyncState persists the last-sync timestamp and known ticket IDs locally so that
// the next sync can pass updated_after to the tracker API and detect orphaned claims.
//
// Stored in ~/.oh/sync-state.json (never committed to the team-state git repo —
// each member has their own sync cadence).
type SyncState struct {
	// LastSyncAt maps "trackerType/projectID" to the timestamp of the last
	// successful sync for that project.
	LastSyncAt map[string]time.Time `json:"last_sync_at"`
	// KnownTickets maps hubProjectID to the list of tickets that had a local claim
	// after the last successful sync. Used for orphan detection (ADR-032):
	// if a ticket is in KnownTickets but absent from ListClaims, it was
	// accidentally deleted or corrupted and needs recovery.
	KnownTickets map[string][]KnownTicket `json:"known_tickets,omitempty"`
	// AutoPlanScanned maps "trackerType/projectID" to the time of the first
	// complete scan of the assigned issues of the project. Before it, the
	// auto-plan does not filter on the update time: an issue assigned long ago
	// and not touched since was never seen by the runs that did.
	AutoPlanScanned map[string]time.Time `json:"autoplan_scanned,omitempty"`
	// LegacyAutoPlanHeld is what oh 5.1.0 wrote under "autoplan_held": the
	// issues its per-member plan cap left off the board. They are not kept any
	// more; reading the field only tells LoadSyncState that such issues exist,
	// so that the next run scans the assigned issues in full again and puts
	// them on the board (an incremental fetch would not see them, untouched
	// since).
	LegacyAutoPlanHeld json.RawMessage `json:"autoplan_held,omitempty"`
}

// KnownTicket records the essential identifiers of a claimed ticket for orphan detection.
type KnownTicket struct {
	TicketID    string `json:"ticket_id"`
	ExternalIID int    `json:"external_iid"`
	ClaimedBy   string `json:"claimed_by"`
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
		return &SyncState{LastSyncAt: make(map[string]time.Time)}, nil //nolint:nilerr // corrupt state file, start fresh
	}
	if s.LastSyncAt == nil {
		s.LastSyncAt = make(map[string]time.Time)
	}
	if s.KnownTickets == nil {
		s.KnownTickets = make(map[string][]KnownTicket)
	}
	if s.AutoPlanScanned == nil || legacyHeldPresent(s.LegacyAutoPlanHeld) {
		s.AutoPlanScanned = make(map[string]time.Time)
	}
	s.LegacyAutoPlanHeld = nil
	return &s, nil
}

// legacyHeldPresent reports whether the "autoplan_held" of an oh 5.1.0 state
// holds issues.
func legacyHeldPresent(raw json.RawMessage) bool {
	switch string(bytes.TrimSpace(raw)) {
	case "", "null", "{}":
		return false
	}
	return true
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
	if err := fsutil.WriteFileAtomic(path, data, 0o600); err != nil {
		return fmt.Errorf("writing sync state: %w", err)
	}
	return nil
}

// LastSync returns the last sync time for (type, projectID), or zero time if unknown.
func (s *SyncState) LastSync(t Type, projectID string) time.Time {
	return s.LastSyncAt[syncStateKey(t, projectID)]
}

// SetLastSync records the sync time for (type, projectID).
func (s *SyncState) SetLastSync(t Type, projectID string, at time.Time) {
	s.LastSyncAt[syncStateKey(t, projectID)] = at
}

// GetKnownTickets returns the list of known tickets for a hub project.
// Returns nil if no tickets are known (first sync or old state file).
func (s *SyncState) GetKnownTickets(hubProjectID string) []KnownTicket {
	return s.KnownTickets[hubProjectID]
}

// SetKnownTickets saves the current list of claimed tickets for a hub project.
// Called at the end of each reconcileProject to snapshot the post-sync state.
func (s *SyncState) SetKnownTickets(hubProjectID string, tickets []KnownTicket) {
	if s.KnownTickets == nil {
		s.KnownTickets = make(map[string][]KnownTicket)
	}
	s.KnownTickets[hubProjectID] = tickets
}

// ForgetKnownTicket takes a ticket out of the known tickets of a hub project:
// its claim was released on purpose, so its absence is not a loss.
func (s *SyncState) ForgetKnownTicket(hubProjectID, ticketID string) {
	known := s.KnownTickets[hubProjectID]
	kept := known[:0:0]
	for _, kt := range known {
		if kt.TicketID != ticketID {
			kept = append(kept, kt)
		}
	}
	if len(kept) != len(known) {
		s.KnownTickets[hubProjectID] = kept
	}
}

// AutoPlanScan reports whether the assigned issues of (type, projectID) were
// scanned in full; MarkAutoPlanScanned records it.
func (s *SyncState) AutoPlanScan(t Type, projectID string) bool {
	return !s.AutoPlanScanned[syncStateKey(t, projectID)].IsZero()
}

// MarkAutoPlanScanned records the first complete scan of (type, projectID).
func (s *SyncState) MarkAutoPlanScanned(t Type, projectID string, at time.Time) {
	if s.AutoPlanScanned == nil {
		s.AutoPlanScanned = make(map[string]time.Time)
	}
	if k := syncStateKey(t, projectID); s.AutoPlanScanned[k].IsZero() {
		s.AutoPlanScanned[k] = at
	}
}
