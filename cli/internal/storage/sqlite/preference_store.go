package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// PreferenceStore implements domain.PreferenceStore and
// domain.WorkflowUsageReader (recent workflows read from sessions).
type PreferenceStore struct{ db *sql.DB }

// NewPreferenceStore creates a PreferenceStore from a shared Store.
func NewPreferenceStore(s *Store) *PreferenceStore { return &PreferenceStore{db: s.DB()} }

var (
	_ domain.PreferenceStore     = (*PreferenceStore)(nil)
	_ domain.WorkflowUsageReader = (*PreferenceStore)(nil)
)

// Get returns a preference.
func (ps *PreferenceStore) Get(ctx context.Context, scope, key string) (*domain.Preference, error) {
	p := domain.Preference{Scope: scope, Key: key}
	var value string
	err := ps.db.QueryRowContext(ctx, `SELECT value, updated_at FROM preferences WHERE scope = ? AND key = ?`, scope, key).
		Scan(&value, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting preference %s/%s: %w", scope, key, err)
	}
	p.Value = json.RawMessage(value)
	return &p, nil
}

// Set inserts or replaces a preference. The value must be valid JSON.
func (ps *PreferenceStore) Set(ctx context.Context, scope, key string, value json.RawMessage) error {
	if !json.Valid(value) {
		return fmt.Errorf("preference %s/%s: invalid JSON value", scope, key)
	}
	_, err := ps.db.ExecContext(ctx, `INSERT INTO preferences (scope, key, value, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(scope, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		scope, key, string(value), time.Now())
	if err != nil {
		return fmt.Errorf("saving preference %s/%s: %w", scope, key, err)
	}
	return nil
}

// Delete removes a preference (no error when absent).
func (ps *PreferenceStore) Delete(ctx context.Context, scope, key string) error {
	if _, err := ps.db.ExecContext(ctx, `DELETE FROM preferences WHERE scope = ? AND key = ?`, scope, key); err != nil {
		return fmt.Errorf("deleting preference %s/%s: %w", scope, key, err)
	}
	return nil
}

// List returns the preferences of a scope, ordered by key.
func (ps *PreferenceStore) List(ctx context.Context, scope string) ([]domain.Preference, error) {
	rows, err := ps.db.QueryContext(ctx, `SELECT key, value, updated_at FROM preferences WHERE scope = ? ORDER BY key`, scope)
	if err != nil {
		return nil, fmt.Errorf("listing preferences %s: %w", scope, err)
	}
	defer rows.Close()
	var out []domain.Preference
	for rows.Next() {
		p := domain.Preference{Scope: scope}
		var value string
		if err := rows.Scan(&p.Key, &value, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning preference: %w", err)
		}
		p.Value = json.RawMessage(value)
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecentWorkflows returns the distinct workflow IDs of the sessions, most
// recently started first.
func (ps *PreferenceStore) RecentWorkflows(ctx context.Context, projectID string, limit int) ([]domain.WorkflowUse, error) {
	// started_at is read as a typed column (an aggregate would come back as
	// driver-formatted text); duplicates are skipped while scanning.
	query := `SELECT workflow_id, started_at FROM sessions WHERE workflow_id != ''`
	var args []any
	if projectID != "" {
		query += ` AND project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY started_at DESC`
	rows, err := ps.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing recent workflows: %w", err)
	}
	defer rows.Close()
	var out []domain.WorkflowUse
	seen := map[string]bool{}
	for rows.Next() {
		var u domain.WorkflowUse
		if err := rows.Scan(&u.WorkflowID, &u.LastUsed); err != nil {
			return nil, fmt.Errorf("scanning recent workflow: %w", err)
		}
		if seen[u.WorkflowID] {
			continue
		}
		seen[u.WorkflowID] = true
		out = append(out, u)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out, rows.Err()
}
