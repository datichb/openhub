package config

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// memProjects is an in-memory project store.
type memProjects struct {
	domain.ProjectStore
	byID      map[string]*domain.Project
	updates   int
	updateErr error
}

func (m *memProjects) List(context.Context, domain.ProjectStatus) ([]domain.Project, error) {
	var out []domain.Project
	for _, p := range m.byID {
		out = append(out, *p)
	}
	return out, nil
}

func (m *memProjects) Get(_ context.Context, id string) (*domain.Project, error) {
	p, ok := m.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (m *memProjects) Update(_ context.Context, p *domain.Project) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updates++
	cp := *p
	m.byID[p.ID] = &cp
	return nil
}

func projectWith(id string, svcs ...domain.ProjectMCPService) *domain.Project {
	return &domain.Project{ID: id, MCPConfig: &domain.ProjectMCPConfig{Services: svcs}}
}

func TestMigrateProjectTokenKeys(t *testing.T) {
	ctx := t.Context()
	projects := &memProjects{byID: map[string]*domain.Project{
		"p1": projectWith("p1",
			domain.ProjectMCPService{Name: "gitlab", TokenKey: "gitlab-token-p1"},
			domain.ProjectMCPService{Name: "jira", TokenKey: "jira-token-p1"}, // key absent from the keychain
			domain.ProjectMCPService{Name: "figma", TokenKey: "custom-name"},  // not ours
			domain.ProjectMCPService{Name: "gslides"},                         // inherits the hub
		),
		"p2": projectWith("p2", domain.ProjectMCPService{Name: "gitlab", TokenKey: "openhub.mcp.gitlab.token.p2"}), // already migrated
		"p3": {ID: "p3"},                                                                                           // no MCP config
		"p4": projectWith("p4", domain.ProjectMCPService{Name: "gitlab", TokenKey: "gitlab-token-other"}),          // another project's name: left alone
	}}
	store := newMemStore(map[string]string{
		"gitlab-token-p1":             "tok1",
		"openhub.mcp.gitlab.token.p2": "tok2",
		"gitlab-token":                "hub",
		"custom-name":                 "c",
	})

	n, err := MigrateProjectTokenKeys(ctx, store, projects)
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	svcs := projects.byID["p1"].MCPConfig.Services
	assert.Equal(t, "openhub.mcp.gitlab.token.p1", svcs[0].TokenKey)
	assert.Equal(t, "openhub.mcp.jira.token.p1", svcs[1].TokenKey, "a missing key is tolerated")
	assert.Equal(t, "custom-name", svcs[2].TokenKey)
	assert.Empty(t, svcs[3].TokenKey)
	assert.Equal(t, "gitlab-token-other", projects.byID["p4"].MCPConfig.Services[0].TokenKey)

	assert.Equal(t, "tok1", store.m["openhub.mcp.gitlab.token.p1"], "value copied")
	assert.Equal(t, "tok1", store.m["gitlab-token-p1"], "the legacy key is kept")
	assert.Equal(t, "hub", store.m["gitlab-token"])
	assert.NotContains(t, store.m, "openhub.mcp.jira.token.p1")
	assert.Equal(t, 1, projects.updates, "only the changed project is rewritten")

	// Replayed: nothing left to do.
	n, err = MigrateProjectTokenKeys(ctx, store, projects)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Equal(t, 1, projects.updates)
}

func TestMigrateProjectTokenKeysKeepsAnExistingNewValue(t *testing.T) {
	projects := &memProjects{byID: map[string]*domain.Project{
		"p1": projectWith("p1", domain.ProjectMCPService{Name: "gitlab", TokenKey: "gitlab-token-p1"}),
	}}
	store := newMemStore(map[string]string{"gitlab-token-p1": "old", "openhub.mcp.gitlab.token.p1": "newer"})
	n, err := MigrateProjectTokenKeys(t.Context(), store, projects)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, "newer", store.m["openhub.mcp.gitlab.token.p1"], "never overwritten")
	assert.Empty(t, store.setKeys)
}

// Nothing changes when the keychain cannot be written or read: the legacy
// name stays and the readers fall back to it.
func TestMigrateProjectTokenKeysLeavesTheRecordWhenTheKeychainFails(t *testing.T) {
	newProjects := func() *memProjects {
		return &memProjects{byID: map[string]*domain.Project{
			"p1": projectWith("p1", domain.ProjectMCPService{Name: "gitlab", TokenKey: "gitlab-token-p1"}),
		}}
	}

	projects := newProjects()
	store := newMemStore(map[string]string{"gitlab-token-p1": "tok"})
	store.setErr = errors.New("read-only")
	n, err := MigrateProjectTokenKeys(t.Context(), store, projects)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Equal(t, "gitlab-token-p1", projects.byID["p1"].MCPConfig.Services[0].TokenKey)

	projects = newProjects()
	store = newMemStore(map[string]string{})
	store.getErr = errors.New("locked")
	n, err = MigrateProjectTokenKeys(t.Context(), store, projects)
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Equal(t, "gitlab-token-p1", projects.byID["p1"].MCPConfig.Services[0].TokenKey)

	n, err = MigrateProjectTokenKeys(t.Context(), nil, projects)
	require.NoError(t, err)
	assert.Zero(t, n)
}

func TestProjectTokenKeys(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"gitlab-token-p1", "openhub.mcp.gitlab.token.p1"}, ProjectTokenKeys("gitlab", "p1", "gitlab-token-p1"), "the recorded key is the legacy name")
	assert.Equal(t, []string{"custom", "openhub.mcp.gitlab.token.p1", "gitlab-token-p1"}, ProjectTokenKeys("gitlab", "p1", "custom"))
	assert.Equal(t, []string{"openhub.mcp.gitlab.token.p1", "gitlab-token-p1"}, ProjectTokenKeys("gitlab", "p1", ""))
	assert.Equal(t, []string{"openhub.mcp.gitlab.token.p1", "gitlab-token-p1"}, ProjectTokenKeys("gitlab", "p1", "openhub.mcp.gitlab.token"), "a shared hub key is never listed")
	assert.Equal(t, []string{"openhub.mcp.gitlab.token.p1", "gitlab-token-p1"}, ProjectTokenKeys("gitlab", "p1", "gitlab-token"))
}
