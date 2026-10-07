package platform

import "time"

// SessionStat holds statistics for a single session from any backend.
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
	TotalSessions   int
	TotalTokensIn   int64
	TotalTokensOut  int64
	TotalCost       float64
	TodaySessions   int
	ActiveProjects  int
	CacheReadTokens int64
	ReasoningTokens int64
}

// DayCost represents aggregated cost for a single calendar day.
type DayCost struct {
	Day  string // "2026-08-20" (local time, ISO 8601 date)
	Cost float64
}

// StatsProvider abstracts access to session statistics.
//
// The implementation reads the oh session registry. Future
// implementations may aggregate stats from multiple backends, use internal
// records for direct LLM API sessions, or call remote analytics APIs.
//
// Implementations:
//   - internal/sessionstats: the oh session registry (oh.db)
type StatsProvider interface {
	// Available reports whether stats data is accessible.
	Available() bool

	// AggregateStats returns aggregate statistics for the given period.
	// period: "7d", "30d", "all". Anything else defaults to "all".
	AggregateStats(period string) (*AggregateStats, error)

	// ProjectStats returns aggregate statistics for a specific project
	// (identified by filesystem path) within a time period.
	ProjectStats(projectPath, period string) (*AggregateStats, error)

	// RecentSessions returns the N most recent sessions across all projects.
	RecentSessions(limit int) ([]SessionStat, error)

	// ProjectSessions returns recent sessions for a specific project
	// (identified by filesystem path).
	ProjectSessions(projectPath string, limit int) ([]SessionStat, error)

	// DailyCosts returns per-day cost aggregates for a period.
	// period: "7d", "30d", "all".
	DailyCosts(period string) ([]DayCost, error)
}
