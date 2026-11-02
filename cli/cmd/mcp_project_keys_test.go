package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// The startup migration, on a real database: the legacy project key is
// copied, TokenKey is rewritten in the database, the legacy key is kept, and
// a second run changes nothing.
func TestProjectTokenKeyMigrationOnTheDatabase(t *testing.T) {
	s, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ps := sqlite.NewProjectStore(s)
	ctx := t.Context()

	now := time.Now().Truncate(time.Second)
	require.NoError(t, ps.Create(ctx, &domain.Project{
		ID: "p1", Name: "one", Path: "/tmp/one", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now,
		MCPConfig: &domain.ProjectMCPConfig{Services: []domain.ProjectMCPService{{Name: "gitlab", TokenKey: "gitlab-token-p1"}}},
	}))
	store := &mockSecretStore{secrets: map[string]string{"gitlab-token-p1": "tok"}}

	n, err := config.MigrateProjectTokenKeys(ctx, store, ps)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	got, err := ps.Get(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, "openhub.mcp.gitlab.token.p1", got.MCPConfig.Services[0].TokenKey)
	assert.Equal(t, "tok", store.secrets["openhub.mcp.gitlab.token.p1"])
	assert.Equal(t, "tok", store.secrets["gitlab-token-p1"])

	n, err = config.MigrateProjectTokenKeys(ctx, store, ps)
	require.NoError(t, err)
	assert.Zero(t, n)
}

// checkProjectToken finds a project token under either name.
func TestCheckProjectTokenReadsBothNames(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	p := &domain.Project{ID: "p1"}
	for _, key := range []string{"openhub.mcp.gitlab.token.p1", "gitlab-token-p1"} {
		a := &app.App{Secrets: &mockSecretStore{secrets: map[string]string{key: "tok"}}}
		assert.True(t, checkProjectToken(t.Context(), a, "gitlab", p), key)
	}
	a := &app.App{Secrets: &mockSecretStore{secrets: map[string]string{"gitlab-token-p2": "tok"}}}
	assert.False(t, checkProjectToken(t.Context(), a, "gitlab", p), "another project's token")
}
