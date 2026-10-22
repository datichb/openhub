package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// UsageStore implements domain.UsageStore (migration v40).
type UsageStore struct{ db *sql.DB }

// NewUsageStore creates a UsageStore from a shared Store.
func NewUsageStore(s *Store) *UsageStore { return &UsageStore{db: s.DB()} }

var _ domain.UsageStore = (*UsageStore)(nil)

// AddSession implements domain.UsageStore.
func (us *UsageStore) AddSession(ctx context.Context, u domain.SessionUsage) error {
	if u.RootID == "" {
		u.RootID = u.SessionID
	}
	_, err := us.db.ExecContext(ctx, `INSERT INTO usage_sessions (day, session_id, root_id, project_id, group_key, cost_usd, tokens_in, tokens_out, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(day, session_id) DO UPDATE SET
			cost_usd = cost_usd + excluded.cost_usd, tokens_in = tokens_in + excluded.tokens_in,
			tokens_out = tokens_out + excluded.tokens_out, project_id = excluded.project_id,
			group_key = excluded.group_key, updated_at = excluded.updated_at`,
		u.Day, u.SessionID, u.RootID, u.ProjectID, u.GroupKey, u.CostUSD, u.TokensIn, u.TokensOut, time.Now())
	if err != nil {
		return fmt.Errorf("recording session usage: %w", err)
	}
	return nil
}

// SessionTotal implements domain.UsageStore.
func (us *UsageStore) SessionTotal(ctx context.Context, rootID string) (domain.SessionUsage, error) {
	u, err := us.total(ctx, "root_id", rootID)
	u.RootID = rootID
	return u, err
}

// ToolSessionTotal implements domain.UsageStore.
func (us *UsageStore) ToolSessionTotal(ctx context.Context, sessionID string) (domain.SessionUsage, error) {
	u, err := us.total(ctx, "session_id", sessionID)
	u.SessionID = sessionID
	return u, err
}

func (us *UsageStore) total(ctx context.Context, column, id string) (domain.SessionUsage, error) {
	var u domain.SessionUsage
	err := us.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(cost_usd), 0), COALESCE(SUM(tokens_in), 0), COALESCE(SUM(tokens_out), 0),
		COALESCE(MAX(project_id), ''), COALESCE(MAX(group_key), '') FROM usage_sessions WHERE `+column+` = ?`, id).
		Scan(&u.CostUSD, &u.TokensIn, &u.TokensOut, &u.ProjectID, &u.GroupKey)
	return u, err
}

// DayCost implements domain.UsageStore.
func (us *UsageStore) DayCost(ctx context.Context, day, projectID string) (float64, error) {
	var v float64
	q, args := `SELECT COALESCE(SUM(cost_usd), 0) FROM usage_sessions WHERE day = ?`, []any{day}
	if projectID != "" {
		q, args = q+` AND project_id = ?`, append(args, projectID)
	}
	err := us.db.QueryRowContext(ctx, q, args...).Scan(&v)
	return v, err
}

// UsageReport implements domain.UsageStore. No migration: it joins the
// ledger (project_id, root_id) with sessions (project_id, workflow_id).
func (us *UsageStore) UsageReport(ctx context.Context, by domain.UsageBy, since string) ([]domain.UsageGroup, error) {
	var key string
	switch by {
	case domain.UsageByProject:
		key = `COALESCE(NULLIF(u.project_id, ''), NULLIF(s.project_id, ''), '')`
	case domain.UsageByWorkflow:
		key = `COALESCE(NULLIF(s.workflow_id, ''), '')`
	default:
		return nil, fmt.Errorf("usage report: unknown axis %q", by)
	}
	rows, err := us.db.QueryContext(ctx, `SELECT `+key+` AS k, COUNT(DISTINCT u.root_id), COALESCE(SUM(u.cost_usd), 0),
		COALESCE(SUM(u.tokens_in), 0), COALESCE(SUM(u.tokens_out), 0)
		FROM usage_sessions u LEFT JOIN sessions s ON s.id = u.root_id
		WHERE u.day >= ? GROUP BY k ORDER BY SUM(u.cost_usd) DESC, k`, since)
	if err != nil {
		return nil, fmt.Errorf("usage report: %w", err)
	}
	defer rows.Close()
	var out []domain.UsageGroup
	for rows.Next() {
		var g domain.UsageGroup
		if err := rows.Scan(&g.Key, &g.Sessions, &g.CostUSD, &g.TokensIn, &g.TokensOut); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// AddProxy implements domain.UsageStore.
func (us *UsageStore) AddProxy(ctx context.Context, group, day string, u domain.ProxyUsage) error {
	_, err := us.db.ExecContext(ctx, `INSERT INTO usage_proxy (day, group_key, requests, tokens_in, tokens_out)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(day, group_key) DO UPDATE SET requests = requests + excluded.requests,
			tokens_in = tokens_in + excluded.tokens_in, tokens_out = tokens_out + excluded.tokens_out`,
		day, group, u.Requests, u.TokensIn, u.TokensOut)
	if err != nil {
		return fmt.Errorf("recording proxy usage: %w", err)
	}
	return nil
}

// ProxyTotal implements domain.UsageStore.
func (us *UsageStore) ProxyTotal(ctx context.Context, group string) (domain.ProxyUsage, error) {
	var u domain.ProxyUsage
	err := us.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(requests), 0), COALESCE(SUM(tokens_in), 0), COALESCE(SUM(tokens_out), 0)
		FROM usage_proxy WHERE group_key = ?`, group).Scan(&u.Requests, &u.TokensIn, &u.TokensOut)
	return u, err
}

// AddBudgetExtra implements domain.UsageStore.
func (us *UsageStore) AddBudgetExtra(ctx context.Context, scope, day string, usd float64) error {
	_, err := us.db.ExecContext(ctx, `INSERT INTO budget_extra (scope, day, extra_usd) VALUES (?, ?, ?)
		ON CONFLICT(scope, day) DO UPDATE SET extra_usd = extra_usd + excluded.extra_usd`, scope, day, usd)
	return err
}

// BudgetExtra implements domain.UsageStore.
func (us *UsageStore) BudgetExtra(ctx context.Context, scope, day string) (float64, error) {
	var v float64
	err := us.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(extra_usd), 0) FROM budget_extra WHERE scope = ? AND day = ?`, scope, day).Scan(&v)
	return v, err
}
