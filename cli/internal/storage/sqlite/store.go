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
	migrations := []struct {
		version int
		sql     string
	}{
		{1, `CREATE TABLE IF NOT EXISTS projects (
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
		)`},
		{2, `CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status)`},
		{3, `CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name)`},
		{4, `CREATE TABLE IF NOT EXISTS sessions (
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
		)`},
		{5, `CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_id)`},
		{6, `CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)`},
	}

	// Apply only unapplied migrations
	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}
		if _, err := s.db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration v%d: %w", m.version, err)
		}
		if _, err := s.db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			return fmt.Errorf("recording migration v%d: %w", m.version, err)
		}
	}
	return nil
}
