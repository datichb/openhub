package opencode

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/platform"
)

// --- fakeServer implements serverProvider for tests ---

type fakeServer struct {
	started      bool
	ready        bool
	alive        bool
	disposed     bool
	killed       bool
	startErr     error
	readyErr     error
	sessionID    string
	createErr    error
	promptSent   string
	promptErr    error
	notifSent    string
	statuses     map[string]string
	statusErr    error
	files        []platform.FileChange
	filesErr     error
	abortCalled  bool
	abortErr     error
	portVal      int
	dirVal       string
}

func (f *fakeServer) start(_ context.Context) error  { f.started = true; return f.startErr }
func (f *fakeServer) waitReady(_ context.Context, _ time.Duration) error {
	if f.readyErr != nil {
		return f.readyErr
	}
	f.ready = true
	return nil
}
func (f *fakeServer) isAlive() bool                   { return f.alive }
func (f *fakeServer) dispose() error                  { f.disposed = true; return nil }
func (f *fakeServer) kill()                           { f.killed = true }
func (f *fakeServer) port() int                       { return f.portVal }
func (f *fakeServer) dir() string                     { return f.dirVal }

func (f *fakeServer) createSession(title string) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	return f.sessionID, nil
}
func (f *fakeServer) sendPrompt(sessionID, prompt, agent string) error {
	f.promptSent = prompt
	return f.promptErr
}
func (f *fakeServer) sendNotification(sessionID, message string) error {
	f.notifSent = message
	return nil
}
func (f *fakeServer) getStatus() (map[string]string, error) {
	return f.statuses, f.statusErr
}
func (f *fakeServer) getModifiedFiles() ([]platform.FileChange, error) {
	return f.files, f.filesErr
}
func (f *fakeServer) abortSession(sessionID string) error {
	f.abortCalled = true
	return f.abortErr
}

// --- Test helper: build a runner with a fake factory ---

func newTestRunner(t *testing.T, factory serverProviderFactory) *OpenCodeParallelRunner {
	t.Helper()
	return &OpenCodeParallelRunner{
		servers:       make(map[string]serverProvider),
		sessionIDs:    make(map[string]string),
		opts:          platform.ParallelRunnerOpts{},
		portBase:      5000,
		portNext:      5000,
		bin:           "/fake/opencode",
		serverFactory: factory,
	}
}

// --- LaunchTask tests ---

func TestLaunchTask_Success(t *testing.T) {
	srv := &fakeServer{alive: true, sessionID: "sess-1"}
	runner := newTestRunner(t, func(port int, dir, id, bin string) serverProvider {
		return srv
	})

	handle, err := runner.LaunchTask(context.Background(), platform.TaskOpts{
		TaskID:       "task-1",
		Title:        "Test task",
		WorktreePath: "/tmp/wt",
		Prompt:       "do something",
		Agent:        "dev",
	})
	if err != nil {
		t.Fatalf("LaunchTask error: %v", err)
	}
	if handle.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want task-1", handle.TaskID)
	}
	if handle.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want sess-1", handle.SessionID)
	}
	if !srv.started {
		t.Error("expected server to be started")
	}
	if srv.promptSent != "do something" {
		t.Errorf("prompt = %q, want 'do something'", srv.promptSent)
	}
}

func TestLaunchTask_StartFailRetrySuccess(t *testing.T) {
	callCount := int32(0)
	runner := newTestRunner(t, func(port int, dir, id, bin string) serverProvider {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			return &fakeServer{startErr: fmt.Errorf("port busy"), alive: true, sessionID: "s1"}
		}
		return &fakeServer{alive: true, sessionID: "s1"}
	})

	handle, err := runner.LaunchTask(context.Background(), platform.TaskOpts{
		TaskID: "task-retry",
		Prompt: "test",
	})
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if handle.SessionID != "s1" {
		t.Errorf("SessionID = %q, want s1", handle.SessionID)
	}
	// Should have used 2 ports (5000 failed, 5001 succeeded)
	if runner.portNext != 5002 {
		t.Errorf("portNext = %d, want 5002", runner.portNext)
	}
}

func TestLaunchTask_AllStartsFail(t *testing.T) {
	runner := newTestRunner(t, func(port int, dir, id, bin string) serverProvider {
		return &fakeServer{startErr: fmt.Errorf("port busy")}
	})

	_, err := runner.LaunchTask(context.Background(), platform.TaskOpts{
		TaskID: "task-fail",
		Prompt: "test",
	})
	if err == nil {
		t.Error("expected error when all starts fail")
	}
}

func TestLaunchTask_CreateSessionFails(t *testing.T) {
	srv := &fakeServer{alive: true, createErr: fmt.Errorf("session limit")}
	runner := newTestRunner(t, func(port int, dir, id, bin string) serverProvider {
		return srv
	})

	_, err := runner.LaunchTask(context.Background(), platform.TaskOpts{
		TaskID: "task-cs",
		Prompt: "test",
	})
	if err == nil {
		t.Error("expected error when create session fails")
	}
	if !srv.killed {
		t.Error("expected server to be killed on failure")
	}
}

// --- GetAllStatuses tests ---

func TestGetAllStatuses_Mixed(t *testing.T) {
	runner := newTestRunner(t, nil)

	runner.servers["t1"] = &fakeServer{
		alive:    true,
		statuses: map[string]string{"s1": "completed"},
		files:    []platform.FileChange{{Path: "main.go", Operation: "modified"}},
	}
	runner.sessionIDs["t1"] = "s1"

	runner.servers["t2"] = &fakeServer{
		alive:    true,
		statuses: map[string]string{"s2": "running"},
	}
	runner.sessionIDs["t2"] = "s2"

	runner.servers["t3"] = &fakeServer{alive: false}
	runner.sessionIDs["t3"] = "s3"

	statuses, err := runner.GetAllStatuses(context.Background())
	if err != nil {
		t.Fatalf("GetAllStatuses error: %v", err)
	}
	if statuses["t1"].Status != "completed" {
		t.Errorf("t1 status = %q, want completed", statuses["t1"].Status)
	}
	if len(statuses["t1"].FilesModified) != 1 || statuses["t1"].FilesModified[0] != "main.go" {
		t.Errorf("t1 files = %v, want [main.go]", statuses["t1"].FilesModified)
	}
	if statuses["t2"].Status != "running" {
		t.Errorf("t2 status = %q, want running", statuses["t2"].Status)
	}
	if statuses["t3"].Status != "failed" {
		t.Errorf("t3 status = %q, want failed (dead server)", statuses["t3"].Status)
	}
}

func TestGetAllStatuses_Empty(t *testing.T) {
	runner := newTestRunner(t, nil)
	statuses, err := runner.GetAllStatuses(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(statuses) != 0 {
		t.Errorf("expected empty statuses, got %d", len(statuses))
	}
}

// --- SendMessage tests ---

func TestSendMessage_Success(t *testing.T) {
	srv := &fakeServer{alive: true}
	runner := newTestRunner(t, nil)
	runner.servers["t1"] = srv
	runner.sessionIDs["t1"] = "s1"

	err := runner.SendMessage(context.Background(), "t1", "hello")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if srv.notifSent != "hello" {
		t.Errorf("notification = %q, want 'hello'", srv.notifSent)
	}
}

func TestSendMessage_TaskNotFound(t *testing.T) {
	runner := newTestRunner(t, nil)
	err := runner.SendMessage(context.Background(), "unknown", "hello")
	if err == nil {
		t.Error("expected error for unknown task")
	}
}

func TestSendMessage_NoSession(t *testing.T) {
	runner := newTestRunner(t, nil)
	runner.servers["t1"] = &fakeServer{}
	runner.sessionIDs["t1"] = "" // no session yet

	err := runner.SendMessage(context.Background(), "t1", "hello")
	if err == nil {
		t.Error("expected error when no session ID")
	}
}

// --- AbortTask tests ---

func TestAbortTask_Success(t *testing.T) {
	srv := &fakeServer{alive: true}
	runner := newTestRunner(t, nil)
	runner.servers["t1"] = srv
	runner.sessionIDs["t1"] = "s1"

	err := runner.AbortTask(context.Background(), "t1")
	if err != nil {
		t.Fatalf("AbortTask error: %v", err)
	}
	if !srv.abortCalled {
		t.Error("expected abortSession to be called")
	}
	if !srv.disposed {
		t.Error("expected dispose to be called")
	}
	if !srv.killed {
		t.Error("expected kill to be called")
	}
	// Task should be removed from the runner
	if _, ok := runner.servers["t1"]; ok {
		t.Error("expected task to be removed from servers map")
	}
	if _, ok := runner.sessionIDs["t1"]; ok {
		t.Error("expected task to be removed from sessionIDs map")
	}
}

func TestAbortTask_NotFound(t *testing.T) {
	runner := newTestRunner(t, nil)
	err := runner.AbortTask(context.Background(), "unknown")
	if err == nil {
		t.Error("expected error for unknown task")
	}
}

// --- Cleanup tests ---

func TestCleanup(t *testing.T) {
	srv1 := &fakeServer{alive: true}
	srv2 := &fakeServer{alive: true}
	runner := newTestRunner(t, nil)
	runner.servers["t1"] = srv1
	runner.servers["t2"] = srv2
	runner.sessionIDs["t1"] = "s1"
	runner.sessionIDs["t2"] = "s2"

	runner.Cleanup(context.Background())

	if !srv1.disposed || !srv1.killed {
		t.Error("expected srv1 to be disposed and killed")
	}
	if !srv2.disposed || !srv2.killed {
		t.Error("expected srv2 to be disposed and killed")
	}
	if len(runner.servers) != 0 {
		t.Errorf("expected empty servers map, got %d", len(runner.servers))
	}
	if len(runner.sessionIDs) != 0 {
		t.Errorf("expected empty sessionIDs map, got %d", len(runner.sessionIDs))
	}
}

// --- Port allocation tests ---

func TestPortAllocation_Increments(t *testing.T) {
	runner := newTestRunner(t, func(port int, dir, id, bin string) serverProvider {
		return &fakeServer{alive: true, sessionID: "s-" + id, portVal: port}
	})

	_, _ = runner.LaunchTask(context.Background(), platform.TaskOpts{TaskID: "t1", Prompt: "a"})
	_, _ = runner.LaunchTask(context.Background(), platform.TaskOpts{TaskID: "t2", Prompt: "b"})

	// t1 uses port 5000, t2 uses port 5001
	if runner.portNext != 5002 {
		t.Errorf("portNext = %d, want 5002", runner.portNext)
	}
}

// --- GetModifiedFiles tests ---

func TestGetModifiedFiles_Success(t *testing.T) {
	runner := newTestRunner(t, nil)
	runner.servers["t1"] = &fakeServer{
		files: []platform.FileChange{
			{Path: "a.go", Operation: "modified"},
			{Path: "b.go", Operation: "created"},
		},
	}

	files, err := runner.GetModifiedFiles(context.Background(), "t1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestGetModifiedFiles_NotFound(t *testing.T) {
	runner := newTestRunner(t, nil)
	_, err := runner.GetModifiedFiles(context.Background(), "unknown")
	if err == nil {
		t.Error("expected error for unknown task")
	}
}
