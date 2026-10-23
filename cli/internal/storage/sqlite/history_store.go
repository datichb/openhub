package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// HistoryStore counts and deletes the history past its retention (oh gc,
// N16): stopped sessions, agent events, settled decisions and closed wiki
// proposals (applied or refused, LD-J) older than the history cutoff;
// usage ledger and budget raises older than the usage cutoff. Deletions go
// by batches of historyBatch rows.
type HistoryStore struct{ db *sql.DB }

// NewHistoryStore creates a HistoryStore.
func NewHistoryStore(s *Store) *HistoryStore { return &HistoryStore{db: s.DB()} }

const historyBatch = 500

// HistoryRows counts rows per table.
type HistoryRows = domain.HistoryRows

var _ domain.HistoryStore = (*HistoryStore)(nil)

// oldIDs returns the ids of a table whose time column is before cutoff
// (dates are compared in Go: the column holds the driver's text format).
func (h *HistoryStore) oldIDs(ctx context.Context, query string, cutoff time.Time) ([]string, error) {
	rows, err := h.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		times := make([]sql.NullTime, len(cols)-1)
		dest := []any{&id}
		for i := range times {
			dest = append(dest, &times[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		// The first known date of the row (ended, changed, started).
		for _, at := range times {
			if at.Valid {
				if at.Time.Before(cutoff) {
					ids = append(ids, id)
				}
				break
			}
		}
	}
	return ids, rows.Err()
}

const (
	oldSessionsQuery  = `SELECT id, ended_at, state_changed_at, started_at FROM sessions WHERE state = 'stopped'`
	oldEventsQuery    = `SELECT id, started_at FROM agent_events`
	oldDecisionsQuery = `SELECT id, resolved_at FROM pending_decisions WHERE resolved_at IS NOT NULL`
	// The last proposal of a project stays: its number is the next one's
	// base (a purged number is never given again).
	oldWikiQuery = `SELECT project_id || char(31) || seq, applied_at, decided_at, created_at FROM wiki_proposals
		WHERE status IN ('applied', 'refused', 'stale') AND seq < (SELECT MAX(w.seq) FROM wiki_proposals w WHERE w.project_id = wiki_proposals.project_id)`
)

// deleteWiki removes the closed wiki proposals of ids (project US seq).
func (h *HistoryStore) deleteWiki(ctx context.Context, ids []string) (int, error) {
	n := 0
	for _, id := range ids {
		project, seq, _ := strings.Cut(id, "\x1f")
		res, err := h.db.ExecContext(ctx, `DELETE FROM wiki_proposals WHERE project_id = ? AND seq = ? AND status IN ('applied', 'refused', 'stale')`, project, seq)
		if err != nil {
			return n, fmt.Errorf("deleting old wiki proposals: %w", err)
		}
		k, _ := res.RowsAffected()
		n += int(k)
	}
	return n, nil
}

func (h *HistoryStore) countDays(ctx context.Context, table, day string) (int, error) {
	var n int
	err := h.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE day != '' AND day < ?`, day).Scan(&n)
	return n, err
}

// Count counts the rows that Delete would remove.
func (h *HistoryStore) Count(ctx context.Context, history, usage time.Time) (HistoryRows, error) {
	var r HistoryRows
	for _, q := range []struct {
		query string
		n     *int
	}{{oldSessionsQuery, &r.Sessions}, {oldEventsQuery, &r.AgentEvents}, {oldDecisionsQuery, &r.Decisions}} {
		ids, err := h.oldIDs(ctx, q.query, history)
		if err != nil {
			return r, err
		}
		*q.n = len(ids)
	}
	wiki, err := h.oldIDs(ctx, oldWikiQuery, history)
	if err != nil {
		return r, err
	}
	r.WikiProposals = len(wiki)
	day := usage.UTC().Format("2006-01-02")
	if r.UsageSessions, err = h.countDays(ctx, "usage_sessions", day); err != nil {
		return r, err
	}
	if r.UsageProxy, err = h.countDays(ctx, "usage_proxy", day); err != nil {
		return r, err
	}
	r.BudgetExtra, err = h.countDays(ctx, "budget_extra", day)
	return r, err
}

// Delete removes the rows past their retention, then optimizes the
// database (no VACUUM: the daemon runs it, VACUUM would block the others).
func (h *HistoryStore) Delete(ctx context.Context, history, usage time.Time) (HistoryRows, error) {
	var r HistoryRows
	for _, q := range []struct {
		query, table string
		n            *int
	}{{oldSessionsQuery, "sessions", &r.Sessions}, {oldEventsQuery, "agent_events", &r.AgentEvents}, {oldDecisionsQuery, "pending_decisions", &r.Decisions}} {
		ids, err := h.oldIDs(ctx, q.query, history)
		if err != nil {
			return r, err
		}
		for start := 0; start < len(ids); start += historyBatch {
			batch := ids[start:min(start+historyBatch, len(ids))]
			args := make([]any, len(batch))
			for i, id := range batch {
				args[i] = id
			}
			res, err := h.db.ExecContext(ctx, `DELETE FROM `+q.table+` WHERE id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")+`)`, args...)
			if err != nil {
				return r, fmt.Errorf("deleting old %s: %w", q.table, err)
			}
			n, _ := res.RowsAffected()
			*q.n += int(n)
		}
	}
	wiki, err := h.oldIDs(ctx, oldWikiQuery, history)
	if err != nil {
		return r, err
	}
	if r.WikiProposals, err = h.deleteWiki(ctx, wiki); err != nil {
		return r, err
	}
	day := usage.UTC().Format("2006-01-02")
	for _, t := range []struct {
		table string
		n     *int
	}{{"usage_sessions", &r.UsageSessions}, {"usage_proxy", &r.UsageProxy}, {"budget_extra", &r.BudgetExtra}} {
		for {
			res, err := h.db.ExecContext(ctx, `DELETE FROM `+t.table+` WHERE rowid IN (SELECT rowid FROM `+t.table+` WHERE day != '' AND day < ? LIMIT ?)`, day, historyBatch)
			if err != nil {
				return r, fmt.Errorf("deleting old %s: %w", t.table, err)
			}
			n, _ := res.RowsAffected()
			*t.n += int(n)
			if n < historyBatch {
				break
			}
		}
	}
	_, _ = h.db.ExecContext(ctx, `PRAGMA optimize`)
	return r, nil
}
