package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildInitialConfig_WithProvider(t *testing.T) {
	result := buildInitialConfig("fr", "latest", "bedrock", nil, "")
	assert.Equal(t, "fr", result.CLI.Language)
	assert.Equal(t, "latest", result.Opencode.Version)
	assert.Equal(t, "bedrock", result.Opencode.DefaultProvider)
	assert.Equal(t, "stable", result.Opencode.Channel)
	assert.Equal(t, false, result.Opencode.AutoUpdate)
	assert.Equal(t, true, result.Worktree.AutoCleanup)
	// No MCP selected — all disabled
	assert.Equal(t, false, result.MCP.Figma.Enabled)
	assert.Equal(t, false, result.MCP.Gitlab.Enabled)
	assert.Equal(t, false, result.MCP.Gslides.Enabled)
}

func TestBuildInitialConfig_WithMCP(t *testing.T) {
	result := buildInitialConfig("en", "1.17.15", "anthropic", []string{"figma", "gitlab"}, "")
	assert.Equal(t, "en", result.CLI.Language)
	assert.Equal(t, "1.17.15", result.Opencode.Version)
	assert.Equal(t, "anthropic", result.Opencode.DefaultProvider)
	// Figma and GitLab should be enabled, gslides should not
	assert.Equal(t, true, result.MCP.Figma.Enabled)
	assert.Equal(t, true, result.MCP.Gitlab.Enabled)
	assert.Equal(t, false, result.MCP.Gslides.Enabled)
	// Token keys should be set
	assert.Equal(t, "figma-token", result.MCP.Figma.Token)
	assert.Equal(t, "gitlab-token", result.MCP.Gitlab.Token)
}

func TestBuildInitialConfig_NoMCP(t *testing.T) {
	result := buildInitialConfig("fr", "latest", "openrouter", []string{}, "")
	assert.Equal(t, "openrouter", result.Opencode.DefaultProvider)
	// All MCP should be disabled
	assert.Equal(t, false, result.MCP.Figma.Enabled)
	assert.Equal(t, false, result.MCP.Gitlab.Enabled)
	assert.Equal(t, false, result.MCP.Gslides.Enabled)
}

func TestBuildInitialConfig_NilMCP(t *testing.T) {
	result := buildInitialConfig("en", "latest", "bedrock", nil, "")
	assert.Equal(t, "bedrock", result.Opencode.DefaultProvider)
	// All MCP disabled
	assert.Equal(t, false, result.MCP.Figma.Enabled)
	assert.Equal(t, false, result.MCP.Gitlab.Enabled)
	assert.Equal(t, false, result.MCP.Gslides.Enabled)
	// Token keys should still be populated
	assert.Equal(t, "figma-token", result.MCP.Figma.Token)
	assert.Equal(t, "gitlab-token", result.MCP.Gitlab.Token)
	assert.Equal(t, "gslides-token", result.MCP.Gslides.Token)
}

func TestBuildInitialConfig_BranchPattern(t *testing.T) {
	result := buildInitialConfig("en", "latest", "bedrock", nil, "feat/%s")
	require.NotNil(t, result)
	assert.Equal(t, "feat/%s", result.Worktree.BranchPattern)
}
