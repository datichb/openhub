package parallel

import (
	"testing"
)

func TestNewCoordinator_NoTickets(t *testing.T) {
	_, err := NewCoordinator(CoordinatorOpts{
		ProjectPath: "/tmp/project",
		ProjectID:   "test",
		Tickets:     nil,
		Config:      DefaultConfig(),
		PromptFunc:  func(string) string { return "" },
	})
	if err == nil {
		t.Error("expected error for empty tickets")
	}
}

func TestNewCoordinator_TooManyTickets(t *testing.T) {
	_, err := NewCoordinator(CoordinatorOpts{
		ProjectPath: "/tmp/project",
		ProjectID:   "test",
		Tickets:     []string{"a", "b", "c", "d"},
		Config:      Config{MaxSessions: 3, PortRangeStart: 4100},
		PromptFunc:  func(string) string { return "" },
	})
	if err == nil {
		t.Error("expected error for too many tickets")
	}
}

func TestNewCoordinator_Valid(t *testing.T) {
	coord, err := NewCoordinator(CoordinatorOpts{
		ProjectPath: "/tmp/project",
		ProjectID:   "test",
		Tickets:     []string{"bd-42", "bd-43"},
		Priority:    "bd-42",
		Config:      DefaultConfig(),
		PromptFunc:  func(id string) string { return "work on " + id },
	})
	if err != nil {
		t.Fatalf("NewCoordinator failed: %v", err)
	}
	if coord == nil {
		t.Fatal("coordinator should not be nil")
	}
	if coord.State() == nil {
		t.Error("state should not be nil")
	}
}

func TestSortByPriority(t *testing.T) {
	sessions := []SessionInfo{
		{TicketID: "bd-42", Priority: false},
		{TicketID: "bd-43", Priority: true},
		{TicketID: "bd-44", Priority: false},
	}

	sortByPriority(sessions)

	if sessions[0].TicketID != "bd-43" {
		t.Errorf("expected bd-43 first (priority), got %s", sessions[0].TicketID)
	}
}

func TestSortByPriority_NoPriority(t *testing.T) {
	sessions := []SessionInfo{
		{TicketID: "bd-42", Priority: false},
		{TicketID: "bd-43", Priority: false},
	}

	sortByPriority(sessions)

	// Order unchanged
	if sessions[0].TicketID != "bd-42" {
		t.Errorf("expected bd-42 first (no priority = unchanged order), got %s", sessions[0].TicketID)
	}
}
