package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/fsutil"
)

// ServerStore implements domain.ServerStore.
//
// The password of a tool server is not kept in oh.db (v5 critical N26): an
// agent able to read the database (any interpreter) would drive the servers
// of every project. It is written to a 0600 file per group under
// <hub>/run/server-passwords/ (refused to the shell of the agents by the
// global guard); the password column keeps only its hash.
type ServerStore struct {
	db  *sql.DB
	dir string // password files ("" = next to nothing: kept in memory only)
}

// NewServerStore creates a ServerStore from a shared Store.
func NewServerStore(s *Store) *ServerStore {
	ss := &ServerStore{db: s.DB()}
	if s.path != "" {
		ss.dir = filepath.Join(filepath.Dir(s.path), "run", "server-passwords")
	}
	return ss
}

// passwordHashPrefix marks a password column holding a hash.
const passwordHashPrefix = "sha256:"

func passwordHash(pw string) string {
	sum := sha256.Sum256([]byte(pw))
	return passwordHashPrefix + hex.EncodeToString(sum[:])
}

func (ss *ServerStore) passwordFile(group string) string {
	sum := sha256.Sum256([]byte(group))
	return filepath.Join(ss.dir, hex.EncodeToString(sum[:16])+".pw")
}

// storePassword writes the password of a group to its file and returns the
// value of the password column.
func (ss *ServerStore) storePassword(group, pw string) (string, error) {
	if ss.dir == "" {
		return "", errors.New("no directory for the server passwords")
	}
	if pw == "" {
		_ = os.Remove(ss.passwordFile(group))
		return "", nil
	}
	if err := os.MkdirAll(ss.dir, 0o700); err != nil {
		return "", err
	}
	_ = os.Chmod(ss.dir, 0o700)
	if err := fsutil.WriteFileAtomic(ss.passwordFile(group), []byte(pw), 0o600); err != nil {
		return "", err
	}
	return passwordHash(pw), nil
}

// loadPassword returns the password of a group from the column value: read
// from its file when the column holds a hash ("" when the file is missing
// or does not match), the column itself for a value left in clear by an
// older oh (moved by MovePasswords).
func (ss *ServerStore) loadPassword(group, column string) string {
	if !strings.HasPrefix(column, passwordHashPrefix) {
		return column
	}
	data, err := os.ReadFile(ss.passwordFile(group))
	if err != nil || passwordHash(string(data)) != column {
		return ""
	}
	return string(data)
}

// MovePasswords moves the server passwords stored in clear in oh.db by an
// older oh to their files (daemon start-up).
func (ss *ServerStore) MovePasswords(ctx context.Context) (int, error) {
	rows, err := ss.db.QueryContext(ctx, `SELECT group_key, password FROM servers WHERE password <> '' AND substr(password, 1, ?) <> ?`,
		len(passwordHashPrefix), passwordHashPrefix)
	if err != nil {
		return 0, err
	}
	type row struct{ group, pw string }
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.group, &r.pw); err != nil {
			rows.Close()
			return 0, err
		}
		list = append(list, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, r := range list {
		col, err := ss.storePassword(r.group, r.pw)
		if err != nil {
			return 0, fmt.Errorf("moving the password of server %s: %w", r.group, err)
		}
		if _, err := ss.db.ExecContext(ctx, `UPDATE servers SET password = ? WHERE group_key = ? AND password = ?`, col, r.group, r.pw); err != nil {
			return 0, err
		}
	}
	if len(list) > 0 {
		// The old values may stay in free pages and the WAL.
		_, _ = ss.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
		_, _ = ss.db.ExecContext(ctx, `VACUUM`)
	}
	return len(list), nil
}

// CountClearPasswords counts the server passwords still in clear (Doctor).
func (ss *ServerStore) CountClearPasswords(ctx context.Context) (int, error) {
	var n int
	err := ss.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM servers WHERE password <> '' AND substr(password, 1, ?) <> ?`,
		len(passwordHashPrefix), passwordHashPrefix).Scan(&n)
	return n, err
}

var _ domain.ServerStore = (*ServerStore)(nil)

const serverColumns = `group_key, adapter, adapter_version, runtime, project_id, bundle_hash, pid, url, port, password, data_dir, work_dir, proxy_token, status, created_at, last_activity_at`

func (ss *ServerStore) scanServer(sc interface{ Scan(...any) error }) (domain.Server, error) {
	s, err := scanServer(sc)
	if err == nil {
		s.Password = ss.loadPassword(s.GroupKey, s.Password)
	}
	return s, err
}

func scanServer(sc interface{ Scan(...any) error }) (domain.Server, error) {
	var s domain.Server
	var status string
	err := sc.Scan(&s.GroupKey, &s.Adapter, &s.AdapterVersion, &s.Runtime, &s.ProjectID, &s.BundleHash,
		&s.PID, &s.URL, &s.Port, &s.Password, &s.DataDir, &s.WorkDir, &s.ProxyTokenHash, &status, &s.CreatedAt, &s.LastActivityAt)
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
	password, err := ss.storePassword(s.GroupKey, s.Password)
	if err != nil {
		return fmt.Errorf("saving server %s: %w", s.GroupKey, err)
	}
	_, err = ss.db.ExecContext(ctx, `INSERT INTO servers (`+serverColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(group_key) DO UPDATE SET
			adapter=excluded.adapter, adapter_version=excluded.adapter_version, runtime=excluded.runtime,
			project_id=excluded.project_id, bundle_hash=excluded.bundle_hash, pid=excluded.pid, url=excluded.url,
			port=excluded.port, password=excluded.password, data_dir=excluded.data_dir, work_dir=excluded.work_dir,
			proxy_token=excluded.proxy_token, status=excluded.status, created_at=excluded.created_at,
			last_activity_at=excluded.last_activity_at`,
		s.GroupKey, s.Adapter, s.AdapterVersion, s.Runtime, s.ProjectID, s.BundleHash, s.PID, s.URL, s.Port,
		password, s.DataDir, s.WorkDir, s.ProxyTokenHash, string(s.Status), s.CreatedAt, s.LastActivityAt)
	if err != nil {
		return fmt.Errorf("saving server %s: %w", s.GroupKey, err)
	}
	return nil
}

// Get returns a server by group key.
func (ss *ServerStore) Get(ctx context.Context, groupKey string) (*domain.Server, error) {
	s, err := ss.scanServer(ss.db.QueryRowContext(ctx, `SELECT `+serverColumns+` FROM servers WHERE group_key = ?`, groupKey))
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
		s, err := ss.scanServer(rows)
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

// SetStatusIf changes the status only if pid and status are unchanged.
func (ss *ServerStore) SetStatusIf(ctx context.Context, groupKey string, pid int, from, to domain.ServerStatus) (bool, error) {
	res, err := ss.db.ExecContext(ctx, `UPDATE servers SET status = ? WHERE group_key = ? AND pid = ? AND status = ?`,
		string(to), groupKey, pid, string(from))
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Touch records server activity.
func (ss *ServerStore) Touch(ctx context.Context, groupKey string, at time.Time) error {
	_, err := ss.db.ExecContext(ctx, `UPDATE servers SET last_activity_at = ? WHERE group_key = ?`, at, groupKey)
	return err
}

// Delete removes a server (and its password file).
func (ss *ServerStore) Delete(ctx context.Context, groupKey string) error {
	_, err := ss.db.ExecContext(ctx, `DELETE FROM servers WHERE group_key = ?`, groupKey)
	if err == nil && ss.dir != "" {
		_ = os.Remove(ss.passwordFile(groupKey))
	}
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
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, g.TokenHash, g.Owner, g.Provider, g.Region, string(src), string(models), g.MaxTokens, g.CreatedAt)
	if err != nil {
		return fmt.Errorf("saving proxy grant: %w", err)
	}
	return nil
}

var (
	_ domain.LegacyTokenHasher = (*GrantStore)(nil)
	_ domain.LegacyTokenHasher = (*ServerStore)(nil)
)

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
		if err := rows.Scan(&g.TokenHash, &g.Owner, &g.Provider, &g.Region, &src, &models, &g.MaxTokens, &g.CreatedAt); err != nil {
			return nil, err
		}
		// An unreadable source or model list is never read as "any model"
		// (audit v5, S-23): the grant is left out, its token refused.
		if err := json.Unmarshal([]byte(src), &g.Source); err != nil {
			slog.Warn("sqlite: proxy grant with an unreadable credential source, not restored", "owner", g.Owner, "error", err)
			continue
		}
		if err := json.Unmarshal([]byte(models), &g.AllowedModels); err != nil {
			slog.Warn("sqlite: proxy grant with an unreadable model list, not restored", "owner", g.Owner, "error", err)
			continue
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Revoke marks a grant revoked (by token hash).
func (gs *GrantStore) Revoke(ctx context.Context, tokenHash string, at time.Time) error {
	_, err := gs.db.ExecContext(ctx, `UPDATE proxy_grants SET revoked_at = ? WHERE token = ? AND revoked_at IS NULL`, at, tokenHash)
	return err
}

// HashLegacyTokens implements domain.LegacyTokenHasher (column token).
func (gs *GrantStore) HashLegacyTokens(ctx context.Context, prefix string, hash func(string) string) (int, error) {
	return hashLegacy(ctx, gs.db, "proxy_grants", "token", prefix, hash)
}

// HashLegacyTokens implements domain.LegacyTokenHasher (column proxy_token).
func (ss *ServerStore) HashLegacyTokens(ctx context.Context, prefix string, hash func(string) string) (int, error) {
	return hashLegacy(ctx, ss.db, "servers", "proxy_token", prefix, hash)
}

// CountLegacyTokens counts the values still stored in clear (Doctor).
func (gs *GrantStore) CountLegacyTokens(ctx context.Context, prefix string) (int, error) {
	return countLegacy(ctx, gs.db, "proxy_grants", "token", prefix)
}

// CountLegacyTokens counts the values still stored in clear (Doctor).
func (ss *ServerStore) CountLegacyTokens(ctx context.Context, prefix string) (int, error) {
	return countLegacy(ctx, ss.db, "servers", "proxy_token", prefix)
}

func countLegacy(ctx context.Context, db *sql.DB, table, column, prefix string) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE substr(`+column+`, 1, ?) = ?`, len(prefix), prefix).Scan(&n)
	return n, err
}

// hashLegacy replaces the values of column starting with prefix (tokens
// stored in clear by older versions) by their hash, in one transaction.
func hashLegacy(ctx context.Context, db *sql.DB, table, column, prefix string, hash func(string) string) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT `+column+` FROM `+table+` WHERE substr(`+column+`, 1, ?) = ?`, len(prefix), prefix)
	if err != nil {
		return 0, fmt.Errorf("listing legacy tokens in %s: %w", table, err)
	}
	var vals []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return 0, err
		}
		vals = append(vals, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, v := range vals {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET `+column+` = ? WHERE `+column+` = ?`, hash(v), v); err != nil {
			return 0, fmt.Errorf("hashing legacy tokens in %s: %w", table, err)
		}
	}
	return len(vals), tx.Commit()
}

// RevokeOrphans revokes the active grants created before createdBefore whose
// owner (a server group) has no row in the starting or ready status: no
// server can hold them. It reaches the rows ListActive leaves out
// (unreadable credential source or model list). The age is compared in Go:
// dates are stored in the driver's text format, not comparable in SQL.
func (gs *GrantStore) RevokeOrphans(ctx context.Context, createdBefore, at time.Time) (int, error) {
	tx, err := gs.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT token, created_at FROM proxy_grants
		WHERE revoked_at IS NULL AND owner NOT IN (SELECT group_key FROM servers WHERE status IN (?, ?))`,
		string(domain.ServerStarting), string(domain.ServerReady))
	if err != nil {
		return 0, fmt.Errorf("listing orphan proxy grants: %w", err)
	}
	var orphans []string
	for rows.Next() {
		var token string
		var created time.Time
		if err := rows.Scan(&token, &created); err != nil {
			rows.Close()
			return 0, fmt.Errorf("reading orphan proxy grants: %w", err)
		}
		if created.Before(createdBefore) {
			orphans = append(orphans, token)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, token := range orphans {
		if _, err := tx.ExecContext(ctx, `UPDATE proxy_grants SET revoked_at = ? WHERE token = ? AND revoked_at IS NULL`, at, token); err != nil {
			return 0, fmt.Errorf("revoking orphan proxy grants: %w", err)
		}
	}
	return len(orphans), tx.Commit()
}

// RevokeOwner revokes every grant of an owner.
func (gs *GrantStore) RevokeOwner(ctx context.Context, owner string, at time.Time) error {
	_, err := gs.db.ExecContext(ctx, `UPDATE proxy_grants SET revoked_at = ? WHERE owner = ? AND revoked_at IS NULL`, at, owner)
	return err
}
