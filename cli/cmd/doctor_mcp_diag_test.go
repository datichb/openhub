package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

// oh doctor warns (never fails) about a service that a project enables with
// no token: the sessions leave it out, which nothing said.
func TestDoctorWarnsAboutAProjectServiceWithoutToken(t *testing.T) {
	a := statusApp(t, nil, func(c *config.Config) {
		c.MCP.Gitlab = config.MCPServerConfig{Enabled: true} // hub-level: the API keys check says it
	})
	a.Projects = &mockProjectStore{projects: []domain.Project{
		{ID: "p1", Name: "demo", Status: domain.ProjectStatusActive, MCPConfig: &domain.ProjectMCPConfig{Services: []domain.ProjectMCPService{
			{Name: "linear", Enabled: boolPtr(true)},
			{Name: "jira", Enabled: boolPtr(true), TokenKey: "openhub.mcp.jira.token.p1"},
		}}},
	}}

	results := mcpDiagnosticChecks(t.Context(), a)

	require.Len(t, results, 1, "linear only: jira has a key, gitlab is a hub service")
	r := results[0]
	assert.True(t, r.OK, "a warning, not a failure")
	assert.True(t, r.Warn)
	assert.Contains(t, r.Name, "linear")
	assert.Contains(t, r.Detail, "demo")
	assert.Contains(t, r.Detail, "LINEAR_API_KEY")
	assert.Contains(t, r.Detail, "oh mcp setup linear")
}

func TestDoctorSaysNothingWhenEveryEnabledServiceHasAToken(t *testing.T) {
	a := statusApp(t, nil, nil)
	a.Projects = &mockProjectStore{projects: []domain.Project{
		{ID: "p1", Name: "demo", Status: domain.ProjectStatusActive, MCPConfig: &domain.ProjectMCPConfig{Services: []domain.ProjectMCPService{
			{Name: "linear", Enabled: boolPtr(true)},
		}}},
	}}
	t.Setenv("LINEAR_API_KEY", "from-the-environment")
	assert.Empty(t, mcpDiagnosticChecks(t.Context(), a))
}
