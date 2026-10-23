package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
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

	_, err = runWorkflowSub(t, configModelAgentCmd, "reviewer", "anthropic/")
	require.Error(t, err, "a model without a name is refused")
	_, err = runWorkflowSub(t, configModelDefaultCmd, "claude haiku")
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

// The short names of the frontmatter (claude-sonnet-5-5) are accepted at every
// level, like anthropic/claude-sonnet-5-5: the session bundle converts both
// for the provider.
func TestConfigModelAcceptsShortNames(t *testing.T) {
	store := modelCLI(t)
	const short = "claude-sonnet-5-5"

	_, err := runWorkflowSub(t, configModelDefaultCmd, short)
	require.NoError(t, err)
	_, err = runWorkflowSub(t, configModelFamilyCmd, "quality", short)
	require.NoError(t, err)
	_, err = runWorkflowSub(t, configModelAgentCmd, "reviewer", short+"#high")
	require.NoError(t, err)
	_, err = runWorkflowSub(t, configModelDefaultCmd, short, "-p", "p1")
	require.NoError(t, err)

	c, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, short, c.Models.Default)
	assert.Equal(t, short, c.Models.Families["quality"])
	assert.Equal(t, short+"#high", c.Models.Agents["reviewer"])
	assert.Equal(t, short, store.projects[0].Model)
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

// An unknown project in oh config model says so, in the language of the hub.
func TestProjectLoadErrorTranslated(t *testing.T) {
	i18ntest.Set(t, "fr")
	err := projectLoadError("nope", fmt.Errorf("project nope: %w", domain.ErrNotFound))
	assert.EqualError(t, err, "projet inconnu : nope (oh project list)")
}
