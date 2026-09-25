package opencode

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/platform"

	_ "modernc.org/sqlite"
)

// --- Conversion function tests (pure, no DB) ---

func TestToAggregateStats_Nil(t *testing.T) {
	result := toAggregateStats(nil)
	if result == nil {
		t.Fatal("expected non-nil result for nil input")
	}
	if result.TotalSessions != 0 || result.TotalCost != 0 {
		t.Error("expected zero-valued stats for nil input")
	}
}

func TestToAggregateStats_Populated(t *testing.T) {
	input := &AggregateStats{
		TotalSessions:   42,
		TotalTokensIn:   1000,
		TotalTokensOut:  500,
		TotalCost:       12.34,
		TodaySessions:   3,
		ActiveProjects:  2,
		CacheReadTokens: 200,
		ReasoningTokens: 100,
	}
	result := toAggregateStats(input)
	if result.TotalSessions != 42 {
		t.Errorf("TotalSessions = %d, want 42", result.TotalSessions)
	}
	if result.TotalCost != 12.34 {
		t.Errorf("TotalCost = %f, want 12.34", result.TotalCost)
	}
	if result.CacheReadTokens != 200 {
		t.Errorf("CacheReadTokens = %d, want 200", result.CacheReadTokens)
	}
}

func TestToSessionStats_Nil(t *testing.T) {
	result := toSessionStats(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestToSessionStats_Empty(t *testing.T) {
	result := toSessionStats([]SessionStat{})
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(result))
	}
}

func TestToSessionStats_Populated(t *testing.T) {
	now := time.Now()
	input := []SessionStat{
		{ID: "s1", Title: "Session 1", Model: "opus", Cost: 1.5, TokensInput: 100, TimeCreated: now},
		{ID: "s2", Title: "Session 2", Model: "sonnet", Cost: 0.5, TokensOutput: 50, TimeUpdated: now},
	}
	result := toSessionStats(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(result))
	}
	if result[0].ID != "s1" || result[0].Model != "opus" || result[0].Cost != 1.5 {
		t.Errorf("session 0 mismatch: %+v", result[0])
	}
	if result[1].ID != "s2" || result[1].TokensOutput != 50 {
		t.Errorf("session 1 mismatch: %+v", result[1])
	}
}

func TestToDayCosts_Nil(t *testing.T) {
	result := toDayCosts(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestToDayCosts_Populated(t *testing.T) {
	input := []DayCost{
		{Day: "2026-09-20", Cost: 1.23},
		{Day: "2026-09-21", Cost: 4.56},
	}
	result := toDayCosts(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 day costs, got %d", len(result))
	}
	if result[0].Day != "2026-09-20" || result[0].Cost != 1.23 {
		t.Errorf("day 0 mismatch: %+v", result[0])
	}
}

// --- ResolveDBPath tests ---

func TestResolveDBPath_OpenCodeDataHome(t *testing.T) {
	t.Setenv("OPENCODE_DATA_HOME", "/tmp/test-oc")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg") // should be ignored
	path := ResolveDBPath()
	expected := "/tmp/test-oc/opencode.db"
	if path != expected {
		t.Errorf("ResolveDBPath() = %q, want %q", path, expected)
	}
}

func TestResolveDBPath_XDGDataHome(t *testing.T) {
	t.Setenv("OPENCODE_DATA_HOME", "")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	path := ResolveDBPath()
	expected := "/tmp/xdg/opencode/opencode.db"
	if path != expected {
		t.Errorf("ResolveDBPath() = %q, want %q", path, expected)
	}
}

func TestResolveDBPath_Default(t *testing.T) {
	t.Setenv("OPENCODE_DATA_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	path := ResolveDBPath()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	if path != expected {
		t.Errorf("ResolveDBPath() = %q, want %q", path, expected)
	}
}

// --- SQL query tests with in-memory SQLite ---

// createTestDB creates an in-memory SQLite DB with the opencode schema
// and populates it with test data.
func createTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("opening in-memory db: %v", err)
	}

	// Create tables matching opencode's schema
	_, err = db.Exec(`
		CREATE TABLE project (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			worktree TEXT NOT NULL DEFAULT '',
			time_created INTEGER NOT NULL DEFAULT 0,
			time_updated INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE session (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			model TEXT,
			cost REAL NOT NULL DEFAULT 0,
			tokens_input INTEGER NOT NULL DEFAULT 0,
			tokens_output INTEGER NOT NULL DEFAULT 0,
			tokens_reasoning INTEGER NOT NULL DEFAULT 0,
			tokens_cache_read INTEGER NOT NULL DEFAULT 0,
			time_created INTEGER NOT NULL DEFAULT 0,
			time_updated INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (project_id) REFERENCES project(id)
		);
	`)
	if err != nil {
		t.Fatalf("creating schema: %v", err)
	}

	// Insert test projects
	_, err = db.Exec(`
		INSERT INTO project (id, name, worktree) VALUES ('p1', 'Project Alpha', '/path/to/alpha');
		INSERT INTO project (id, name, worktree) VALUES ('p2', 'Project Beta', '/path/to/beta');
	`)
	if err != nil {
		t.Fatalf("inserting projects: %v", err)
	}

	// Insert test sessions
	now := time.Now()
	todayMs := now.UnixMilli()
	yesterdayMs := now.Add(-24 * time.Hour).UnixMilli()
	lastWeekMs := now.Add(-5 * 24 * time.Hour).UnixMilli()
	lastMonthMs := now.Add(-20 * 24 * time.Hour).UnixMilli()

	_, err = db.Exec(`
		INSERT INTO session (id, project_id, title, model, cost, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, time_created, time_updated)
		VALUES
			('s1', 'p1', 'Fix auth bug', 'claude-opus', 1.50, 1000, 500, 100, 200, ?, ?),
			('s2', 'p1', 'Add tests', 'claude-sonnet', 0.30, 800, 400, 0, 100, ?, ?),
			('s3', 'p2', 'Refactor DB', 'claude-opus', 2.00, 2000, 1000, 200, 300, ?, ?),
			('s4', 'p1', 'Old session', 'gpt-4', 0.50, 500, 250, 0, 0, ?, ?);
	`, todayMs, todayMs, yesterdayMs, yesterdayMs, lastWeekMs, lastWeekMs, lastMonthMs, lastMonthMs)
	if err != nil {
		t.Fatalf("inserting sessions: %v", err)
	}

	return db
}

func TestTotalStats(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	stats, err := TotalStats(db)
	if err != nil {
		t.Fatalf("TotalStats error: %v", err)
	}
	if stats.TotalSessions != 4 {
		t.Errorf("TotalSessions = %d, want 4", stats.TotalSessions)
	}
	// 1.50 + 0.30 + 2.00 + 0.50 = 4.30
	if stats.TotalCost < 4.29 || stats.TotalCost > 4.31 {
		t.Errorf("TotalCost = %f, want ~4.30", stats.TotalCost)
	}
	// 1000 + 800 + 2000 + 500 = 4300
	if stats.TotalTokensIn != 4300 {
		t.Errorf("TotalTokensIn = %d, want 4300", stats.TotalTokensIn)
	}
	// 500 + 400 + 1000 + 250 = 2150
	if stats.TotalTokensOut != 2150 {
		t.Errorf("TotalTokensOut = %d, want 2150", stats.TotalTokensOut)
	}
	if stats.ActiveProjects != 2 {
		t.Errorf("ActiveProjects = %d, want 2", stats.ActiveProjects)
	}
	// Today: s1 only
	if stats.TodaySessions < 1 {
		t.Errorf("TodaySessions = %d, want >= 1", stats.TodaySessions)
	}
}

func TestTotalStats_NilDB(t *testing.T) {
	stats, err := TotalStats(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalSessions != 0 {
		t.Errorf("expected 0 sessions for nil DB, got %d", stats.TotalSessions)
	}
}

func TestPeriodStats_7d(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	stats, err := PeriodStats(db, "7d")
	if err != nil {
		t.Fatalf("PeriodStats error: %v", err)
	}
	// s1 (today), s2 (yesterday), s3 (5 days ago) are within 7d. s4 (20 days) is not.
	if stats.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want 3", stats.TotalSessions)
	}
}

func TestPeriodStats_30d(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	stats, err := PeriodStats(db, "30d")
	if err != nil {
		t.Fatalf("PeriodStats error: %v", err)
	}
	// All 4 sessions are within 30d
	if stats.TotalSessions != 4 {
		t.Errorf("TotalSessions = %d, want 4", stats.TotalSessions)
	}
}

func TestPeriodStats_All(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	stats, err := PeriodStats(db, "all")
	if err != nil {
		t.Fatalf("PeriodStats error: %v", err)
	}
	if stats.TotalSessions != 4 {
		t.Errorf("TotalSessions = %d, want 4", stats.TotalSessions)
	}
}

func TestRecentSessions(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sessions, err := RecentSessions(db, 2)
	if err != nil {
		t.Fatalf("RecentSessions error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	// Most recent first (s1 is today)
	if sessions[0].ID != "s1" {
		t.Errorf("expected s1 first (most recent), got %s", sessions[0].ID)
	}
	if sessions[0].Title != "Fix auth bug" {
		t.Errorf("Title = %q, want 'Fix auth bug'", sessions[0].Title)
	}
	if sessions[0].Model != "claude-opus" {
		t.Errorf("Model = %q, want 'claude-opus'", sessions[0].Model)
	}
}

func TestRecentSessions_NilDB(t *testing.T) {
	sessions, err := RecentSessions(nil, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil for nil DB, got %v", sessions)
	}
}

func TestProjectSessions(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sessions, err := ProjectSessions(db, "/path/to/alpha", 10)
	if err != nil {
		t.Fatalf("ProjectSessions error: %v", err)
	}
	// p1 has 3 sessions (s1, s2, s4)
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions for project alpha, got %d", len(sessions))
	}
	// Most recent first
	if sessions[0].ID != "s1" {
		t.Errorf("expected s1 first, got %s", sessions[0].ID)
	}
}

func TestProjectSessions_UnknownPath(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	sessions, err := ProjectSessions(db, "/nonexistent", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions for unknown path, got %d", len(sessions))
	}
}

func TestDailyCosts_All(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	costs, err := DailyCosts(db, "all")
	if err != nil {
		t.Fatalf("DailyCosts error: %v", err)
	}
	// We have sessions on 4 different timestamps, collapsed to days
	if len(costs) == 0 {
		t.Fatal("expected at least 1 day cost entry")
	}
	// Total cost across all days should be ~4.30
	var totalCost float64
	for _, c := range costs {
		totalCost += c.Cost
	}
	if totalCost < 4.29 || totalCost > 4.31 {
		t.Errorf("total daily costs = %f, want ~4.30", totalCost)
	}
}

func TestDailyCosts_NilDB(t *testing.T) {
	costs, err := DailyCosts(nil, "7d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if costs != nil {
		t.Errorf("expected nil for nil DB, got %v", costs)
	}
}

func TestProjectPeriodStats(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	stats, err := ProjectPeriodStats(db, "/path/to/alpha", "7d")
	if err != nil {
		t.Fatalf("ProjectPeriodStats error: %v", err)
	}
	// p1 within 7d: s1 (today) + s2 (yesterday) = 2 sessions
	if stats.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d, want 2", stats.TotalSessions)
	}
}

func TestProjectStats(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()

	stats, err := ProjectStats(db, "/path/to/beta")
	if err != nil {
		t.Fatalf("ProjectStats error: %v", err)
	}
	// p2 has 1 session (s3)
	if stats.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d, want 1", stats.TotalSessions)
	}
	if stats.TotalCost < 1.99 || stats.TotalCost > 2.01 {
		t.Errorf("TotalCost = %f, want ~2.00", stats.TotalCost)
	}
}

// --- StatsProvider integration test ---

func TestStatsProvider_Available_NoFile(t *testing.T) {
	t.Setenv("OPENCODE_DATA_HOME", t.TempDir()) // empty dir, no opencode.db
	p := NewStatsProvider()
	if p.Available() {
		t.Error("expected Available() = false when DB file doesn't exist")
	}
}

func TestStatsProvider_Available_WithFile(t *testing.T) {
	tmpDir := t.TempDir()
	// Create an empty file at the expected path
	dbPath := filepath.Join(tmpDir, "opencode.db")
	if err := os.WriteFile(dbPath, []byte{}, 0644); err != nil {
		t.Fatalf("creating test db file: %v", err)
	}
	t.Setenv("OPENCODE_DATA_HOME", tmpDir)
	p := NewStatsProvider()
	if !p.Available() {
		t.Error("expected Available() = true when DB file exists")
	}
}

func TestStatsProvider_AggregateStats_NoDB(t *testing.T) {
	t.Setenv("OPENCODE_DATA_HOME", t.TempDir())
	p := NewStatsProvider()
	stats, err := p.AggregateStats("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.TotalSessions != 0 {
		t.Errorf("expected 0 sessions, got %d", stats.TotalSessions)
	}
}

// --- Helper to verify platform type conversion ---

func TestStatsProvider_RecentSessions_ReturnsCorrectType(t *testing.T) {
	// This test verifies the type conversion works end-to-end.
	// We can't easily test the full StatsProvider without a real DB file
	// (because openDB uses ResolveDBPath internally), but we can test
	// the type contract by calling the conversion functions directly.
	input := []SessionStat{
		{ID: "x", Title: "Test", Model: "m", Cost: 1.0, TokensInput: 100},
	}
	result := toSessionStats(input)
	// Verify it's the platform type
	var _ []platform.SessionStat = result
	if result[0].ID != "x" || result[0].TokensInput != 100 {
		t.Errorf("conversion mismatch: %+v", result[0])
	}
}
