package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/datichb/openhub/cli/internal/domain"
)

// RemoteStore implements domain.RemoteStore on sessions.remote_ref (v39).
type RemoteStore struct {
	db *sql.DB
}

// NewRemoteStore creates a RemoteStore from a shared Store.
func NewRemoteStore(s *Store) *RemoteStore { return &RemoteStore{db: s.DB()} }

var _ domain.RemoteStore = (*RemoteStore)(nil)

// GetRemoteRef implements domain.RemoteStore.
func (r *RemoteStore) GetRemoteRef(ctx context.Context, id string) (*domain.RemoteRef, error) {
	var raw string
	err := r.db.QueryRowContext(ctx, `SELECT remote_ref FROM sessions WHERE id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil || raw == "" {
		return nil, err
	}
	var ref domain.RemoteRef
	if err := json.Unmarshal([]byte(raw), &ref); err != nil {
		return nil, fmt.Errorf("decoding remote reference: %w", err)
	}
	return &ref, nil
}

// SetRemoteRef implements domain.RemoteStore.
func (r *RemoteStore) SetRemoteRef(ctx context.Context, id string, ref domain.RemoteRef) error {
	data, err := json.Marshal(ref)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE sessions SET remote_ref = ? WHERE id = ?`, string(data), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListRemote implements domain.RemoteStore.
func (r *RemoteStore) ListRemote(ctx context.Context) (map[string]domain.RemoteRef, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, remote_ref FROM sessions WHERE remote_ref != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]domain.RemoteRef{}
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		var ref domain.RemoteRef
		if err := json.Unmarshal([]byte(raw), &ref); err != nil {
			continue
		}
		out[id] = ref
	}
	return out, rows.Err()
}
