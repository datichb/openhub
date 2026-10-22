package config

import "time"

// GCConfig configures the clean-up of oh (`oh gc`, and the daemon every
// EveryHours). A zero value takes the default.
type GCConfig struct {
	// Enabled runs the periodic clean-up of the daemon (default true; oh gc
	// works either way).
	Enabled bool `toml:"enabled"`
	// EveryHours is the interval of the periodic clean-up (default 24).
	EveryHours int `toml:"every_hours,omitempty"`
	// SessionDays: the files of a stopped session (bundle, server data,
	// session folder, automatic worktree) are kept that long (default 30).
	SessionDays int `toml:"session_days,omitempty"`
	// HistoryDays: the history (stopped sessions, agent events, settled
	// decisions, session results) is kept that long (default 180).
	HistoryDays int `toml:"history_days,omitempty"`
	// UsageDays: the usage ledger (costs, budgets) is kept that long
	// (default 400).
	UsageDays int `toml:"usage_days,omitempty"`
	// BundleGraceHours: a session bundle used less than that ago is kept
	// (default 24).
	BundleGraceHours int `toml:"bundle_grace_hours,omitempty"`
	// LogMaxMB and LogKeep bound the logs (default 20 MB, 3 copies).
	LogMaxMB int `toml:"log_max_mb,omitempty"`
	LogKeep  int `toml:"log_keep,omitempty"`
}

// Defaults of [gc].
const (
	DefaultGCEveryHours       = 24
	DefaultGCSessionDays      = 30
	DefaultGCHistoryDays      = 180
	DefaultGCUsageDays        = 400
	DefaultGCBundleGraceHours = 24
	DefaultGCLogMaxMB         = 20
	DefaultGCLogKeep          = 3
)

func orDefault(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// Every is the interval of the periodic clean-up.
func (g GCConfig) Every() time.Duration {
	return time.Duration(orDefault(g.EveryHours, DefaultGCEveryHours)) * time.Hour
}

// Sessions is how long the files of a stopped session are kept.
func (g GCConfig) Sessions() time.Duration {
	return days(orDefault(g.SessionDays, DefaultGCSessionDays))
}

// History is how long the history is kept.
func (g GCConfig) History() time.Duration {
	return days(orDefault(g.HistoryDays, DefaultGCHistoryDays))
}

// Usage is how long the usage ledger is kept.
func (g GCConfig) Usage() time.Duration { return days(orDefault(g.UsageDays, DefaultGCUsageDays)) }

// BundleGrace is how long an unused bundle is kept after its last use.
func (g GCConfig) BundleGrace() time.Duration {
	return time.Duration(orDefault(g.BundleGraceHours, DefaultGCBundleGraceHours)) * time.Hour
}

// LogMaxBytes is the size above which a log is rotated.
func (g GCConfig) LogMaxBytes() int64 { return int64(orDefault(g.LogMaxMB, DefaultGCLogMaxMB)) << 20 }

// Logs is the number of rotated copies kept.
func (g GCConfig) Logs() int { return orDefault(g.LogKeep, DefaultGCLogKeep) }

func days(n int) time.Duration { return time.Duration(n) * 24 * time.Hour }
