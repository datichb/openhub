package opencode

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// --- IsGhostSession tests ---

func TestIsGhostSession_RunningOver24h(t *testing.T) {
	s := domain.Session{
		Status:    domain.SessionStatusRunning,
		StartedAt: time.Now().Add(-25 * time.Hour),
	}
	if !IsGhostSession(s) {
		t.Error("expected ghost: running session older than 24h")
	}
}

func TestIsGhostSession_RunningUnder24h(t *testing.T) {
	s := domain.Session{
		Status:    domain.SessionStatusRunning,
		StartedAt: time.Now().Add(-23 * time.Hour),
	}
	if IsGhostSession(s) {
		t.Error("expected NOT ghost: running session under 24h")
	}
}

func TestIsGhostSession_CompletedOver24h(t *testing.T) {
	s := domain.Session{
		Status:    domain.SessionStatusCompleted,
		StartedAt: time.Now().Add(-48 * time.Hour),
	}
	if IsGhostSession(s) {
		t.Error("expected NOT ghost: completed sessions are never ghost regardless of age")
	}
}

func TestIsGhostSession_FailedOver24h(t *testing.T) {
	s := domain.Session{
		Status:    domain.SessionStatusFailed,
		StartedAt: time.Now().Add(-48 * time.Hour),
	}
	if IsGhostSession(s) {
		t.Error("expected NOT ghost: failed sessions are never ghost")
	}
}

func TestIsGhostSession_RunningJustNow(t *testing.T) {
	s := domain.Session{
		Status:    domain.SessionStatusRunning,
		StartedAt: time.Now().Add(-5 * time.Minute),
	}
	if IsGhostSession(s) {
		t.Error("expected NOT ghost: session started 5 minutes ago")
	}
}

// --- FormatActiveSessionWarning tests ---

func TestFormatActiveSessionWarning_Empty(t *testing.T) {
	result := FormatActiveSessionWarning(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatActiveSessionWarning_Single(t *testing.T) {
	sessions := []ActiveSessionInfo{
		{SessionID: "s1", StartedAt: time.Now().Add(-45 * time.Minute)},
	}
	result := FormatActiveSessionWarning(sessions)
	if result == "" {
		t.Error("expected non-empty warning for single session")
	}
	if !contains(result, "Une session") {
		t.Errorf("expected 'Une session' in warning, got %q", result)
	}
}

func TestFormatActiveSessionWarning_Multiple(t *testing.T) {
	sessions := []ActiveSessionInfo{
		{SessionID: "s1", StartedAt: time.Now().Add(-10 * time.Minute)},
		{SessionID: "s2", StartedAt: time.Now().Add(-20 * time.Minute)},
	}
	result := FormatActiveSessionWarning(sessions)
	if !contains(result, "2 sessions") {
		t.Errorf("expected '2 sessions' in warning, got %q", result)
	}
}

// --- formatDuration tests ---

func TestFormatDuration_LessThanMinute(t *testing.T) {
	result := formatDuration(30 * time.Second)
	if result != "moins d'une minute" {
		t.Errorf("expected \"moins d'une minute\", got %q", result)
	}
}

func TestFormatDuration_Zero(t *testing.T) {
	result := formatDuration(0)
	if result != "moins d'une minute" {
		t.Errorf("expected \"moins d'une minute\", got %q", result)
	}
}

func TestFormatDuration_Minutes(t *testing.T) {
	result := formatDuration(45 * time.Minute)
	if result != "45min" {
		t.Errorf("expected \"45min\", got %q", result)
	}
}

func TestFormatDuration_HoursAndMinutes(t *testing.T) {
	result := formatDuration(2*time.Hour + 30*time.Minute)
	if result != "2h30min" {
		t.Errorf("expected \"2h30min\", got %q", result)
	}
}

func TestFormatDuration_HoursOnly(t *testing.T) {
	result := formatDuration(3 * time.Hour)
	if result != "3h" {
		t.Errorf("expected \"3h\", got %q", result)
	}
}

func TestFormatDuration_OneMinute(t *testing.T) {
	result := formatDuration(1 * time.Minute)
	if result != "1min" {
		t.Errorf("expected \"1min\", got %q", result)
	}
}

// --- mockSessionStore for FindActive* tests ---

type mockSessionStore struct {
	running []domain.Session
}

func (m *mockSessionStore) List(_ context.Context, _ string) ([]domain.Session, error) {
	return nil, nil
}

func (m *mockSessionStore) Get(_ context.Context, _ string) (*domain.Session, error) {
	return nil, fmt.Errorf("not found")
}

func (m *mockSessionStore) Create(_ context.Context, _ *domain.Session) error {
	return nil
}

func (m *mockSessionStore) Update(_ context.Context, _ *domain.Session) error {
	return nil
}

func (m *mockSessionStore) ListRunning(_ context.Context, _ string) ([]domain.Session, error) {
	return m.running, nil
}

// --- FindActiveSessionsOnPath tests ---

func TestFindActiveSessionsOnPath_NoRunning(t *testing.T) {
	store := &mockSessionStore{running: nil}
	result, err := FindActiveSessionsOnPath(context.Background(), store, "proj1", "/path/a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(result))
	}
}

func TestFindActiveSessionsOnPath_FiltersGhosts(t *testing.T) {
	store := &mockSessionStore{
		running: []domain.Session{
			{ID: "s1", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-5 * time.Minute), LaunchPath: "/path/a"},
			{ID: "s2", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-48 * time.Hour), LaunchPath: "/path/a"}, // ghost
		},
	}
	result, err := FindActiveSessionsOnPath(context.Background(), store, "proj1", "/path/a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 session (ghost filtered), got %d", len(result))
	}
	if result[0].SessionID != "s1" {
		t.Errorf("expected s1, got %s", result[0].SessionID)
	}
}

func TestFindActiveSessionsOnPath_FiltersPath(t *testing.T) {
	store := &mockSessionStore{
		running: []domain.Session{
			{ID: "s1", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-5 * time.Minute), LaunchPath: "/path/a"},
			{ID: "s2", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-5 * time.Minute), LaunchPath: "/path/b"},
		},
	}
	result, err := FindActiveSessionsOnPath(context.Background(), store, "proj1", "/path/a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 session (path filtered), got %d", len(result))
	}
	if result[0].SessionID != "s1" {
		t.Errorf("expected s1, got %s", result[0].SessionID)
	}
}

func TestFindActiveSessionsOnPath_EmptyPath_ReturnsAll(t *testing.T) {
	store := &mockSessionStore{
		running: []domain.Session{
			{ID: "s1", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-5 * time.Minute), LaunchPath: "/path/a"},
			{ID: "s2", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-5 * time.Minute), LaunchPath: "/path/b"},
		},
	}
	result, err := FindActiveSessionsOnPath(context.Background(), store, "proj1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 sessions (empty path = all), got %d", len(result))
	}
}

// --- FindAllActiveSessionsByPath tests ---

func TestFindAllActiveSessionsByPath_GroupsByPath(t *testing.T) {
	store := &mockSessionStore{
		running: []domain.Session{
			{ID: "s1", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-5 * time.Minute), LaunchPath: "/path/a"},
			{ID: "s2", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-5 * time.Minute), LaunchPath: "/path/b"},
			{ID: "s3", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-5 * time.Minute), LaunchPath: "/path/a"},
		},
	}
	result, err := FindAllActiveSessionsByPath(context.Background(), store, "proj1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(result))
	}
	if len(result["/path/a"]) != 2 {
		t.Errorf("expected 2 sessions for /path/a, got %d", len(result["/path/a"]))
	}
	if len(result["/path/b"]) != 1 {
		t.Errorf("expected 1 session for /path/b, got %d", len(result["/path/b"]))
	}
}

func TestFindAllActiveSessionsByPath_ExcludesGhosts(t *testing.T) {
	store := &mockSessionStore{
		running: []domain.Session{
			{ID: "s1", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-5 * time.Minute), LaunchPath: "/path/a"},
			{ID: "s2", Status: domain.SessionStatusRunning, StartedAt: time.Now().Add(-48 * time.Hour), LaunchPath: "/path/a"}, // ghost
		},
	}
	result, err := FindAllActiveSessionsByPath(context.Background(), store, "proj1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result["/path/a"]) != 1 {
		t.Errorf("expected 1 session (ghost excluded), got %d", len(result["/path/a"]))
	}
}

// --- helpers ---

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
