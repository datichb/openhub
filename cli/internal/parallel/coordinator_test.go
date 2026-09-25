package parallel

import (
	"context"
	"testing"

	"github.com/datichb/openhub/cli/internal/platform"
)

// testPlatform is a minimal mock that satisfies platform.SessionPlatform for
// coordinator unit tests. Most methods are unreachable in these tests since
// they only validate admission (ticket count, budget, etc.).
type testPlatform struct{}

func (testPlatform) Name() platform.Name                                                      { return platform.OpenCode }
func (testPlatform) Available() bool                                                          { return true }
func (testPlatform) Version() (string, error)                                                 { return "test", nil }
func (testPlatform) RunInteractive(_ context.Context, _ platform.RunOpts) (*platform.RunResult, error) { return nil, nil }
func (testPlatform) ExecReplace(_ platform.RunOpts) error                                     { return nil }
func (testPlatform) RunHeadless(_ context.Context, _ platform.HeadlessOpts) (*platform.HeadlessResult, error) { return nil, nil }
func (testPlatform) FindActiveSessions(_ context.Context, _ string) ([]platform.ActiveSession, error) { return nil, nil }
func (testPlatform) IsGhostSession(_ platform.ActiveSession) bool                             { return false }
func (testPlatform) Capabilities() platform.Capabilities                                          { return platform.Capabilities{Parallel: true} }
func (testPlatform) NewParallelRunner(_ platform.ParallelRunnerOpts) (platform.ParallelRunner, error) { return nil, nil }
func (testPlatform) RequiresDeploy() bool                                                     { return false }

func TestNewCoordinator_NoTickets(t *testing.T) {
	_, err := NewCoordinator(CoordinatorOpts{
		ProjectPath: "/tmp/project",
		ProjectID:   "test",
		Tickets:     nil,
		Config:      DefaultConfig(),
		PromptFunc:  func(string) string { return "" },
	}, testPlatform{})
	if err == nil {
		t.Error("expected error for empty tickets")
	}
}

func TestNewCoordinator_TooManyTickets(t *testing.T) {
	cfg := Config{MaxSessions: 3, PortRangeStart: 4100, DefaultTicketWeightMin: 60}
	_, err := NewCoordinator(CoordinatorOpts{
		ProjectPath: "/tmp/project",
		ProjectID:   "test",
		Tickets:     []string{"a", "b", "c", "d"},
		Config:      cfg,
		PromptFunc:  func(string) string { return "" },
	}, testPlatform{})
	if err == nil {
		t.Error("expected error for too many tickets")
	}
}

func TestNewCoordinator_BudgetExceeded(t *testing.T) {
	cfg := Config{
		MaxSessions:            10,
		MaxBudgetMinutes:       180,
		DefaultTicketWeightMin: 60,
		PortRangeStart:         4100,
	}
	_, err := NewCoordinator(CoordinatorOpts{
		ProjectPath: "/tmp/project",
		ProjectID:   "test",
		Tickets:     []string{"a", "b", "c", "d"},
		TicketEstimates: map[string]int{
			"a": 120,
			"b": 120,
			"c": 120,
			"d": 120,
		},
		Config:     cfg,
		PromptFunc: func(string) string { return "" },
	}, testPlatform{})
	if err == nil {
		t.Error("expected error for budget exceeded (480 > 180)")
	}
}

func TestNewCoordinator_BudgetDisabled(t *testing.T) {
	cfg := Config{
		MaxSessions:            10,
		MaxBudgetMinutes:       0, // disabled
		DefaultTicketWeightMin: 60,
		PortRangeStart:         4100,
	}
	// This should NOT fail even though total weight is huge -- budget is disabled
	coord, err := NewCoordinator(CoordinatorOpts{
		ProjectPath: "/tmp/project",
		ProjectID:   "test",
		Tickets:     []string{"a", "b", "c"},
		TicketEstimates: map[string]int{
			"a": 480,
			"b": 480,
			"c": 480,
		},
		Config:     cfg,
		PromptFunc: func(string) string { return "" },
	}, testPlatform{})
	if err != nil {
		if contains(err.Error(), "budget") {
			t.Errorf("should not fail on budget when disabled, got: %v", err)
		}
	}
	_ = coord
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsImpl(s, sub))
}

func containsImpl(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestNewCoordinator_Valid(t *testing.T) {
	coord, err := NewCoordinator(CoordinatorOpts{
		ProjectPath: "/tmp/project",
		ProjectID:   "test",
		Tickets:     []string{"bd-42", "bd-43"},
		Priority:    "bd-42",
		Config:      DefaultConfig(),
		PromptFunc:  func(id string) string { return "work on " + id },
	}, testPlatform{})
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
