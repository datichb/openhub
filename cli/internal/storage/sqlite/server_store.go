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

// ServerStore implements domain.ServerStore.
type ServerStore struct{ db *sql.DB }

// NewServerStore creates a ServerStore from a shared Store.
func NewServerStore(s *Store) *ServerStore { return &ServerStore{db: s.DB()} }

var _ domain.ServerStore = (*ServerStore)(nil)

const serverColumns = `group_key, adapter, adapter_version, runtime, project_id, bundle_hash, pid, url, port, password, data_dir, work_dir, proxy_token, status, created_at, last_activity_at`

func scanServer(sc interface{ Scan(...any) error }) (domain.Server, error) {
	var s domain.Server
	var status string
	err := sc.Scan(&s.GroupKey, &s.Adapter, &s.AdapterVersion, &s.Runtime, &s.ProjectID, &s.BundleHash,
		&s.PID, &s.URL, &s.Port, &s.Password, &s.DataDir, &s.WorkDir, &s.ProxyToken, &status, &s.CreatedAt, &s.LastActivityAt)
	s.Status = domain.ServerStatus(status)
	return s, err
}

// Upsert inserts or replaces a server.
func (ss *ServerStore) Upsert(ctx context.Context, s *domain.Server) error {
	now := time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.LastActivityAt.IsZero() {
		s.LastActivityAt = now
	}
	_, err := ss.db.ExecContext(ctx, `INSERT INTO servers (`+serverColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_key) DO UPDATE SET
			adapter=excluded.adapter, adapter_version=excluded.adapter_version, runtime=excluded.runtime,
			project_id=excluded.project_id, bundle_hash=excluded.bundle_hash, pid=excluded.pid, url=excluded.url,
			port=excluded.port, password=excluded.password, data_dir=excluded.data_dir, work_dir=excluded.work_dir,
			proxy_token=excluded.proxy_token, status=excluded.status, last_activity_at=excluded.last_activity_at`,
		s.GroupKey, s.Adapter, s.AdapterVersion, s.Runtime, s.ProjectID, s.BundleHash, s.PID, s.URL, s.Port,
		s.Password, s.DataDir, s.WorkDir, s.ProxyToken, string(s.Status), s.CreatedAt, s.LastActivityAt)
	if err != nil {
		return fmt.Errorf("saving server %s: %w", s.GroupKey, err)
	}
	return nil
}

// Get returns a server by group key.
func (ss *ServerStore) Get(ctx context.Context, groupKey string) (*domain.Server, error) {
	s, err := scanServer(ss.db.QueryRowContext(ctx, `SELECT `+serverColumns+` FROM servers WHERE group_key = ?`, groupKey))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting server %s: %w", groupKey, err)
	}
	return &s, nil
}

// List returns all servers.
func (ss *ServerStore) List(ctx context.Context) ([]domain.Server, error) {
	rows, err := ss.db.QueryContext(ctx, `SELECT `+serverColumns+` FROM servers ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing servers: %w", err)
	}
	defer rows.Close()
	var out []domain.Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetStatus updates a server status.
func (ss *ServerStore) SetStatus(ctx context.Context, groupKey string, status domain.ServerStatus) error {
	_, err := ss.db.ExecContext(ctx, `UPDATE servers SET status = ? WHERE group_key = ?`, string(status), groupKey)
	return err
}

// Touch records server activity.
func (ss *ServerStore) Touch(ctx context.Context, groupKey string, at time.Time) error {
	_, err := ss.db.ExecContext(ctx, `UPDATE servers SET last_activity_at = ? WHERE group_key = ?`, at, groupKey)
	return err
}

// Delete removes a server.
func (ss *ServerStore) Delete(ctx context.Context, groupKey string) error {
	_, err := ss.db.ExecContext(ctx, `DELETE FROM servers WHERE group_key = ?`, groupKey)
	return err
}

// GrantStore implements domain.GrantStore.
type GrantStore struct{ db *sql.DB }

// NewGrantStore creates a GrantStore from a shared Store.
func NewGrantStore(s *Store) *GrantStore { return &GrantStore{db: s.DB()} }

var _ domain.GrantStore = (*GrantStore)(nil)

// Insert stores a grant.
func (gs *GrantStore) Insert(ctx context.Context, g *domain.ProxyGrant) error {
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now()
	}
	src, _ := json.Marshal(g.Source)
	models, _ := json.Marshal(g.AllowedModels)
	_, err := gs.db.ExecContext(ctx, `INSERT INTO proxy_grants (token, owner, provider, region, source, allowed_models, max_tokens, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, g.Token, g.Owner, g.Provider, g.Region, string(src), string(models), g.MaxTokens, g.CreatedAt)
	if err != nil {
		return fmt.Errorf("saving proxy grant: %w", err)
	}
	return nil
}

// ListActive returns grants that are not revoked.
func (gs *GrantStore) ListActive(ctx context.Context) ([]domain.ProxyGrant, error) {
	rows, err := gs.db.QueryContext(ctx, `SELECT token, owner, provider, region, source, allowed_models, max_tokens, created_at
		FROM proxy_grants WHERE revoked_at IS NULL ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing proxy grants: %w", err)
	}
	defer rows.Close()
	var out []domain.ProxyGrant
	for rows.Next() {
		var g domain.ProxyGrant
		var src, models string
		if err := rows.Scan(&g.Token, &g.Owner, &g.Provider, &g.Region, &src, &models, &g.MaxTokens, &g.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(src), &g.Source)
		_ = json.Unmarshal([]byte(models), &g.AllowedModels)
		out = append(out, g)
	}
	return out, rows.Err()
}

// Revoke marks a grant revoked.
func (gs *GrantStore) Revoke(ctx context.Context, token string, at time.Time) error {
	_, err := gs.db.ExecContext(ctx, `UPDATE proxy_grants SET revoked_at = ? WHERE token = ? AND revoked_at IS NULL`, at, token)
	return err
}

// RevokeOwner revokes every grant of an owner.
func (gs *GrantStore) RevokeOwner(ctx context.Context, owner string, at time.Time) error {
	_, err := gs.db.ExecContext(ctx, `UPDATE proxy_grants SET revoked_at = ? WHERE owner = ? AND revoked_at IS NULL`, at, owner)
	return err
}
