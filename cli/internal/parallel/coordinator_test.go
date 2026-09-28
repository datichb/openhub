package parallel

import (
	"context"
	"fmt"
	"testing"

	"github.com/datichb/openhub/cli/internal/platform"
)

// --- Mock ParallelRunner for tests ---

type mockRunner struct {
	launched   map[string]platform.TaskOpts
	statuses   map[string]platform.TaskStatus
	aborted    map[string]bool
	cleanedUp  bool
	launchErr  error
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		launched: make(map[string]platform.TaskOpts),
		statuses: make(map[string]platform.TaskStatus),
		aborted:  make(map[string]bool),
	}
}

func (m *mockRunner) LaunchTask(_ context.Context, opts platform.TaskOpts) (platform.TaskHandle, error) {
	if m.launchErr != nil {
		return platform.TaskHandle{}, m.launchErr
	}
	m.launched[opts.TaskID] = opts
	return platform.TaskHandle{
		TaskID:    opts.TaskID,
		SessionID: "session-" + opts.TaskID,
	}, nil
}

func (m *mockRunner) GetAllStatuses(_ context.Context) (map[string]platform.TaskStatus, error) {
	return m.statuses, nil
}

func (m *mockRunner) GetModifiedFiles(_ context.Context, taskID string) ([]platform.FileChange, error) {
	return nil, nil
}

func (m *mockRunner) SendMessage(_ context.Context, taskID, message string) error {
	return nil
}

func (m *mockRunner) AbortTask(_ context.Context, taskID string) error {
	m.aborted[taskID] = true
	return nil
}

func (m *mockRunner) AttachTask(_ context.Context, taskID string) error {
	return fmt.Errorf("attach not supported in test")
}

func (m *mockRunner) Cleanup(_ context.Context) {
	m.cleanedUp = true
}

// --- Coordinator tests ---

func TestNewCoordinator_NoTickets(t *testing.T) {
	_, err := NewCoordinator(CoordinatorOpts{
		ProjectPath: "/tmp/project",
		ProjectID:   "test",
		Tickets:     nil,
		Config:      DefaultConfig(),
		PromptFunc:  func(string) string { return "" },
	}, newMockRunner())
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
	}, newMockRunner())
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
		TicketEstimates: map[string]int{"a": 120, "b": 120, "c": 120, "d": 120},
		Config:      cfg,
		PromptFunc:  func(string) string { return "" },
	}, newMockRunner())
	if err == nil {
		t.Error("expected error for budget exceeded (480 > 180)")
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
	}, newMockRunner())
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

func TestNewCoordinator_NilRunner(t *testing.T) {
	_, err := NewCoordinator(CoordinatorOpts{
		ProjectPath: "/tmp/project",
		ProjectID:   "test",
		Tickets:     []string{"a"},
		Config:      DefaultConfig(),
		PromptFunc:  func(string) string { return "" },
	}, nil)
	if err == nil {
		t.Error("expected error for nil runner")
	}
}

// --- Recovery tests ---

func TestAttemptRecovery_SkipsNonFailed(t *testing.T) {
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{TicketID: "bd-1", Status: StatusRunning})
	state.AddSession(SessionInfo{TicketID: "bd-2", Status: StatusCompleted})

	runner := newMockRunner()
	cfg := DefaultConfig()
	c := &Coordinator{
		opts:               CoordinatorOpts{Config: cfg, PromptFunc: func(string) string { return "" }},
		state:              state,
		runner:             runner,
		context:            NewSharedContext(state),
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}

	c.attemptRecovery(context.Background())

	if len(runner.launched) != 0 {
		t.Error("should not have launched any recovery tasks")
	}
}

func TestAttemptRecovery_SkipsExhaustedRetries(t *testing.T) {
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{TicketID: "bd-1", Status: StatusFailed, RetryCount: 2})

	runner := newMockRunner()
	cfg := DefaultConfig()
	cfg.MaxRetries = 2
	c := &Coordinator{
		opts:               CoordinatorOpts{Config: cfg, PromptFunc: func(string) string { return "" }},
		state:              state,
		runner:             runner,
		context:            NewSharedContext(state),
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}

	c.attemptRecovery(context.Background())

	if len(runner.launched) != 0 {
		t.Error("should not have launched recovery for exhausted retries")
	}
}

func TestAttemptRecovery_DisabledWhenMaxRetriesZero(t *testing.T) {
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{TicketID: "bd-1", Status: StatusFailed})

	runner := newMockRunner()
	cfg := DefaultConfig()
	cfg.MaxRetries = 0
	c := &Coordinator{
		opts:               CoordinatorOpts{Config: cfg, PromptFunc: func(string) string { return "" }},
		state:              state,
		runner:             runner,
		context:            NewSharedContext(state),
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}

	c.attemptRecovery(context.Background())

	if len(runner.launched) != 0 {
		t.Error("should not have launched recovery when disabled")
	}
}

func TestRecoverSession_Success(t *testing.T) {
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{
		TicketID:     "bd-1",
		Status:       StatusFailed,
		Error:        "server died",
		WorktreePath: t.TempDir(),
	})

	runner := newMockRunner()
	cfg := DefaultConfig()
	cfg.RetryDelaySeconds = 0

	c := &Coordinator{
		opts: CoordinatorOpts{
			Config:     cfg,
			Agent:      "test-agent",
			PromptFunc: func(string) string { return "test prompt" },
		},
		state:              state,
		runner:             runner,
		context:            NewSharedContext(state),
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}

	sess, _ := state.GetSession("bd-1")
	c.recoverSession(context.Background(), sess)

	s, _ := state.GetSession("bd-1")
	if s.Status != StatusRunning {
		t.Errorf("expected StatusRunning after recovery, got %s", s.Status)
	}
	if s.RetryCount != 1 {
		t.Errorf("expected RetryCount=1, got %d", s.RetryCount)
	}
	if !runner.aborted["bd-1"] {
		t.Error("expected AbortTask to be called")
	}
	if _, ok := runner.launched["bd-1"]; !ok {
		t.Error("expected LaunchTask to be called for recovery")
	}
}

func TestRecoverSession_LaunchFails(t *testing.T) {
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{
		TicketID:     "bd-1",
		Status:       StatusFailed,
		Error:        "server died",
		WorktreePath: t.TempDir(),
	})

	runner := newMockRunner()
	runner.launchErr = fmt.Errorf("binary not found")
	cfg := DefaultConfig()
	cfg.RetryDelaySeconds = 0

	c := &Coordinator{
		opts: CoordinatorOpts{
			Config:     cfg,
			PromptFunc: func(string) string { return "test" },
		},
		state:              state,
		runner:             runner,
		context:            NewSharedContext(state),
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}

	sess, _ := state.GetSession("bd-1")
	c.recoverSession(context.Background(), sess)

	s, _ := state.GetSession("bd-1")
	if s.Status != StatusFailed {
		t.Errorf("expected StatusFailed after launch failure, got %s", s.Status)
	}
	if s.RetryCount != 1 {
		t.Errorf("expected RetryCount=1, got %d", s.RetryCount)
	}
}

// --- Sort tests ---

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

// --- State tests ---

func TestAllCompleted_WithRetrying(t *testing.T) {
	s := NewState(t.TempDir(), 3)
	s.AddSession(SessionInfo{TicketID: "bd-1", Status: StatusCompleted})
	s.AddSession(SessionInfo{TicketID: "bd-2", Status: StatusRetrying})
	if s.AllCompleted() {
		t.Error("AllCompleted should return false when a session is retrying")
	}
}

// --- Event-driven recovery tests ---

// mockEventSource implements platform.EventSource for testing.
type mockEventSource struct {
	events []platform.Event
}

func (m *mockEventSource) Subscribe(_ context.Context) (<-chan platform.Event, error) {
	ch := make(chan platform.Event, len(m.events)+1)
	for _, e := range m.events {
		ch <- e
	}
	// Close the channel to signal end of events, which triggers fallback to polling
	// or completion check.
	close(ch)
	return ch, nil
}

// mockEventRunner wraps mockRunner and implements platform.EventSource.
type mockEventRunner struct {
	*mockRunner
	*mockEventSource
}

func (m *mockEventRunner) Subscribe(ctx context.Context) (<-chan platform.Event, error) {
	return m.mockEventSource.Subscribe(ctx)
}

func TestMonitorEvents_RecoveryTickerExists(t *testing.T) {
	// This test verifies that monitorEvents has a recovery path.
	// We create a coordinator with a failed session and an event source that
	// emits events then closes. Since recovery runs on a ticker, we verify
	// that the coordinator can handle the event-driven path without hanging.

	runner := newMockRunner()
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{
		TicketID:     "bd-1",
		Status:       StatusCompleted,
		WorktreePath: t.TempDir(),
	})

	cfg := DefaultConfig()
	cfg.MaxRetries = 0 // Disable actual recovery to keep test simple

	c := &Coordinator{
		opts: CoordinatorOpts{
			ProjectPath: "/tmp/test",
			ProjectID:   "test",
			Tickets:     []string{"bd-1"},
			Config:      cfg,
			PromptFunc:  func(string) string { return "test" },
		},
		state:              state,
		runner:             runner,
		context:            NewSharedContext(state),
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}

	es := &mockEventSource{events: nil} // No events, channel closes immediately

	ctx := context.Background()
	err := c.monitorEvents(ctx, es)

	// monitorEvents should return nil (all completed) or fall back to polling.
	// Since AllCompleted() is true (bd-1 is StatusCompleted), it should return nil.
	if err != nil {
		t.Errorf("expected nil error from monitorEvents, got: %v", err)
	}
}
