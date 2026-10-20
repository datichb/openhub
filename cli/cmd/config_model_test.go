package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

func TestUnsetHubModelRemovesFamilyAndAgent(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
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
