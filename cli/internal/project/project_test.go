package project

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func openTestDB(dir string) (*DB, error) {
	dbPath := filepath.Join(dir, "test_projects.db")
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, err
	}
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

func TestDB_CRUD(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := openTestDB(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	now := time.Now().Truncate(time.Second)

	// Create
	p := &Project{
		ID:        "test-project-1",
		Name:      "Test Project",
		Path:      "/tmp/test-project",
		Language:  "go",
		Tracker:   "github",
		Labels:    "backend,cli",
		Agents:    "coder,reviewer",
		MCP:       "gitlab",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, db.Create(p))

	// Get
	got, err := db.Get("test-project-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Test Project", got.Name)
	assert.Equal(t, "/tmp/test-project", got.Path)
	assert.Equal(t, "go", got.Language)
	assert.Equal(t, "active", got.Status)

	// List
	projects, err := db.List("")
	require.NoError(t, err)
	assert.Len(t, projects, 1)

	// Update
	got.Language = "rust"
	require.NoError(t, db.Update(got))
	updated, err := db.Get("test-project-1")
	require.NoError(t, err)
	assert.Equal(t, "rust", updated.Language)

	// Delete
	require.NoError(t, db.Delete("test-project-1"))
	deleted, err := db.Get("test-project-1")
	require.NoError(t, err)
	assert.Nil(t, deleted)
}

func TestDB_ListByStatus(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := openTestDB(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()
	require.NoError(t, db.Create(&Project{ID: "p1", Name: "P1", Path: "/p1", Status: "active", CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, db.Create(&Project{ID: "p2", Name: "P2", Path: "/p2", Status: "archived", CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, db.Create(&Project{ID: "p3", Name: "P3", Path: "/p3", Status: "active", CreatedAt: now, UpdatedAt: now}))

	active, err := db.List("active")
	require.NoError(t, err)
	assert.Len(t, active, 2)

	archived, err := db.List("archived")
	require.NoError(t, err)
	assert.Len(t, archived, 1)

	all, err := db.List("")
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestDB_GetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := openTestDB(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	got, err := db.Get("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestDB_CreateDuplicatePath(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := openTestDB(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()
	p1 := &Project{ID: "p1", Name: "P1", Path: "/same/path", Status: "active", CreatedAt: now, UpdatedAt: now}
	p2 := &Project{ID: "p2", Name: "P2", Path: "/same/path", Status: "active", CreatedAt: now, UpdatedAt: now}

	require.NoError(t, db.Create(p1))
	assert.Error(t, db.Create(p2)) // UNIQUE constraint on path
}
