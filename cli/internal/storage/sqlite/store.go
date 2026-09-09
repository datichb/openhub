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

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	// Foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling foreign keys: %w", err)
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

// migration represents a single schema change with an optional rollback.
type migration struct {
	version     int
	up          string
	down        string // empty means irreversible
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

	// Get current schema version
	currentVersion := 0
	row := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	_ = row.Scan(&currentVersion)

	// Define migrations (ordered by version)
	for _, m := range schemaMigrations {
		if m.version <= currentVersion {
			continue
		}
		if _, err := s.db.Exec(m.up); err != nil {
			return fmt.Errorf("migration v%d: %w", m.version, err)
		}
		if _, err := s.db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			return fmt.Errorf("recording migration v%d: %w", m.version, err)
		}
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

	// Apply down-migrations in reverse order
	for i := len(schemaMigrations) - 1; i >= 0; i-- {
		m := schemaMigrations[i]
		if m.version <= targetVersion || m.version > currentVersion {
			continue
		}
		if m.irreversible {
			return fmt.Errorf("migration v%d is irreversible — cannot downgrade past this version", m.version)
		}
		if m.down == "" {
			return fmt.Errorf("migration v%d has no rollback defined", m.version)
		}
		if _, err := s.db.Exec(m.down); err != nil {
			return fmt.Errorf("rollback v%d: %w", m.version, err)
		}
		if _, err := s.db.Exec(`DELETE FROM schema_migrations WHERE version = ?`, m.version); err != nil {
			return fmt.Errorf("removing migration record v%d: %w", m.version, err)
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
		version:     7,
		up:          `ALTER TABLE projects ADD COLUMN provider TEXT NOT NULL DEFAULT ''`,
		down:        `ALTER TABLE projects DROP COLUMN provider`,
		irreversible: false,
	},
	{
		version:     8,
		up:          `ALTER TABLE projects ADD COLUMN model TEXT NOT NULL DEFAULT ''`,
		down:        `ALTER TABLE projects DROP COLUMN model`,
		irreversible: false,
	},
	{
		version:     9,
		up:          `ALTER TABLE projects ADD COLUMN model_overrides TEXT NOT NULL DEFAULT ''`,
		down:        `ALTER TABLE projects DROP COLUMN model_overrides`,
		irreversible: false,
	},
	{
		version:     10,
		up:          `ALTER TABLE projects ADD COLUMN mcp_config TEXT NOT NULL DEFAULT ''`,
		down:        `ALTER TABLE projects DROP COLUMN mcp_config`,
		irreversible: false,
	},
	{
		version:     11,
		up:          `ALTER TABLE projects ADD COLUMN provider_config TEXT NOT NULL DEFAULT ''`,
		down:        `ALTER TABLE projects DROP COLUMN provider_config`,
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
}

