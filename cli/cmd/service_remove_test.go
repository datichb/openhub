package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/config"
)

func TestRemoveServiceTokensDeletesCurrentAndLegacyKeys(t *testing.T) {
	store := &mockSecretStore{secrets: map[string]string{
		"openhub.mcp.gitlab.token":   "current",
		"gitlab-token":               "legacy",
		"gitlab-token-p1":            "project", // project-scoped key: not a hub key
		"openhub.mcp.figma.token":    "other service",
		"openhub.provider.bedrock.t": "unrelated",
	}}

	removeServiceTokens(t.Context(), store, &config.Config{}, "gitlab")

	assert.NotContains(t, store.secrets, "openhub.mcp.gitlab.token", "the current key goes")
	assert.NotContains(t, store.secrets, "gitlab-token", "the legacy key goes too")
	assert.Contains(t, store.secrets, "gitlab-token-p1")
	assert.Contains(t, store.secrets, "openhub.mcp.figma.token")
	assert.Contains(t, store.secrets, "openhub.provider.bedrock.t")
}

func TestRemoveServiceTokensFollowsConfiguredKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCP.Gslides.Token = "my-slides-key"
	store := &mockSecretStore{secrets: map[string]string{"my-slides-key": "x", "openhub.mcp.gslides.token": "y"}}

	removeServiceTokens(t.Context(), store, cfg, "gslides")

	assert.Empty(t, store.secrets)
	removeServiceTokens(t.Context(), nil, cfg, "gslides") // no secret store: no-op
}
