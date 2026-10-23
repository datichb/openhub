package domain

import (
	"context"
	"time"
)

// HistoryRows counts rows per table of the history.
type HistoryRows struct {
	Sessions, AgentEvents, Decisions, UsageSessions, UsageProxy, BudgetExtra int
	WikiProposals                                                            int
}

// HistoryStore counts and deletes the history past its retention (oh gc,
// N16): stopped sessions, agent events, settled decisions and closed wiki
// proposals older than the history cutoff; usage ledger and budget raises
// older than the usage cutoff.
type HistoryStore interface {
	// Count counts the rows that Delete would remove.
	Count(ctx context.Context, history, usage time.Time) (HistoryRows, error)
	// Delete removes the rows past their retention.
	Delete(ctx context.Context, history, usage time.Time) (HistoryRows, error)
}
