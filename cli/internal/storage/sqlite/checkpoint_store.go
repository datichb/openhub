package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/datichb/openhub/cli/internal/domain"
)

// CheckpointStore implements domain.CheckpointStore on sessions.checkpoint_state (v35).
type CheckpointStore struct {
	db *sql.DB
}

// NewCheckpointStore creates a CheckpointStore from a shared Store.
func NewCheckpointStore(s *Store) *CheckpointStore { return &CheckpointStore{db: s.DB()} }

var _ domain.CheckpointStore = (*CheckpointStore)(nil)

func (c *CheckpointStore) raw(ctx context.Context, id string) (string, error) {
	var raw string
	err := c.db.QueryRowContext(ctx, `SELECT checkpoint_state FROM sessions WHERE id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return raw, err
}

func decodeCheckpointState(raw string) (domain.CheckpointState, error) {
	var st domain.CheckpointState
	if raw == "" {
		return st, nil
	}
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return st, fmt.Errorf("decoding checkpoint state: %w", err)
	}
	return st, nil
}

// GetCheckpointState implements domain.CheckpointStore.
func (c *CheckpointStore) GetCheckpointState(ctx context.Context, id string) (domain.CheckpointState, error) {
	raw, err := c.raw(ctx, id)
	if err != nil {
		return domain.CheckpointState{}, err
	}
	return decodeCheckpointState(raw)
}

// UpdateCheckpointState implements domain.CheckpointStore with a
// compare-and-swap on the stored JSON (retried on a concurrent update).
func (c *CheckpointStore) UpdateCheckpointState(ctx context.Context, id string, fn func(*domain.CheckpointState) error) (domain.CheckpointState, error) {
	for attempt := 0; attempt < 20; attempt++ {
		raw, err := c.raw(ctx, id)
		if err != nil {
			return domain.CheckpointState{}, err
		}
		st, err := decodeCheckpointState(raw)
		if err != nil {
			return st, err
		}
		if err := fn(&st); err != nil {
			return st, err
		}
		data, err := json.Marshal(st)
		if err != nil {
			return st, err
		}
		res, err := c.db.ExecContext(ctx, `UPDATE sessions SET checkpoint_state = ? WHERE id = ? AND checkpoint_state = ?`, string(data), id, raw)
		if err != nil {
			return st, err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return st, nil
		}
	}
	return domain.CheckpointState{}, errors.New("checkpoint state: too many concurrent updates")
}

var _ domain.SessionOutputStore = (*CheckpointStore)(nil)

// SetSessionOutput implements domain.SessionOutputStore (sessions.outputs, v34).
func (c *CheckpointStore) SetSessionOutput(ctx context.Context, id, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	res, err := c.db.ExecContext(ctx,
		`UPDATE sessions SET outputs = json_set(CASE WHEN json_valid(outputs) THEN outputs ELSE '{}' END, '$.' || json_quote(?), json(?)) WHERE id = ?`,
		key, string(data), id)
	if err != nil {
		return fmt.Errorf("setting session output: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}
