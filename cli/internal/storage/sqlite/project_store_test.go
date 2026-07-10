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

func TestProjectStore_MCPConfigRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	now := time.Now()
	writeEnabled := true
	p := &domain.Project{
		ID:     "mcp-test",
		Name:   "MCP Test",
		Path:   "/mcp/test",
		Status: domain.ProjectStatusActive,
		MCPConfig: &domain.ProjectMCPConfig{
			Services: []domain.ProjectMCPService{
				{Name: "figma"},
				{Name: "gitlab", TokenKey: "gitlab-token-custom", WriteEnabled: &writeEnabled},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, ps.Create(ctx, p))

	got, err := ps.Get(ctx, "mcp-test")
	require.NoError(t, err)
	require.NotNil(t, got.MCPConfig)
	assert.Len(t, got.MCPConfig.Services, 2)
	assert.Equal(t, "figma", got.MCPConfig.Services[0].Name)
	assert.Empty(t, got.MCPConfig.Services[0].TokenKey)
	assert.Equal(t, "gitlab", got.MCPConfig.Services[1].Name)
	assert.Equal(t, "gitlab-token-custom", got.MCPConfig.Services[1].TokenKey)
	require.NotNil(t, got.MCPConfig.Services[1].WriteEnabled)
	assert.True(t, *got.MCPConfig.Services[1].WriteEnabled)
}

func TestProjectStore_MCPConfigBackwardCompat(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	now := time.Now()
	// Create project with legacy MCP field only (no mcp_config)
	p := &domain.Project{
		ID:        "legacy-test",
		Name:      "Legacy",
		Path:      "/legacy",
		MCP:       []string{"figma", "gslides"},
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	require.NoError(t, ps.Create(ctx, p))

	got, err := ps.Get(ctx, "legacy-test")
	require.NoError(t, err)
	// MCPConfig should be auto-populated from legacy MCP field
	require.NotNil(t, got.MCPConfig)
	assert.Len(t, got.MCPConfig.Services, 2)
	assert.Equal(t, "figma", got.MCPConfig.Services[0].Name)
	assert.Equal(t, "gslides", got.MCPConfig.Services[1].Name)
	// No credential overrides in migrated config
	assert.Empty(t, got.MCPConfig.Services[0].TokenKey)
}

func TestProjectStore_GetByName(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	now := time.Now()
	p := &domain.Project{
		ID:        "proj-abc123",
		Name:      "My Cool Project",
		Path:      "/home/user/cool",
		Language:  "typescript",
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, ps.Create(ctx, p))

	// Retrieve by name
	got, err := ps.GetByName(ctx, "My Cool Project")
	require.NoError(t, err)
	assert.Equal(t, "proj-abc123", got.ID)
	assert.Equal(t, "My Cool Project", got.Name)
	assert.Equal(t, "typescript", got.Language)

	// Not found by name
	_, err = ps.GetByName(ctx, "Nonexistent Project")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestProjectStore_CreateDuplicateName(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	now := time.Now()
	p1 := &domain.Project{ID: "id-1", Name: "Same Name", Path: "/path/one", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, ps.Create(ctx, p1))

	// Creating another project with the same name but different path/id should fail
	p2 := &domain.Project{ID: "id-2", Name: "Same Name", Path: "/path/two", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}
	err := ps.Create(ctx, p2)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}
