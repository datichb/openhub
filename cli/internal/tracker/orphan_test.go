package tracker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSyncState_KnownTickets(t *testing.T) {
	s := &SyncState{
		LastSyncAt:   make(map[string]time.Time),
		KnownTickets: make(map[string][]KnownTicket),
	}
	assert.Nil(t, s.GetKnownTickets("project-1"), "nothing known yet")

	s.SetKnownTickets("project-1", []KnownTicket{
		{TicketID: "42", ExternalIID: 42, ClaimedBy: "alice"},
		{TicketID: "43", ExternalIID: 43, ClaimedBy: "bob"},
	})
	got := s.GetKnownTickets("project-1")
	if assert.Len(t, got, 2, "the tickets are kept in order") {
		assert.Equal(t, "42", got[0].TicketID)
		assert.Equal(t, 43, got[1].ExternalIID)
	}
	assert.Nil(t, s.GetKnownTickets("project-2"), "another project knows nothing")

	s.SetKnownTickets("project-1", []KnownTicket{{TicketID: "43"}})
	assert.Len(t, s.GetKnownTickets("project-1"), 1, "a set replaces the list (a ticket was released)")

	old := &SyncState{LastSyncAt: make(map[string]time.Time)} // a sync-state.json written before known_tickets
	old.SetKnownTickets("p1", []KnownTicket{{TicketID: "1"}})
	assert.Len(t, old.GetKnownTickets("p1"), 1, "the missing map is created")
}
