package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestProjectStore_CRUD(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	// Create
	p := &domain.Project{
		ID:        "test-1",
		Name:      "Test Project",
		Path:      "/tmp/test-project",
		Language:  "go",
		Labels:    []string{"backend", "cli"},
		Agents:    []string{"coder", "reviewer"},
		MCP:       []string{"gitlab"},
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, ps.Create(ctx, p))

	// Get
	got, err := ps.Get(ctx, "test-1")
	require.NoError(t, err)
	assert.Equal(t, "Test Project", got.Name)
	assert.Equal(t, "/tmp/test-project", got.Path)
	assert.Equal(t, "go", got.Language)
	assert.Equal(t, domain.ProjectStatusActive, got.Status)
	assert.Equal(t, []string{"backend", "cli"}, got.Labels)
	assert.Equal(t, []string{"coder", "reviewer"}, got.Agents)

	// GetByPath
	byPath, err := ps.GetByPath(ctx, "/tmp/test-project")
	require.NoError(t, err)
	assert.Equal(t, "test-1", byPath.ID)

	// List
	projects, err := ps.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, projects, 1)

	// Update
	got.Language = "rust"
	got.Labels = []string{"backend", "cli", "systems"}
	require.NoError(t, ps.Update(ctx, got))
	updated, err := ps.Get(ctx, "test-1")
	require.NoError(t, err)
	assert.Equal(t, "rust", updated.Language)
	assert.Equal(t, []string{"backend", "cli", "systems"}, updated.Labels)

	// Delete
	require.NoError(t, ps.Delete(ctx, "test-1"))
	_, err = ps.Get(ctx, "test-1")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestProjectStore_ListByStatus(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	now := time.Now()
	require.NoError(t, ps.Create(ctx, &domain.Project{ID: "p1", Name: "P1", Path: "/p1", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, ps.Create(ctx, &domain.Project{ID: "p2", Name: "P2", Path: "/p2", Status: domain.ProjectStatusArchived, CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, ps.Create(ctx, &domain.Project{ID: "p3", Name: "P3", Path: "/p3", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}))

	active, err := ps.List(ctx, domain.ProjectStatusActive)
	require.NoError(t, err)
	assert.Len(t, active, 2)

	archived, err := ps.List(ctx, domain.ProjectStatusArchived)
	require.NoError(t, err)
	assert.Len(t, archived, 1)

	all, err := ps.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestProjectStore_GetNotFound(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	_, err := ps.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestProjectStore_CreateDuplicate(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	now := time.Now()
	p := &domain.Project{ID: "p1", Name: "P1", Path: "/same/path", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, ps.Create(ctx, p))

	p2 := &domain.Project{ID: "p2", Name: "P2", Path: "/same/path", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}
	err := ps.Create(ctx, p2)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestProjectStore_DeleteNotFound(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	err := ps.Delete(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
