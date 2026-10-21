package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

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

// What the store refuses: an unknown project to read or delete, a second
// project on the same path or under the same name.
func TestProjectStore_Errors(t *testing.T) {
	existing := &domain.Project{ID: "p1", Name: "P1", Path: "/same/path", Status: domain.ProjectStatusActive}
	for _, tc := range []struct {
		name string
		do   func(ps *ProjectStore) error
		want error
	}{
		{"get unknown", func(ps *ProjectStore) error { _, err := ps.Get(context.Background(), "nonexistent"); return err }, domain.ErrNotFound},
		{"delete unknown", func(ps *ProjectStore) error { return ps.Delete(context.Background(), "nonexistent") }, domain.ErrNotFound},
		{"create on the same path", func(ps *ProjectStore) error {
			return ps.Create(context.Background(), &domain.Project{ID: "p2", Name: "P2", Path: "/same/path", Status: domain.ProjectStatusActive})
		}, domain.ErrAlreadyExists},
		{"create under the same name", func(ps *ProjectStore) error {
			return ps.Create(context.Background(), &domain.Project{ID: "p2", Name: "P1", Path: "/other/path", Status: domain.ProjectStatusActive})
		}, domain.ErrAlreadyExists},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ps := NewProjectStore(openTestStore(t))
			now := time.Now()
			p := *existing
			p.CreatedAt, p.UpdatedAt = now, now
			require.NoError(t, ps.Create(context.Background(), &p), "the project already there")
			assert.ErrorIs(t, tc.do(ps), tc.want)
		})
	}
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

func TestProjectStore_TeamConfigRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	tc := &domain.ProjectTeamConfig{
		Mode:      domain.ProjectTeamModeCustom,
		StateRepo: "git@gitlab.com:acme/other-team.git",
		StatePath: "/home/alice/.oh/team-states/other-team",
		MemberID:  "bob",
	}

	p := &domain.Project{
		ID:         "team-cfg-1",
		Name:       "Team Config Project",
		Path:       "/tmp/team-cfg",
		Language:   "go",
		Status:     domain.ProjectStatusActive,
		TeamConfig: tc,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	require.NoError(t, ps.Create(ctx, p))

	got, err := ps.Get(ctx, "team-cfg-1")
	require.NoError(t, err)
	require.NotNil(t, got.TeamConfig)
	assert.Equal(t, domain.ProjectTeamModeCustom, got.TeamConfig.Mode)
	assert.Equal(t, "git@gitlab.com:acme/other-team.git", got.TeamConfig.StateRepo)
	assert.Equal(t, "/home/alice/.oh/team-states/other-team", got.TeamConfig.StatePath)
	assert.Equal(t, "bob", got.TeamConfig.MemberID)
}

func TestProjectStore_TeamConfigNilRoundTrip(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	p := &domain.Project{
		ID:         "team-cfg-nil",
		Name:       "No Team Project",
		Path:       "/tmp/no-team",
		Language:   "typescript",
		Status:     domain.ProjectStatusActive,
		TeamConfig: nil, // explicit nil — inherit from hub
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	require.NoError(t, ps.Create(ctx, p))

	got, err := ps.Get(ctx, "team-cfg-nil")
	require.NoError(t, err)
	assert.Nil(t, got.TeamConfig, "nil TeamConfig should round-trip as nil (inherit mode)")
}

func TestProjectStore_TeamConfigUpdateCycle(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	// Step 1: create without TeamConfig
	p := &domain.Project{
		ID:        "team-cfg-update",
		Name:      "Update Team Project",
		Path:      "/tmp/update-team",
		Language:  "python",
		Status:    domain.ProjectStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, ps.Create(ctx, p))

	got, err := ps.Get(ctx, "team-cfg-update")
	require.NoError(t, err)
	assert.Nil(t, got.TeamConfig)

	// Step 2: update to disabled mode
	got.TeamConfig = &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeDisabled}
	require.NoError(t, ps.Update(ctx, got))

	got2, err := ps.Get(ctx, "team-cfg-update")
	require.NoError(t, err)
	require.NotNil(t, got2.TeamConfig)
	assert.Equal(t, domain.ProjectTeamModeDisabled, got2.TeamConfig.Mode)
	assert.Empty(t, got2.TeamConfig.StateRepo)

	// Step 3: update back to nil (inherit)
	got2.TeamConfig = nil
	require.NoError(t, ps.Update(ctx, got2))

	got3, err := ps.Get(ctx, "team-cfg-update")
	require.NoError(t, err)
	assert.Nil(t, got3.TeamConfig, "clearing TeamConfig to nil should persist as nil")
}
