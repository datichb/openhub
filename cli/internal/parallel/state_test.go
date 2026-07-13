package parallel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewState(t *testing.T) {
	s := NewState("/tmp/project", 3)
	if s.MaxSessions != 3 {
		t.Errorf("expected MaxSessions=3, got %d", s.MaxSessions)
	}
	if s.Phase != "setup" {
		t.Errorf("expected Phase=setup, got %s", s.Phase)
	}
	if s.ProjectPath != "/tmp/project" {
		t.Errorf("expected ProjectPath=/tmp/project, got %s", s.ProjectPath)
	}
}

func TestAddAndGetSession(t *testing.T) {
	s := NewState("/tmp/project", 3)

	s.AddSession(SessionInfo{
		TicketID: "bd-42",
		Project:  "T-SRU",
		Branch:   "feat/bd-42",
		Status:   StatusPending,
	})

	sess, ok := s.GetSession("bd-42")
	if !ok {
		t.Fatal("session bd-42 not found")
	}
	if sess.TicketID != "bd-42" {
		t.Errorf("expected bd-42, got %s", sess.TicketID)
	}
	if sess.Status != StatusPending {
		t.Errorf("expected pending, got %s", sess.Status)
	}

	// Not found
	_, ok = s.GetSession("bd-99")
	if ok {
		t.Error("should not find bd-99")
	}
}

func TestUpdateSession(t *testing.T) {
	s := NewState("/tmp/project", 3)
	s.AddSession(SessionInfo{TicketID: "bd-42", Status: StatusPending})

	s.UpdateSession("bd-42", func(si *SessionInfo) {
		si.Status = StatusRunning
		si.Port = 4100
	})

	sess, _ := s.GetSession("bd-42")
	if sess.Status != StatusRunning {
		t.Errorf("expected running, got %s", sess.Status)
	}
	if sess.Port != 4100 {
		t.Errorf("expected port 4100, got %d", sess.Port)
	}
}

func TestAllCompleted(t *testing.T) {
	s := NewState("/tmp/project", 3)
	s.AddSession(SessionInfo{TicketID: "bd-42", Status: StatusRunning})
	s.AddSession(SessionInfo{TicketID: "bd-43", Status: StatusCompleted})

	if s.AllCompleted() {
		t.Error("should not be all completed (bd-42 still running)")
	}

	s.UpdateSession("bd-42", func(si *SessionInfo) {
		si.Status = StatusCompleted
	})
	if !s.AllCompleted() {
		t.Error("should be all completed now")
	}
}

func TestAllCompleted_WithFailed(t *testing.T) {
	s := NewState("/tmp/project", 3)
	s.AddSession(SessionInfo{TicketID: "bd-42", Status: StatusFailed})
	s.AddSession(SessionInfo{TicketID: "bd-43", Status: StatusCompleted})

	if !s.AllCompleted() {
		t.Error("failed + completed should count as all completed")
	}
}

func TestAllCompleted_Empty(t *testing.T) {
	s := NewState("/tmp/project", 3)
	if s.AllCompleted() {
		t.Error("empty state should not be all completed")
	}
}

func TestRunningCount(t *testing.T) {
	s := NewState("/tmp/project", 3)
	s.AddSession(SessionInfo{TicketID: "bd-42", Status: StatusRunning})
	s.AddSession(SessionInfo{TicketID: "bd-43", Status: StatusStarting})
	s.AddSession(SessionInfo{TicketID: "bd-44", Status: StatusCompleted})

	if s.RunningCount() != 2 {
		t.Errorf("expected 2 running, got %d", s.RunningCount())
	}
}

func TestSetPhase(t *testing.T) {
	s := NewState("/tmp/project", 3)
	s.SetPhase("running")
	snap := s.Snapshot()
	if snap.Phase != "running" {
		t.Errorf("expected phase=running, got %s", snap.Phase)
	}
}

func TestSnapshot_IsCopy(t *testing.T) {
	s := NewState("/tmp/project", 3)
	s.AddSession(SessionInfo{TicketID: "bd-42", Status: StatusRunning})

	snap := s.Snapshot()

	// Modify original — snapshot should not change
	s.UpdateSession("bd-42", func(si *SessionInfo) {
		si.Status = StatusCompleted
	})

	if snap.Sessions[0].Status != StatusRunning {
		t.Error("snapshot should not be affected by state changes")
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	s := NewState("/tmp/project", 3)
	s.AddSession(SessionInfo{
		TicketID: "bd-42",
		Project:  "T-SRU",
		Branch:   "feat/bd-42",
		Port:     4100,
		Status:   StatusRunning,
		StartedAt: time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC),
	})
	s.SetPhase("running")

	if err := s.Save(dir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "parallel-state.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("state file not created")
	}

	// Load back
	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}
	if loaded.MaxSessions != 3 {
		t.Errorf("expected MaxSessions=3, got %d", loaded.MaxSessions)
	}
	if loaded.Phase != "running" {
		t.Errorf("expected Phase=running, got %s", loaded.Phase)
	}
	if len(loaded.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(loaded.Sessions))
	}
	if loaded.Sessions[0].TicketID != "bd-42" {
		t.Errorf("expected bd-42, got %s", loaded.Sessions[0].TicketID)
	}
}

func TestLoadState_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadState(dir)
	if err == nil {
		t.Error("expected error for missing state file")
	}
}
