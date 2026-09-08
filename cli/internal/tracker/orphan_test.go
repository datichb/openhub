package tracker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSyncState_KnownTickets_Empty(t *testing.T) {
	s := &SyncState{
		LastSyncAt:   make(map[string]time.Time),
		KnownTickets: make(map[string][]KnownTicket),
	}
	got := s.GetKnownTickets("project-1")
	assert.Nil(t, got)
}

func TestSyncState_KnownTickets_SetAndGet(t *testing.T) {
	s := &SyncState{
		LastSyncAt:   make(map[string]time.Time),
		KnownTickets: make(map[string][]KnownTicket),
	}
	tickets := []KnownTicket{
		{TicketID: "42", ExternalIID: 42, ClaimedBy: "alice"},
		{TicketID: "43", ExternalIID: 43, ClaimedBy: "bob"},
	}
	s.SetKnownTickets("project-1", tickets)

	got := s.GetKnownTickets("project-1")
	assert.Equal(t, 2, len(got))
	assert.Equal(t, "42", got[0].TicketID)
	assert.Equal(t, 43, got[1].ExternalIID)

	// Different project returns nil
	assert.Nil(t, s.GetKnownTickets("project-2"))
}

func TestSyncState_KnownTickets_OverwriteOnSet(t *testing.T) {
	s := &SyncState{
		LastSyncAt:   make(map[string]time.Time),
		KnownTickets: make(map[string][]KnownTicket),
	}
	s.SetKnownTickets("p1", []KnownTicket{{TicketID: "1"}, {TicketID: "2"}})
	assert.Equal(t, 2, len(s.GetKnownTickets("p1")))

	// Overwrite with fewer tickets (e.g., one was released)
	s.SetKnownTickets("p1", []KnownTicket{{TicketID: "2"}})
	assert.Equal(t, 1, len(s.GetKnownTickets("p1")))
}

func TestSyncState_KnownTickets_NilMapInit(t *testing.T) {
	// Simulates loading an old sync-state.json without the known_tickets field
	s := &SyncState{
		LastSyncAt: make(map[string]time.Time),
		// KnownTickets is nil
	}
	// SetKnownTickets should initialize the map
	s.SetKnownTickets("p1", []KnownTicket{{TicketID: "1"}})
	assert.Equal(t, 1, len(s.GetKnownTickets("p1")))
}

func TestOrphanDetection_Logic(t *testing.T) {
	// Simulate the orphan detection logic from recoverOrphans
	known := []KnownTicket{
		{TicketID: "42", ExternalIID: 42, ClaimedBy: "alice"},
		{TicketID: "43", ExternalIID: 43, ClaimedBy: "bob"},
		{TicketID: "44", ExternalIID: 44, ClaimedBy: "alice"},
	}
	claimedTickets := map[string]bool{
		"43": true,
		"44": true,
		// "42" is missing — orphan!
	}

	var orphans []KnownTicket
	for _, kt := range known {
		if !claimedTickets[kt.TicketID] {
			orphans = append(orphans, kt)
		}
	}

	assert.Equal(t, 1, len(orphans))
	assert.Equal(t, "42", orphans[0].TicketID)
	assert.Equal(t, 42, orphans[0].ExternalIID)
	assert.Equal(t, "alice", orphans[0].ClaimedBy)
}
