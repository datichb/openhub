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

func TestRemoveProjectServiceTokensDeletesBothNamesAndTheRecordedKey(t *testing.T) {
	project := &domain.Project{ID: "p1", MCPConfig: &domain.ProjectMCPConfig{Services: []domain.ProjectMCPService{
		{Name: "gitlab", TokenKey: "custom-p1-key"},
		{Name: "jira", TokenKey: "jira-token-p1"},
	}}}
	store := &mockSecretStore{secrets: map[string]string{
		"openhub.mcp.gitlab.token.p1": "a",
		"gitlab-token-p1":             "b",
		"custom-p1-key":               "c",
		"openhub.mcp.gitlab.token":    "hub",
		"gitlab-token":                "hub legacy",
		"openhub.mcp.gitlab.token.p2": "other project",
		"jira-token-p1":               "j",
	}}

	removeProjectServiceTokens(t.Context(), store, project, "gitlab")

	assert.Equal(t, map[string]string{
		"openhub.mcp.gitlab.token":    "hub",
		"gitlab-token":                "hub legacy",
		"openhub.mcp.gitlab.token.p2": "other project",
		"jira-token-p1":               "j",
	}, store.secrets)
	assert.Empty(t, project.MCPConfig.Services[0].TokenKey, "the recorded key is forgotten")
	assert.Equal(t, "jira-token-p1", project.MCPConfig.Services[1].TokenKey)

	// A project recording the shared hub key never deletes it.
	project = &domain.Project{ID: "p1", MCPConfig: &domain.ProjectMCPConfig{Services: []domain.ProjectMCPService{
		{Name: "gitlab", TokenKey: "openhub.mcp.gitlab.token"},
	}}}
	removeProjectServiceTokens(t.Context(), store, project, "gitlab")
	assert.Contains(t, store.secrets, "openhub.mcp.gitlab.token")

	removeProjectServiceTokens(t.Context(), nil, project, "gitlab") // no secret store: no-op
}

func TestServiceRemoveHasAProjectFlag(t *testing.T) {
	assert.NotNil(t, serviceRemoveCmd.Flags().Lookup("project"))
}
