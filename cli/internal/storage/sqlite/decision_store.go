package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// DecisionStore implements domain.DecisionStore (table pending_decisions).
type DecisionStore struct{ db *sql.DB }

// NewDecisionStore creates a DecisionStore from a shared Store.
func NewDecisionStore(s *Store) *DecisionStore { return &DecisionStore{db: s.DB()} }

var _ domain.DecisionStore = (*DecisionStore)(nil)

const decisionColumns = `id, session_id, group_key, kind, tool_ref, payload, created_at, resolved_at, resolved_by, resolution`

func scanDecision(sc interface{ Scan(...any) error }) (domain.Decision, error) {
	var d domain.Decision
	var kind, payload, by, resolution string
	var resolvedAt sql.NullTime
	if err := sc.Scan(&d.ID, &d.SessionID, &d.GroupKey, &kind, &d.ToolRef, &payload, &d.CreatedAt, &resolvedAt, &by, &resolution); err != nil {
		return d, err
	}
	d.Kind, d.ResolvedBy = domain.DecisionKind(kind), domain.DecisionResolver(by)
	_ = json.Unmarshal([]byte(payload), &d.Payload)
	if resolvedAt.Valid {
		t := resolvedAt.Time
		d.ResolvedAt = &t
	}
	if resolution != "" {
		var r domain.DecisionResolution
		if json.Unmarshal([]byte(resolution), &r) == nil {
			d.Resolution = &r
		}
	}
	return d, nil
}

// Upsert inserts a decision or refreshes its payload.
func (ds *DecisionStore) Upsert(ctx context.Context, d *domain.Decision) error {
	if d.ID == "" || d.SessionID == "" || d.Kind == "" {
		return errors.New("decision: id, session and kind are required")
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	payload, err := json.Marshal(d.Payload)
	if err != nil {
		return err
	}
	_, err = ds.db.ExecContext(ctx, `INSERT INTO pending_decisions (id, session_id, group_key, kind, tool_ref, payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET payload = excluded.payload,
			group_key = CASE WHEN excluded.group_key <> '' THEN excluded.group_key ELSE pending_decisions.group_key END`,
		d.ID, d.SessionID, d.GroupKey, string(d.Kind), d.ToolRef, string(payload), d.CreatedAt)
	if err != nil {
		return fmt.Errorf("saving decision %s: %w", d.ID, err)
	}
	return nil
}

// Get returns a decision.
func (ds *DecisionStore) Get(ctx context.Context, id string) (*domain.Decision, error) {
	d, err := scanDecision(ds.db.QueryRowContext(ctx, `SELECT `+decisionColumns+` FROM pending_decisions WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting decision %s: %w", id, err)
	}
	return &d, nil
}

// ListOpen returns the unresolved decisions matching f, oldest first.
func (ds *DecisionStore) ListOpen(ctx context.Context, f domain.DecisionFilter) ([]domain.Decision, error) {
	where := []string{"resolved_at IS NULL"}
	var args []any
	if f.SessionID != "" {
		where, args = append(where, "session_id = ?"), append(args, f.SessionID)
	}
	if f.GroupKey != "" {
		where, args = append(where, "group_key = ?"), append(args, f.GroupKey)
	}
	if f.Kind != "" {
		where, args = append(where, "kind = ?"), append(args, string(f.Kind))
	}
	return ds.query(ctx, `SELECT `+decisionColumns+` FROM pending_decisions WHERE `+strings.Join(where, " AND ")+` ORDER BY created_at, id`, args...)
}

// ListSince returns the decisions created or resolved after t. Dates are
// compared in Go: the stored text format does not sort across time zones.
func (ds *DecisionStore) ListSince(ctx context.Context, t time.Time) ([]domain.Decision, error) {
	all, err := ds.query(ctx, `SELECT `+decisionColumns+` FROM pending_decisions ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, d := range all {
		if d.CreatedAt.After(t) || (d.ResolvedAt != nil && d.ResolvedAt.After(t)) {
			out = append(out, d)
		}
	}
	return out, nil
}

func (ds *DecisionStore) query(ctx context.Context, q string, args ...any) ([]domain.Decision, error) {
	rows, err := ds.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing decisions: %w", err)
	}
	defer rows.Close()
	var out []domain.Decision
	for rows.Next() {
		d, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Resolve closes an open decision; false when it was already resolved.
func (ds *DecisionStore) Resolve(ctx context.Context, id string, by domain.DecisionResolver, r *domain.DecisionResolution, at time.Time) (bool, error) {
	resolution := ""
	if r != nil {
		buf, err := json.Marshal(r)
		if err != nil {
			return false, err
		}
		resolution = string(buf)
	}
	res, err := ds.db.ExecContext(ctx, `UPDATE pending_decisions SET resolved_at = ?, resolved_by = ?, resolution = ?
		WHERE id = ? AND resolved_at IS NULL`, at, string(by), resolution, id)
	if err != nil {
		return false, fmt.Errorf("resolving decision %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Reopen cancels a resolution.
func (ds *DecisionStore) Reopen(ctx context.Context, id string) error {
	_, err := ds.db.ExecContext(ctx, `UPDATE pending_decisions SET resolved_at = NULL, resolved_by = '', resolution = '' WHERE id = ?`, id)
	return err
}
