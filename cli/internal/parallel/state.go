// Package parallel manages parallel execution of multiple opencode sessions.
package parallel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionStatus represents the state of a parallel session.
type SessionStatus string

const (
	StatusPending    SessionStatus = "pending"
	StatusStarting   SessionStatus = "starting"
	StatusRunning    SessionStatus = "running"
	StatusCompleted  SessionStatus = "completed"
	StatusFailed     SessionStatus = "failed"
	StatusAborted    SessionStatus = "aborted"
)

// SessionInfo holds state for a single parallel session.
type SessionInfo struct {
	TicketID      string        `json:"ticket_id"`
	Project       string        `json:"project"`
	Branch        string        `json:"branch"`
	WorktreePath  string        `json:"worktree_path"`
	Port          int           `json:"port"`
	SessionID     string        `json:"session_id"`     // opencode session ID
	Status        SessionStatus `json:"status"`
	Priority      bool          `json:"priority"`       // is this the priority ticket
	StartedAt     time.Time     `json:"started_at,omitempty"`
	CompletedAt   time.Time     `json:"completed_at,omitempty"`
	Error         string        `json:"error,omitempty"`
	FilesModified []string      `json:"files_modified,omitempty"`
	FilesCreated  []string      `json:"files_created,omitempty"`
}

// ConflictInfo represents a potential file conflict between sessions.
type ConflictInfo struct {
	File     string   `json:"file"`
	Sessions []string `json:"sessions"` // ticket IDs that touch this file
	Severity string   `json:"severity"` // low | medium | high
}

// ParallelState is the complete state of a parallel run.
type ParallelState struct {
	mu sync.RWMutex

	StartedAt   time.Time      `json:"started_at"`
	ProjectPath string         `json:"project_path"`
	MaxSessions int            `json:"max_sessions"`
	Sessions    []SessionInfo  `json:"sessions"`
	Conflicts   []ConflictInfo `json:"conflicts,omitempty"`
	Phase       string         `json:"phase"` // setup | running | merging | done
}

// NewState creates a new parallel state.
func NewState(projectPath string, maxSessions int) *ParallelState {
	return &ParallelState{
		StartedAt:   time.Now().UTC(),
		ProjectPath: projectPath,
		MaxSessions: maxSessions,
		Sessions:    make([]SessionInfo, 0),
		Phase:       "setup",
	}
}

// AddSession adds a session to the state.
func (s *ParallelState) AddSession(info SessionInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Sessions = append(s.Sessions, info)
}

// UpdateSession updates a session by ticket ID.
func (s *ParallelState) UpdateSession(ticketID string, fn func(*SessionInfo)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Sessions {
		if s.Sessions[i].TicketID == ticketID {
			fn(&s.Sessions[i])
			return
		}
	}
}

// GetSession returns a copy of a session by ticket ID.
func (s *ParallelState) GetSession(ticketID string) (SessionInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sess := range s.Sessions {
		if sess.TicketID == ticketID {
			return sess, true
		}
	}
	return SessionInfo{}, false
}

// AllCompleted returns true if all sessions are in a terminal state.
func (s *ParallelState) AllCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sess := range s.Sessions {
		if sess.Status != StatusCompleted && sess.Status != StatusFailed && sess.Status != StatusAborted {
			return false
		}
	}
	return len(s.Sessions) > 0
}

// RunningCount returns the number of sessions currently running.
func (s *ParallelState) RunningCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, sess := range s.Sessions {
		if sess.Status == StatusRunning || sess.Status == StatusStarting {
			count++
		}
	}
	return count
}

// SetPhase sets the current phase.
func (s *ParallelState) SetPhase(phase string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Phase = phase
}

// SetConflicts updates the detected conflicts.
func (s *ParallelState) SetConflicts(conflicts []ConflictInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Conflicts = conflicts
}

// StateSnapshot is a copy of ParallelState without the mutex (safe to copy/return).
type StateSnapshot struct {
	StartedAt   time.Time      `json:"started_at"`
	ProjectPath string         `json:"project_path"`
	MaxSessions int            `json:"max_sessions"`
	Sessions    []SessionInfo  `json:"sessions"`
	Conflicts   []ConflictInfo `json:"conflicts,omitempty"`
	Phase       string         `json:"phase"`
}

// Snapshot returns a thread-safe copy of the state (without the mutex).
func (s *ParallelState) Snapshot() StateSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := StateSnapshot{
		StartedAt:   s.StartedAt,
		ProjectPath: s.ProjectPath,
		MaxSessions: s.MaxSessions,
		Phase:       s.Phase,
		Sessions:    make([]SessionInfo, len(s.Sessions)),
		Conflicts:   make([]ConflictInfo, len(s.Conflicts)),
	}
	copy(snap.Sessions, s.Sessions)
	copy(snap.Conflicts, s.Conflicts)
	return snap
}

// Save persists the state to a JSON file.
func (s *ParallelState) Save(dir string) error {
	snap := s.Snapshot()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "parallel-state.json")
	data, err := json.MarshalIndent(&snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling parallel state: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadState reads a parallel state from disk.
func LoadState(dir string) (*ParallelState, error) {
	path := filepath.Join(dir, "parallel-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state ParallelState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing parallel state: %w", err)
	}
	return &state, nil
}
