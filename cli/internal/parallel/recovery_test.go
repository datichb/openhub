package parallel

import (
	"context"
	"testing"
)

func TestAttemptRecovery_SkipsNonFailed(t *testing.T) {
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{TicketID: "bd-1", Status: StatusRunning})
	state.AddSession(SessionInfo{TicketID: "bd-2", Status: StatusCompleted})
	state.AddSession(SessionInfo{TicketID: "bd-3", Status: StatusIdle})

	cfg := DefaultConfig()
	c := &Coordinator{
		opts:              CoordinatorOpts{Config: cfg, PromptFunc: func(string) string { return "" }},
		state:             state,
		servers:           []*OpenCodeServer{},
		notifiedConflicts: make(map[string]bool),
	}

	// Should not panic or modify any session
	c.attemptRecovery(context.Background())

	for _, id := range []string{"bd-1", "bd-2", "bd-3"} {
		s, _ := state.GetSession(id)
		if s.RetryCount != 0 {
			t.Errorf("session %s should not have been retried, got RetryCount=%d", id, s.RetryCount)
		}
	}
}

func TestAttemptRecovery_SkipsExhaustedRetries(t *testing.T) {
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{TicketID: "bd-1", Status: StatusFailed, RetryCount: 2})

	cfg := DefaultConfig()
	cfg.MaxRetries = 2
	c := &Coordinator{
		opts:              CoordinatorOpts{Config: cfg, PromptFunc: func(string) string { return "" }},
		state:             state,
		servers:           []*OpenCodeServer{},
		notifiedConflicts: make(map[string]bool),
	}

	c.attemptRecovery(context.Background())

	s, _ := state.GetSession("bd-1")
	if s.Status != StatusFailed {
		t.Errorf("expected StatusFailed (exhausted), got %s", s.Status)
	}
	if s.RetryCount != 2 {
		t.Errorf("expected RetryCount=2 (unchanged), got %d", s.RetryCount)
	}
}

func TestAttemptRecovery_DisabledWhenMaxRetriesZero(t *testing.T) {
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{TicketID: "bd-1", Status: StatusFailed})

	cfg := DefaultConfig()
	cfg.MaxRetries = 0
	c := &Coordinator{
		opts:              CoordinatorOpts{Config: cfg, PromptFunc: func(string) string { return "" }},
		state:             state,
		servers:           []*OpenCodeServer{},
		notifiedConflicts: make(map[string]bool),
	}

	c.attemptRecovery(context.Background())

	s, _ := state.GetSession("bd-1")
	if s.Status != StatusFailed {
		t.Errorf("expected StatusFailed (recovery disabled), got %s", s.Status)
	}
}

func TestRecoverSession_TransitionsToRetrying(t *testing.T) {
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{
		TicketID:     "bd-1",
		Status:       StatusFailed,
		Port:         4100,
		Error:        "server process died",
		WorktreePath: t.TempDir(),
	})

	cfg := DefaultConfig()
	cfg.RetryDelaySeconds = 0 // no delay in tests

	// Create a real-ish server (with http client) so Dispose() doesn't panic
	srv := NewServer(4100, t.TempDir(), "bd-1")

	c := &Coordinator{
		opts: CoordinatorOpts{
			Config:     cfg,
			Agent:      "test-agent",
			PromptFunc: func(string) string { return "test prompt" },
		},
		state:              state,
		servers:            []*OpenCodeServer{srv},
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
		opencodeBin:        "/nonexistent/bin", // will fail at Start()
	}

	sess, _ := state.GetSession("bd-1")
	c.recoverSession(context.Background(), sess)

	// The recovery will fail at Start() because the binary doesn't exist,
	// but we can verify the state transitions happened correctly
	s, _ := state.GetSession("bd-1")

	// Should be back to Failed (because Start failed), with incremented RetryCount
	if s.Status != StatusFailed {
		t.Errorf("expected StatusFailed (start failed), got %s", s.Status)
	}
	if s.RetryCount != 1 {
		t.Errorf("expected RetryCount=1, got %d", s.RetryCount)
	}
	if len(s.RetryErrors) != 1 || s.RetryErrors[0] != "server process died" {
		t.Errorf("expected RetryErrors=[server process died], got %v", s.RetryErrors)
	}
}

func TestRecoverSession_PortOffset(t *testing.T) {
	state := NewState(t.TempDir(), 3)
	state.AddSession(SessionInfo{
		TicketID:     "bd-1",
		Status:       StatusFailed,
		Port:         4100,
		RetryCount:   1,
		WorktreePath: t.TempDir(),
	})

	cfg := DefaultConfig()
	cfg.RetryDelaySeconds = 0
	srv := NewServer(4100, t.TempDir(), "bd-1")

	c := &Coordinator{
		opts: CoordinatorOpts{
			Config:     cfg,
			PromptFunc: func(string) string { return "" },
		},
		state:              state,
		servers:            []*OpenCodeServer{srv},
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
		opencodeBin:        "/nonexistent/bin",
	}

	sess, _ := state.GetSession("bd-1")
	c.recoverSession(context.Background(), sess)

	s, _ := state.GetSession("bd-1")
	// Port should be original + 10 + retryCount (which was 1 before recovery, now 2)
	// newPort = 4100 + 10 + 1 = 4111 (retryCount at entry was 1)
	expectedPort := 4100 + 10 + 1
	if s.Port != expectedPort {
		t.Errorf("expected port=%d, got %d", expectedPort, s.Port)
	}
}

func TestFindServer(t *testing.T) {
	c := &Coordinator{
		servers: []*OpenCodeServer{
			{TicketID: "bd-1", Port: 4100},
			{TicketID: "bd-2", Port: 4101},
		},
	}

	srv := c.findServer("bd-2")
	if srv == nil || srv.Port != 4101 {
		t.Error("findServer should return the server for bd-2")
	}

	srv = c.findServer("bd-99")
	if srv != nil {
		t.Error("findServer should return nil for unknown ticket")
	}
}

func TestReplaceServer(t *testing.T) {
	c := &Coordinator{
		servers: []*OpenCodeServer{
			{TicketID: "bd-1", Port: 4100},
			{TicketID: "bd-2", Port: 4101},
		},
	}

	newSrv := &OpenCodeServer{TicketID: "bd-1", Port: 4200}
	c.replaceServer("bd-1", newSrv)

	if c.servers[0].Port != 4200 {
		t.Errorf("expected replaced server port=4200, got %d", c.servers[0].Port)
	}
	if len(c.servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(c.servers))
	}
}

func TestReplaceServer_AppendNew(t *testing.T) {
	c := &Coordinator{
		servers: []*OpenCodeServer{
			{TicketID: "bd-1", Port: 4100},
		},
	}

	newSrv := &OpenCodeServer{TicketID: "bd-99", Port: 4200}
	c.replaceServer("bd-99", newSrv)

	if len(c.servers) != 2 {
		t.Errorf("expected 2 servers after append, got %d", len(c.servers))
	}
}

func TestAllCompleted_WithRetrying(t *testing.T) {
	s := NewState(t.TempDir(), 3)
	s.AddSession(SessionInfo{TicketID: "bd-1", Status: StatusCompleted})
	s.AddSession(SessionInfo{TicketID: "bd-2", Status: StatusRetrying})

	if s.AllCompleted() {
		t.Error("AllCompleted should return false when a session is retrying")
	}
}

func TestAllCompleted_RetryingThenCompleted(t *testing.T) {
	s := NewState(t.TempDir(), 3)
	s.AddSession(SessionInfo{TicketID: "bd-1", Status: StatusRetrying})

	if s.AllCompleted() {
		t.Error("should not be completed while retrying")
	}

	s.UpdateSession("bd-1", func(si *SessionInfo) {
		si.Status = StatusCompleted
	})

	if !s.AllCompleted() {
		t.Error("should be completed after retry succeeds")
	}
}
