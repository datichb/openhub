package tracker

import (
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
	// AutoPlanHeld maps hubProjectID to the issues assigned to members that the
	// auto-plan left off the board at its last run because a member had
	// reached max_auto_plan_per_member. The next run offers them again; the
	// board reads the count to say so.
	AutoPlanHeld map[string]HeldSet `json:"autoplan_held,omitempty"`
	// AutoPlanScanned maps "trackerType/projectID" to the time of the first
	// complete scan of the assigned issues of the project. Before it, the
	// auto-plan does not filter on the update time: an issue assigned long ago
	// and not touched since was never seen by the runs that did.
	AutoPlanScanned map[string]time.Time `json:"autoplan_scanned,omitempty"`
}

// HeldSet is what the auto-plan held back for one hub project.
type HeldSet struct {
	// Limit is max_auto_plan_per_member of the run.
	Limit  int         `json:"limit"`
	Issues []HeldIssue `json:"issues"`
}

// HeldIssue is an assigned issue the auto-plan left off the board.
type HeldIssue struct {
	TicketID    string `json:"ticket_id"`
	ExternalIID int    `json:"external_iid"`
	MemberID    string `json:"member_id"`
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
	if s.AutoPlanHeld == nil {
		s.AutoPlanHeld = make(map[string]HeldSet)
	}
	if s.AutoPlanScanned == nil {
		s.AutoPlanScanned = make(map[string]time.Time)
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

// SetAutoPlanHeld records the issues the auto-plan left off the board for a
// hub project (none: the entry is removed).
func (s *SyncState) SetAutoPlanHeld(hubProjectID string, limit int, issues []HeldIssue) {
	if len(issues) == 0 {
		delete(s.AutoPlanHeld, hubProjectID)
		return
	}
	if s.AutoPlanHeld == nil {
		s.AutoPlanHeld = make(map[string]HeldSet)
	}
	s.AutoPlanHeld[hubProjectID] = HeldSet{Limit: limit, Issues: issues}
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

// AutoPlanHeldFor sums what the auto-plan held back for the given hub
// projects, with the largest limit among them (0 when nothing is held). The
// board reads it from the sync state of the machine.
func AutoPlanHeldFor(stateDir string, hubProjectIDs []string) (held, limit int) {
	state, err := LoadSyncState(stateDir)
	if err != nil {
		return 0, 0
	}
	for _, id := range hubProjectIDs {
		set := state.AutoPlanHeld[id]
		held += len(set.Issues)
		limit = max(limit, set.Limit)
	}
	return held, limit
}
