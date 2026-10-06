// Package sqlite provides SQLite-backed implementations of domain store interfaces.
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/config"
	_ "modernc.org/sqlite"
)

// Store wraps the SQLite connection shared across all store implementations.
type Store struct {
	db *sql.DB
}

// DBPath returns the default path to the SQLite database.
func DBPath() string {
	return filepath.Join(config.HubDir(), "oh.db")
}

// Open opens (or creates) the SQLite database and runs migrations.
func Open(path string) (*Store, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	// The database holds server passwords and proxy tokens: owner-only
	// access. SQLite creates the -wal/-shm files with the mode of the main
	// file, so it is created (or fixed) before the first connection.
	_ = os.Chmod(filepath.Dir(path), 0o700)
	if f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600); err == nil {
		f.Close()
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		_ = os.Chmod(p, 0o600)
	}

	// Pragmas in the DSN apply to every pooled connection (the oh daemon and
	// the CLI/TUI processes share this database: wait for locks instead of
	// failing immediately).
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("opening database: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

// OpenDefault opens the database at the default path.
func OpenDefault() (*Store, error) {
	return Open(DBPath())
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB (for sub-stores or transactions).
func (s *Store) DB() *sql.DB {
	return s.db
}

// SchemaVersion returns the current applied schema version.
func (s *Store) SchemaVersion() (int, error) {
	var version int
	row := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	if err := row.Scan(&version); err != nil {
		return 0, fmt.Errorf("reading schema version: %w", err)
	}
	return version, nil
}

// IntegrityCheck runs SQLite's PRAGMA integrity_check and returns nil if the database is healthy.
func (s *Store) IntegrityCheck() error {
	rows, err := s.db.Query("PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("integrity_check query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("scanning integrity_check row: %w", err)
		}
		if result != "ok" {
			return fmt.Errorf("database integrity failure: %s", result)
		}
	}
	return rows.Err()
}

// sequentialMigrations is the last migration of the strictly ordered era;
// later versions are reserved by parallel branches (v5 phase 1+).
const sequentialMigrations = 31

// migration represents a single schema change with an optional rollback.
type migration struct {
	version      int
	up           string
	down         string // empty means irreversible
	irreversible bool
}

func (s *Store) migrate() error {
	// Ensure schema_migrations table exists
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	// Applied versions. Up to sequentialMigrations, a version below the
	// highest applied one counts as applied (historical behaviour). Above it,
	// every missing migration is applied: parallel branches reserve version
	// numbers and may merge out of order (a v32 merged after a v33 must run).
	applied := map[int]bool{}
	maxApplied := 0
	rows, err := s.db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("reading applied migrations: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("reading applied migrations: %w", err)
		}
		applied[v] = true
		maxApplied = max(maxApplied, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading applied migrations: %w", err)
	}

	// Define migrations (ordered by version).
	// Each migration runs inside a transaction so that the schema change and the
	// version record are committed atomically. Without this, a crash between the
	// ALTER TABLE and the INSERT INTO schema_migrations leaves the database in an
	// inconsistent state where the column exists but the migration is re-attempted
	// on next startup, causing a "duplicate column name" error.
	for _, m := range schemaMigrations {
		if applied[m.version] || (m.version <= sequentialMigrations && m.version <= maxApplied) {
			continue
		}
		if err := s.runMigration(m); err != nil {
			return err
		}
	}
	return nil
}

// appliedVersions returns the recorded migration versions.
func (s *Store) appliedVersions() (map[int]bool, error) {
	rows, err := s.db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("reading applied migrations: %w", err)
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

// runMigration executes a single migration inside a transaction so that the
// schema change and the version record are committed atomically.
func (s *Store) runMigration(m migration) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction for migration v%d: %w", m.version, err)
	}
	if _, err := tx.Exec(m.up); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("migration v%d: %w", m.version, err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("recording migration v%d: %w", m.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing migration v%d: %w", m.version, err)
	}
	return nil
}

// MigrateDown rolls back migrations from the current version down to targetVersion (exclusive).
// Only reversible migrations can be rolled back; irreversible ones return an error.
func (s *Store) MigrateDown(targetVersion int) error {
	currentVersion, err := s.SchemaVersion()
	if err != nil {
		return err
	}
	if targetVersion >= currentVersion {
		return nil // nothing to do
	}
	applied, err := s.appliedVersions()
	if err != nil {
		return err
	}

	// Apply down-migrations in reverse order
	for i := len(schemaMigrations) - 1; i >= 0; i-- {
		m := schemaMigrations[i]
		if m.version <= targetVersion || m.version > currentVersion || (!applied[m.version] && m.version > sequentialMigrations) {
			continue
		}
		if m.irreversible {
			return fmt.Errorf("migration v%d is irreversible — cannot downgrade past this version", m.version)
		}
		if m.down == "" {
			return fmt.Errorf("migration v%d has no rollback defined", m.version)
		}
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("beginning transaction for rollback v%d: %w", m.version, err)
		}
		if _, err := tx.Exec(m.down); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("rollback v%d: %w", m.version, err)
		}
		if _, err := tx.Exec(`DELETE FROM schema_migrations WHERE version = ?`, m.version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("removing migration record v%d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing rollback v%d: %w", m.version, err)
		}
	}
	return nil
}

// schemaMigrations is the ordered list of all schema migrations.
// down is empty for irreversible operations (column drops on old SQLite, etc.).
var schemaMigrations = []migration{
	{
		version: 1,
		up: `CREATE TABLE IF NOT EXISTS projects (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			path       TEXT NOT NULL UNIQUE,
			language   TEXT NOT NULL DEFAULT '',
			tracker    TEXT NOT NULL DEFAULT '',
			labels     TEXT NOT NULL DEFAULT '',
			agents     TEXT NOT NULL DEFAULT '',
			mcp        TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		down: `DROP TABLE IF EXISTS projects`,
	},
	{
		version: 2,
		up:      `CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status)`,
		down:    `DROP INDEX IF EXISTS idx_projects_status`,
	},
	{
		version: 3,
		up:      `CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name)`,
		down:    `DROP INDEX IF EXISTS idx_projects_name`,
	},
	{
		version: 4,
		up: `CREATE TABLE IF NOT EXISTS sessions (
			id         TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ended_at   DATETIME,
			status     TEXT NOT NULL DEFAULT 'running',
			provider   TEXT NOT NULL DEFAULT '',
			model      TEXT NOT NULL DEFAULT '',
			tokens_in  INTEGER NOT NULL DEFAULT 0,
			tokens_out INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		)`,
		down: `DROP TABLE IF EXISTS sessions`,
	},
	{
		version: 5,
		up:      `CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_id)`,
		down:    `DROP INDEX IF EXISTS idx_sessions_project`,
	},
	{
		version: 6,
		up:      `CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)`,
		down:    `DROP INDEX IF EXISTS idx_sessions_status`,
	},
	{
		version:      7,
		up:           `ALTER TABLE projects ADD COLUMN provider TEXT NOT NULL DEFAULT ''`,
		down:         `ALTER TABLE projects DROP COLUMN provider`,
		irreversible: false,
	},
	{
		version:      8,
		up:           `ALTER TABLE projects ADD COLUMN model TEXT NOT NULL DEFAULT ''`,
		down:         `ALTER TABLE projects DROP COLUMN model`,
		irreversible: false,
	},
	{
		version:      9,
		up:           `ALTER TABLE projects ADD COLUMN model_overrides TEXT NOT NULL DEFAULT ''`,
		down:         `ALTER TABLE projects DROP COLUMN model_overrides`,
		irreversible: false,
	},
	{
		version:      10,
		up:           `ALTER TABLE projects ADD COLUMN mcp_config TEXT NOT NULL DEFAULT ''`,
		down:         `ALTER TABLE projects DROP COLUMN mcp_config`,
		irreversible: false,
	},
	{
		version:      11,
		up:           `ALTER TABLE projects ADD COLUMN provider_config TEXT NOT NULL DEFAULT ''`,
		down:         `ALTER TABLE projects DROP COLUMN provider_config`,
		irreversible: false,
	},
	{
		version: 12,
		up:      `DROP INDEX IF EXISTS idx_projects_name; CREATE UNIQUE INDEX idx_projects_name_unique ON projects(name)`,
		down:    `DROP INDEX IF EXISTS idx_projects_name_unique; CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name)`,
	},
	{
		version: 13,
		up: `CREATE TABLE IF NOT EXISTS agent_events (
			id            TEXT PRIMARY KEY,
			session_id    TEXT NOT NULL,
			project_id    TEXT NOT NULL,
			agent_name    TEXT NOT NULL,
			skills_loaded TEXT NOT NULL DEFAULT '[]',
			started_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at  DATETIME,
			status        TEXT NOT NULL DEFAULT 'success',
			tokens_in     INTEGER NOT NULL DEFAULT 0,
			tokens_out    INTEGER NOT NULL DEFAULT 0,
			cost_usd      REAL NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT ''
		)`,
		down: `DROP TABLE IF EXISTS agent_events`,
	},
	{
		version: 14,
		up:      `CREATE INDEX IF NOT EXISTS idx_agent_events_session ON agent_events(session_id)`,
		down:    `DROP INDEX IF EXISTS idx_agent_events_session`,
	},
	{
		version: 15,
		up:      `CREATE INDEX IF NOT EXISTS idx_agent_events_agent ON agent_events(agent_name)`,
		down:    `DROP INDEX IF EXISTS idx_agent_events_agent`,
	},
	{
		version: 16,
		up:      `CREATE INDEX IF NOT EXISTS idx_agent_events_project ON agent_events(project_id)`,
		down:    `DROP INDEX IF EXISTS idx_agent_events_project`,
	},
	{
		version:      17,
		up:           `ALTER TABLE projects ADD COLUMN team_config TEXT NOT NULL DEFAULT ''`,
		down:         `ALTER TABLE projects DROP COLUMN team_config`,
		irreversible: false,
	},
	{
		version:      18,
		up:           `ALTER TABLE projects ADD COLUMN tracker_config TEXT NOT NULL DEFAULT ''`,
		down:         `ALTER TABLE projects DROP COLUMN tracker_config`,
		irreversible: false,
	},
	{
		version:      19,
		up:           `ALTER TABLE sessions ADD COLUMN launch_path TEXT NOT NULL DEFAULT ''`,
		down:         `ALTER TABLE sessions DROP COLUMN launch_path`,
		irreversible: false,
	},
	{
		version:      20,
		up:           `ALTER TABLE projects ADD COLUMN team_id TEXT DEFAULT NULL`,
		down:         `ALTER TABLE projects DROP COLUMN team_id`,
		irreversible: false,
	},
	{
		version:      21,
		up:           `ALTER TABLE projects ADD COLUMN workflow_config TEXT NOT NULL DEFAULT ''`,
		down:         `ALTER TABLE projects DROP COLUMN workflow_config`,
		irreversible: false,
	},
	{
		version:      22,
		up:           `ALTER TABLE sessions ADD COLUMN member_id TEXT DEFAULT NULL`,
		down:         `ALTER TABLE sessions DROP COLUMN member_id`,
		irreversible: false,
	},
	{
		version:      23,
		up:           `ALTER TABLE agent_events ADD COLUMN member_id TEXT DEFAULT NULL`,
		down:         `ALTER TABLE agent_events DROP COLUMN member_id`,
		irreversible: false,
	},
	{
		version: 24,
		up: `ALTER TABLE sessions ADD COLUMN cost REAL NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN tokens_reasoning INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN tokens_cache_read INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN platform TEXT NOT NULL DEFAULT 'opencode';
ALTER TABLE sessions ADD COLUMN external_session_id TEXT DEFAULT NULL;
ALTER TABLE sessions ADD COLUMN slug TEXT DEFAULT NULL`,
		irreversible: false,
	},
	{
		version:      25,
		up:           `ALTER TABLE sessions ADD COLUMN pid INTEGER NOT NULL DEFAULT 0`,
		irreversible: false,
	},
	{
		version:      26,
		up:           `ALTER TABLE sessions ADD COLUMN title TEXT DEFAULT NULL`,
		irreversible: false,
	},
	{
		version: 27,
		up: `ALTER TABLE sessions ADD COLUMN type TEXT NOT NULL DEFAULT 'interactive';
ALTER TABLE sessions ADD COLUMN label TEXT DEFAULT NULL;
ALTER TABLE sessions ADD COLUMN correlation_id TEXT DEFAULT NULL`,
		irreversible: false,
	},
	{
		version: 28,
		up: `CREATE TABLE IF NOT EXISTS servers (
			group_key        TEXT PRIMARY KEY,
			adapter          TEXT NOT NULL,
			adapter_version  TEXT NOT NULL DEFAULT '',
			runtime          TEXT NOT NULL DEFAULT 'local',
			project_id       TEXT NOT NULL DEFAULT '',
			bundle_hash      TEXT NOT NULL DEFAULT '',
			pid              INTEGER NOT NULL DEFAULT 0,
			url              TEXT NOT NULL DEFAULT '',
			port             INTEGER NOT NULL DEFAULT 0,
			password         TEXT NOT NULL DEFAULT '',
			data_dir         TEXT NOT NULL DEFAULT '',
			work_dir         TEXT NOT NULL DEFAULT '',
			proxy_token      TEXT NOT NULL DEFAULT '',
			status           TEXT NOT NULL DEFAULT 'starting',
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_activity_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		down: `DROP TABLE IF EXISTS servers`,
	},
	{
		version: 29,
		up: `ALTER TABLE sessions ADD COLUMN workflow_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN entry_agent TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN bundle_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN group_key TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN runtime TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN mode TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN state TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_sessions_group ON sessions(group_key)`,
		irreversible: false,
	},
	{
		version: 30,
		up: `CREATE TABLE IF NOT EXISTS proxy_grants (
			token          TEXT PRIMARY KEY,
			owner          TEXT NOT NULL,
			provider       TEXT NOT NULL,
			region         TEXT NOT NULL DEFAULT '',
			source         TEXT NOT NULL DEFAULT '{}',
			allowed_models TEXT NOT NULL DEFAULT '[]',
			max_tokens     INTEGER NOT NULL DEFAULT 0,
			created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			revoked_at     DATETIME
		);
CREATE INDEX IF NOT EXISTS idx_proxy_grants_owner ON proxy_grants(owner)`,
		down: `DROP TABLE IF EXISTS proxy_grants`,
	},
	{
		version:      31,
		up:           `ALTER TABLE sessions ADD COLUMN state_changed_at DATETIME DEFAULT NULL`,
		irreversible: false,
	},
	{
		// v5 phase 1 (P1-T21): pinned workflows and UI settings. scope =
		// "global" | "project:<id>" | "team:<id>"; value = JSON.
		version: 32,
		up: `CREATE TABLE IF NOT EXISTS preferences (
			scope      TEXT NOT NULL,
			key        TEXT NOT NULL,
			value      TEXT NOT NULL DEFAULT 'null',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (scope, key)
		)`,
		down: `DROP TABLE IF EXISTS preferences`,
	},
	{
		version: 33,
		up: `CREATE TABLE IF NOT EXISTS pending_decisions (
			id          TEXT PRIMARY KEY,
			session_id  TEXT NOT NULL,
			group_key   TEXT NOT NULL DEFAULT '',
			kind        TEXT NOT NULL,
			tool_ref    TEXT NOT NULL DEFAULT '',
			payload     TEXT NOT NULL DEFAULT '{}',
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME,
			resolved_by TEXT NOT NULL DEFAULT '',
			resolution  TEXT NOT NULL DEFAULT ''
		);
CREATE INDEX IF NOT EXISTS idx_pending_decisions_open ON pending_decisions(resolved_at, session_id);
CREATE INDEX IF NOT EXISTS idx_pending_decisions_session ON pending_decisions(session_id)`,
		down: `DROP TABLE IF EXISTS pending_decisions`,
	},
}
