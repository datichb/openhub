// Package project manages the project registry backed by SQLite.
package project

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/datichb/openhub/cli/internal/config"
	_ "modernc.org/sqlite"
)

// Project represents a registered project.
type Project struct {
	ID        string
	Name      string
	Path      string
	Language  string
	Tracker   string
	Labels    string // comma-separated
	Agents    string // comma-separated
	MCP       string // comma-separated
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DB wraps the SQLite connection for the project registry.
type DB struct {
	conn *sql.DB
}

// DBPath returns the path to the projects database.
func DBPath() string {
	return filepath.Join(config.HubDir(), "projects.db")
}

// Open opens (or creates) the project database.
func Open() (*DB, error) {
	dbPath := DBPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS projects (
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
	);

	CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
	CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// List returns all projects matching the given status (empty = all).
func (db *DB) List(status string) ([]Project, error) {
	query := "SELECT id, name, path, language, tracker, labels, agents, mcp, status, created_at, updated_at FROM projects"
	var args []interface{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY name ASC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Language, &p.Tracker, &p.Labels, &p.Agents, &p.MCP, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// Get retrieves a project by ID.
func (db *DB) Get(id string) (*Project, error) {
	var p Project
	err := db.conn.QueryRow(
		"SELECT id, name, path, language, tracker, labels, agents, mcp, status, created_at, updated_at FROM projects WHERE id = ?",
		id,
	).Scan(&p.ID, &p.Name, &p.Path, &p.Language, &p.Tracker, &p.Labels, &p.Agents, &p.MCP, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Create inserts a new project.
func (db *DB) Create(p *Project) error {
	_, err := db.conn.Exec(
		`INSERT INTO projects (id, name, path, language, tracker, labels, agents, mcp, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Path, p.Language, p.Tracker, p.Labels, p.Agents, p.MCP, p.Status, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// Update modifies an existing project.
func (db *DB) Update(p *Project) error {
	p.UpdatedAt = time.Now()
	_, err := db.conn.Exec(
		`UPDATE projects SET name=?, path=?, language=?, tracker=?, labels=?, agents=?, mcp=?, status=?, updated_at=?
		 WHERE id=?`,
		p.Name, p.Path, p.Language, p.Tracker, p.Labels, p.Agents, p.MCP, p.Status, p.UpdatedAt, p.ID,
	)
	return err
}

// Delete removes a project by ID.
func (db *DB) Delete(id string) error {
	_, err := db.conn.Exec("DELETE FROM projects WHERE id = ?", id)
	return err
}
