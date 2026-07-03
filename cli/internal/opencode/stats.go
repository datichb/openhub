package opencode

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// Pure Go SQLite driver (same as the rest of the CLI)
	_ "modernc.org/sqlite"
)

// DefaultDBPath returns the path to opencode's SQLite database.
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}

// OpenStatsDB opens the opencode database in read-only mode.
// Returns nil, nil if the database file doesn't exist.
func OpenStatsDB() (*sql.DB, error) {
	dbPath := DefaultDBPath()
	if dbPath == "" {
		return nil, fmt.Errorf("cannot determine home directory")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil // DB doesn't exist yet
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("opening opencode db: %w", err)
	}
	return db, nil
}

// SessionStat holds statistics for a single opencode session.
type SessionStat struct {
	ID              string
	ProjectID       string
	Title           string
	Model           string
	Cost            float64
	TokensInput     int64
	TokensOutput    int64
	TokensReasoning int64
	TokensCacheRead int64
	TimeCreated     time.Time
	TimeUpdated     time.Time
}

// AggregateStats holds aggregate statistics across sessions.
type AggregateStats struct {
	TotalSessions  int
	TotalTokensIn  int64
	TotalTokensOut int64
	TotalCost      float64
	TodaySessions  int
	ActiveProjects int
	CacheReadTokens int64
	ReasoningTokens int64
}

// ProjectSessions returns recent sessions for a project (matched by worktree path).
func ProjectSessions(db *sql.DB, projectPath string, limit int) ([]SessionStat, error) {
	if db == nil {
		return nil, nil
	}

	query := `
		SELECT s.id, s.project_id, s.title, s.model, s.cost,
		       s.tokens_input, s.tokens_output, s.tokens_reasoning, s.tokens_cache_read,
		       s.time_created, s.time_updated
		FROM session s
		JOIN project p ON s.project_id = p.id
		WHERE p.worktree = ?
		ORDER BY s.time_created DESC
		LIMIT ?`

	rows, err := db.Query(query, projectPath, limit)
	if err != nil {
		return nil, fmt.Errorf("querying sessions: %w", err)
	}
	defer rows.Close()

	return scanSessions(rows)
}

// RecentSessions returns the N most recent sessions across all projects.
func RecentSessions(db *sql.DB, limit int) ([]SessionStat, error) {
	if db == nil {
		return nil, nil
	}

	query := `
		SELECT id, project_id, title, model, cost,
		       tokens_input, tokens_output, tokens_reasoning, tokens_cache_read,
		       time_created, time_updated
		FROM session
		ORDER BY time_created DESC
		LIMIT ?`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying recent sessions: %w", err)
	}
	defer rows.Close()

	return scanSessions(rows)
}

// TotalStats returns aggregate statistics across all sessions.
func TotalStats(db *sql.DB) (*AggregateStats, error) {
	if db == nil {
		return &AggregateStats{}, nil
	}

	stats := &AggregateStats{}

	// Total counts
	err := db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(tokens_input), 0), COALESCE(SUM(tokens_output), 0),
		       COALESCE(SUM(cost), 0), COALESCE(SUM(tokens_cache_read), 0), COALESCE(SUM(tokens_reasoning), 0)
		FROM session
	`).Scan(&stats.TotalSessions, &stats.TotalTokensIn, &stats.TotalTokensOut,
		&stats.TotalCost, &stats.CacheReadTokens, &stats.ReasoningTokens)
	if err != nil {
		return nil, fmt.Errorf("querying total stats: %w", err)
	}

	// Today's sessions (time_created is epoch milliseconds)
	todayStart := startOfDay(time.Now()).UnixMilli()
	err = db.QueryRow(`SELECT COUNT(*) FROM session WHERE time_created >= ?`, todayStart).Scan(&stats.TodaySessions)
	if err != nil {
		stats.TodaySessions = 0 // non-fatal
	}

	// Active projects
	err = db.QueryRow(`SELECT COUNT(DISTINCT project_id) FROM session`).Scan(&stats.ActiveProjects)
	if err != nil {
		stats.ActiveProjects = 0 // non-fatal
	}

	return stats, nil
}

// PeriodStats returns aggregate statistics for sessions within a time period.
// period: "7d", "30d", "all". Anything else defaults to "all".
func PeriodStats(db *sql.DB, period string) (*AggregateStats, error) {
	if db == nil {
		return &AggregateStats{}, nil
	}

	if period == "all" || period == "" {
		return TotalStats(db)
	}

	var days int
	switch period {
	case "7d":
		days = 7
	case "30d":
		days = 30
	default:
		return TotalStats(db)
	}

	since := time.Now().AddDate(0, 0, -days).UnixMilli()
	stats := &AggregateStats{}

	err := db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(tokens_input), 0), COALESCE(SUM(tokens_output), 0),
		       COALESCE(SUM(cost), 0), COALESCE(SUM(tokens_cache_read), 0), COALESCE(SUM(tokens_reasoning), 0)
		FROM session
		WHERE time_created >= ?
	`, since).Scan(&stats.TotalSessions, &stats.TotalTokensIn, &stats.TotalTokensOut,
		&stats.TotalCost, &stats.CacheReadTokens, &stats.ReasoningTokens)
	if err != nil {
		return nil, fmt.Errorf("querying period stats: %w", err)
	}

	// Today's sessions
	todayStart := startOfDay(time.Now()).UnixMilli()
	err = db.QueryRow(`SELECT COUNT(*) FROM session WHERE time_created >= ?`, todayStart).Scan(&stats.TodaySessions)
	if err != nil {
		stats.TodaySessions = 0
	}

	// Active projects in this period
	err = db.QueryRow(`SELECT COUNT(DISTINCT project_id) FROM session WHERE time_created >= ?`, since).Scan(&stats.ActiveProjects)
	if err != nil {
		stats.ActiveProjects = 0
	}

	return stats, nil
}

// ProjectPeriodStats returns aggregate stats for a specific project within a time period.
func ProjectPeriodStats(db *sql.DB, projectPath, period string) (*AggregateStats, error) {
	if db == nil {
		return &AggregateStats{}, nil
	}

	if period == "all" || period == "" {
		return ProjectStats(db, projectPath)
	}

	var days int
	switch period {
	case "7d":
		days = 7
	case "30d":
		days = 30
	default:
		return ProjectStats(db, projectPath)
	}

	since := time.Now().AddDate(0, 0, -days).UnixMilli()
	stats := &AggregateStats{}

	err := db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(s.tokens_input), 0), COALESCE(SUM(s.tokens_output), 0),
		       COALESCE(SUM(s.cost), 0), COALESCE(SUM(s.tokens_cache_read), 0), COALESCE(SUM(s.tokens_reasoning), 0)
		FROM session s
		JOIN project p ON s.project_id = p.id
		WHERE p.worktree = ? AND s.time_created >= ?
	`, projectPath, since).Scan(&stats.TotalSessions, &stats.TotalTokensIn, &stats.TotalTokensOut,
		&stats.TotalCost, &stats.CacheReadTokens, &stats.ReasoningTokens)
	if err != nil {
		return nil, fmt.Errorf("querying project period stats: %w", err)
	}

	return stats, nil
}

// ProjectStats returns aggregate stats for a specific project (by worktree path).
func ProjectStats(db *sql.DB, projectPath string) (*AggregateStats, error) {
	if db == nil {
		return &AggregateStats{}, nil
	}

	stats := &AggregateStats{}
	err := db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(s.tokens_input), 0), COALESCE(SUM(s.tokens_output), 0),
		       COALESCE(SUM(s.cost), 0), COALESCE(SUM(s.tokens_cache_read), 0), COALESCE(SUM(s.tokens_reasoning), 0)
		FROM session s
		JOIN project p ON s.project_id = p.id
		WHERE p.worktree = ?
	`, projectPath).Scan(&stats.TotalSessions, &stats.TotalTokensIn, &stats.TotalTokensOut,
		&stats.TotalCost, &stats.CacheReadTokens, &stats.ReasoningTokens)
	if err != nil {
		return nil, fmt.Errorf("querying project stats: %w", err)
	}

	return stats, nil
}

func scanSessions(rows *sql.Rows) ([]SessionStat, error) {
	var sessions []SessionStat
	for rows.Next() {
		var s SessionStat
		var timeCreated, timeUpdated int64
		var model sql.NullString
		err := rows.Scan(
			&s.ID, &s.ProjectID, &s.Title, &model, &s.Cost,
			&s.TokensInput, &s.TokensOutput, &s.TokensReasoning, &s.TokensCacheRead,
			&timeCreated, &timeUpdated,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		s.Model = model.String
		s.TimeCreated = time.UnixMilli(timeCreated)
		s.TimeUpdated = time.UnixMilli(timeUpdated)
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
