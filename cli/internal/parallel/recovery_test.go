package parallel

import (
	"context"
	"testing"

	"github.com/datichb/openhub/cli/internal/platform"
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
		servers:           []SessionServer{},
		platform:          testPlatform{},
		notifiedConflicts: make(map[string]bool),
	}

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
		servers:           []SessionServer{},
		platform:          testPlatform{},
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
		servers:           []SessionServer{},
		platform:          testPlatform{},
		notifiedConflicts: make(map[string]bool),
	}

	c.attemptRecovery(context.Background())

	s, _ := state.GetSession("bd-1")
	if s.Status != StatusFailed {
		t.Errorf("expected StatusFailed (recovery disabled), got %s", s.Status)
	}
}

// failingPlatform returns errors from NewServer to test recovery failure path.
type failingPlatform struct{ testPlatform }

func (failingPlatform) NewParallelRunner(_ platform.ParallelRunnerOpts) (platform.ParallelRunner, error) {
	return nil, nil
}

// failingServerFactory creates a ServerAdapter that will fail on Start (binary doesn't exist).
func failingServerFactory(port int, dir, id string) (SessionServer, error) {
	return NewServerAdapter(port, dir, id, "/nonexistent/bin"), nil
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
	cfg.RetryDelaySeconds = 0

	srv := NewServerAdapter(4100, t.TempDir(), "bd-1", "/nonexistent/bin")

	c := &Coordinator{
		opts: CoordinatorOpts{
			Config:        cfg,
			Agent:         "test-agent",
			PromptFunc:    func(string) string { return "test prompt" },
			ServerFactory: failingServerFactory,
		},
		state:              state,
		servers:            []SessionServer{srv},
		platform:           failingPlatform{},
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}

	sess, _ := state.GetSession("bd-1")
	c.recoverSession(context.Background(), sess)

	s, _ := state.GetSession("bd-1")
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
	srv := NewServerAdapter(4100, t.TempDir(), "bd-1", "/nonexistent/bin")

	c := &Coordinator{
		opts: CoordinatorOpts{
			Config:        cfg,
			PromptFunc:    func(string) string { return "" },
			ServerFactory: failingServerFactory,
		},
		state:              state,
		servers:            []SessionServer{srv},
		platform:           failingPlatform{},
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}

	sess, _ := state.GetSession("bd-1")
	c.recoverSession(context.Background(), sess)

	s, _ := state.GetSession("bd-1")
	expectedPort := 4100 + 10 + 1
	if s.Port != expectedPort {
		t.Errorf("expected port=%d, got %d", expectedPort, s.Port)
	}
}

func TestFindServer(t *testing.T) {
	s1 := NewServerAdapter(4100, "/tmp/a", "bd-1", "")
	s2 := NewServerAdapter(4101, "/tmp/b", "bd-2", "")

	c := &Coordinator{
		servers: []SessionServer{s1, s2},
	}

	srv := c.findServer("bd-2")
	if srv == nil || srv.Port() != 4101 {
		t.Error("findServer should return the server for bd-2")
	}

	srv = c.findServer("bd-99")
	if srv != nil {
		t.Error("findServer should return nil for unknown ticket")
	}
}

func TestReplaceServer(t *testing.T) {
	s1 := NewServerAdapter(4100, "/tmp/a", "bd-1", "")
	s2 := NewServerAdapter(4101, "/tmp/b", "bd-2", "")

	c := &Coordinator{
		servers: []SessionServer{s1, s2},
	}

	newSrv := NewServerAdapter(4200, "/tmp/a", "bd-1", "")
	c.replaceServer("bd-1", newSrv)

	if c.servers[0].Port() != 4200 {
		t.Errorf("expected replaced server port=4200, got %d", c.servers[0].Port())
	}
	if len(c.servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(c.servers))
	}
}

func TestReplaceServer_AppendNew(t *testing.T) {
	s1 := NewServerAdapter(4100, "/tmp/a", "bd-1", "")

	c := &Coordinator{
		servers: []SessionServer{s1},
	}

	newSrv := NewServerAdapter(4200, "/tmp/b", "bd-99", "")
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
