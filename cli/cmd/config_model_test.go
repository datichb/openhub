package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

func TestUnsetHubModelRemovesFamilyAndAgent(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	config.Reset()
	t.Cleanup(config.Reset)
	require.NoError(t, config.Update(func(c *config.Config) error {
		c.Models.Default = "anthropic/claude-sonnet-4-5"
		c.Models.Families = map[string]string{"quality": "anthropic/claude-haiku-4-5", "planning": "anthropic/claude-opus-4-1"}
		c.Models.Agents = map[string]string{"reviewer": "anthropic/claude-opus-4-1", "planner": "anthropic/claude-sonnet-4-5"}
		return nil
	}))

	require.NoError(t, unsetHubModel("models.families.quality"))
	require.NoError(t, unsetHubModel("models.agents.reviewer"))

	c, err := config.Load()
	require.NoError(t, err)
	assert.NotContains(t, c.Models.Families, "quality")
	assert.NotContains(t, c.Models.Agents, "reviewer")
	assert.Contains(t, c.Models.Families, "planning", "the other keys stay")
	assert.Contains(t, c.Models.Agents, "planner", "the other keys stay")
	assert.Equal(t, "anthropic/claude-sonnet-4-5", c.Models.Default)

	require.NoError(t, unsetHubModel("models.default"))
	c, err = config.Load()
	require.NoError(t, err)
	assert.Empty(t, c.Models.Default)
}

// modelCLI installs a hub whose catalogue is the repository content, an app
// with one project "web" and returns the output of runWorkflowSub.
func modelCLI(t *testing.T) *mockProjectStore {
	t.Helper()
	root := repoRoot(t)
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	config.Reset()
	t.Cleanup(config.Reset)
	withoutV2(t, nil)
	require.NoError(t, os.Symlink(root, filepath.Join(home, "hub")))
	io, _, _ := testIO()
	store := &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "web", Path: t.TempDir(), Status: domain.ProjectStatusActive}}}
	application = &app.App{Config: &config.Config{}, Projects: store, IO: io}
	t.Cleanup(func() { application = nil })
	return store
}

func TestConfigModelRejectsUnknownIdentifiers(t *testing.T) {
	modelCLI(t)
	const model = "anthropic/claude-haiku-4-5"

	_, err := runWorkflowSub(t, configModelAgentCmd, "reviwer", model)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"reviwer"`)
	assert.Contains(t, err.Error(), "reviewer", "the error proposes the close names")

	_, err = runWorkflowSub(t, configModelFamilyCmd, "qualty", model)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quality")

	_, err = runWorkflowSub(t, configModelAgentCmd, "reviewer", "claude-haiku")
	require.Error(t, err, "a model without provider is refused")
	_, err = runWorkflowSub(t, configModelDefaultCmd, "claude-haiku")
	require.Error(t, err)

	c, err := config.Load()
	require.NoError(t, err)
	assert.Empty(t, c.Models.Agents)
	assert.Empty(t, c.Models.Families)
	assert.Empty(t, c.Models.Default)

	_, err = runWorkflowSub(t, configModelAgentCmd, "reviewer", model)
	require.NoError(t, err)
	_, err = runWorkflowSub(t, configModelFamilyCmd, "quality", model)
	require.NoError(t, err)
	_, err = runWorkflowSub(t, configModelDefaultCmd, model)
	require.NoError(t, err)
	c, err = config.Load()
	require.NoError(t, err)
	assert.Equal(t, model, c.Models.Agents["reviewer"])
	assert.Equal(t, model, c.Models.Families["quality"])
	assert.Equal(t, model, c.Models.Default)
}

func TestConfigModelProjectLevelValidates(t *testing.T) {
	store := modelCLI(t)

	_, err := runWorkflowSub(t, configModelAgentCmd, "nope", "anthropic/claude-haiku-4-5", "-p", "p1")
	require.Error(t, err)
	assert.Nil(t, store.projects[0].ModelOverrides, "nothing written")

	_, err = runWorkflowSub(t, configModelAgentCmd, "reviewer", "anthropic/claude-haiku-4-5", "-p", "p1")
	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-haiku-4-5", store.projects[0].ModelOverrides.Agents["reviewer"])
}
