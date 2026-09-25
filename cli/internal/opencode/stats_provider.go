package opencode

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/platform"
)

// StatsProvider implements platform.StatsProvider by reading opencode's SQLite database.
type StatsProvider struct{}

// Compile-time check.
var _ platform.StatsProvider = (*StatsProvider)(nil)

// NewStatsProvider returns a StatsProvider backed by opencode's SQLite DB.
func NewStatsProvider() *StatsProvider {
	return &StatsProvider{}
}

// Available reports whether the opencode stats database file exists.
func (p *StatsProvider) Available() bool {
	path := ResolveDBPath()
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// AggregateStats returns aggregate statistics for the given period.
func (p *StatsProvider) AggregateStats(period string) (*platform.AggregateStats, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	if db == nil {
		return &platform.AggregateStats{}, nil
	}
	defer db.Close()

	stats, err := PeriodStats(db, period)
	if err != nil {
		return nil, err
	}
	return toAggregateStats(stats), nil
}

// ProjectStats returns aggregate statistics for a specific project within a period.
func (p *StatsProvider) ProjectStats(projectPath, period string) (*platform.AggregateStats, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	if db == nil {
		return &platform.AggregateStats{}, nil
	}
	defer db.Close()

	stats, err := ProjectPeriodStats(db, projectPath, period)
	if err != nil {
		return nil, err
	}
	return toAggregateStats(stats), nil
}

// RecentSessions returns the N most recent sessions across all projects.
func (p *StatsProvider) RecentSessions(limit int) ([]platform.SessionStat, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}
	defer db.Close()

	sessions, err := RecentSessions(db, limit)
	if err != nil {
		return nil, err
	}
	return toSessionStats(sessions), nil
}

// ProjectSessions returns recent sessions for a specific project.
func (p *StatsProvider) ProjectSessions(projectPath string, limit int) ([]platform.SessionStat, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}
	defer db.Close()

	sessions, err := ProjectSessions(db, projectPath, limit)
	if err != nil {
		return nil, err
	}
	return toSessionStats(sessions), nil
}

// DailyCosts returns per-day cost aggregates for a period.
func (p *StatsProvider) DailyCosts(period string) ([]platform.DayCost, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}
	defer db.Close()

	costs, err := DailyCosts(db, period)
	if err != nil {
		return nil, err
	}
	return toDayCosts(costs), nil
}

// --- Internal helpers ---

// ResolveDBPath returns the path to opencode's SQLite database.
// Resolution order:
//  1. $OPENCODE_DATA_HOME/opencode.db
//  2. $XDG_DATA_HOME/opencode/opencode.db
//  3. ~/.local/share/opencode/opencode.db (fallback)
func ResolveDBPath() string {
	// 1. Explicit override
	if p := os.Getenv("OPENCODE_DATA_HOME"); p != "" {
		return filepath.Join(p, "opencode.db")
	}

	// 2. XDG standard
	if p := os.Getenv("XDG_DATA_HOME"); p != "" {
		return filepath.Join(p, "opencode", "opencode.db")
	}

	// 3. Default
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}

// openDB opens the opencode database in read-only mode.
// Returns nil, nil if the database file doesn't exist.
func openDB() (*sql.DB, error) {
	dbPath := ResolveDBPath()
	if dbPath == "" {
		return nil, fmt.Errorf("cannot determine opencode database path")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("opening opencode db: %w", err)
	}
	return db, nil
}

// toAggregateStats converts the internal AggregateStats to the platform type.
func toAggregateStats(s *AggregateStats) *platform.AggregateStats {
	if s == nil {
		return &platform.AggregateStats{}
	}
	return &platform.AggregateStats{
		TotalSessions:   s.TotalSessions,
		TotalTokensIn:   s.TotalTokensIn,
		TotalTokensOut:  s.TotalTokensOut,
		TotalCost:       s.TotalCost,
		TodaySessions:   s.TodaySessions,
		ActiveProjects:  s.ActiveProjects,
		CacheReadTokens: s.CacheReadTokens,
		ReasoningTokens: s.ReasoningTokens,
	}
}

// toSessionStats converts internal SessionStat slices to the platform type.
func toSessionStats(sessions []SessionStat) []platform.SessionStat {
	if sessions == nil {
		return nil
	}
	result := make([]platform.SessionStat, len(sessions))
	for i, s := range sessions {
		result[i] = platform.SessionStat{
			ID:              s.ID,
			ProjectID:       s.ProjectID,
			Title:           s.Title,
			Model:           s.Model,
			Cost:            s.Cost,
			TokensInput:     s.TokensInput,
			TokensOutput:    s.TokensOutput,
			TokensReasoning: s.TokensReasoning,
			TokensCacheRead: s.TokensCacheRead,
			TimeCreated:     s.TimeCreated,
			TimeUpdated:     s.TimeUpdated,
		}
	}
	return result
}

// toDayCosts converts internal DayCost slices to the platform type.
func toDayCosts(costs []DayCost) []platform.DayCost {
	if costs == nil {
		return nil
	}
	result := make([]platform.DayCost, len(costs))
	for i, c := range costs {
		result[i] = platform.DayCost{
			Day:  c.Day,
			Cost: c.Cost,
		}
	}
	return result
}
