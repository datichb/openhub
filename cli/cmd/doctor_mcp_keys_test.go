package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

func enabledGitlab(token string) *config.Config {
	cfg := &config.Config{}
	cfg.MCP.Gitlab.Enabled = true
	cfg.MCP.Gitlab.Token = token
	return cfg
}

func TestDoctorReportsAnEmptyOrMissingTokenKey(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	store := &mockSecretStore{secrets: map[string]string{}}

	detail, ok := mcpTokensReport(enabledGitlab("openhub.mcp.gitlab.token"), store)
	assert.False(t, ok)
	assert.Contains(t, detail, "openhub.mcp.gitlab.token", "the key is named")
	assert.Contains(t, detail, "oh mcp setup gitlab", "the command to redo it is given")

	store.secrets["openhub.mcp.gitlab.token"] = ""
	_, ok = mcpTokensReport(enabledGitlab("openhub.mcp.gitlab.token"), store)
	assert.False(t, ok, "an empty value is a missing token")

	detail, ok = mcpTokensReport(enabledGitlab(""), nil)
	assert.False(t, ok, "no secret store: nothing found")
	assert.Contains(t, detail, "openhub.mcp.gitlab.token", "no key configured: the one setup writes is named")
}

func TestDoctorAcceptsAFoundToken(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	store := &mockSecretStore{secrets: map[string]string{"openhub.mcp.gitlab.token": "x"}}
	detail, ok := mcpTokensReport(enabledGitlab("openhub.mcp.gitlab.token"), store)
	assert.True(t, ok)
	assert.Contains(t, detail, "gitlab (keychain)")

	// Only the legacy name holds the value: found, and copied.
	t.Run("legacy fallback", func(t *testing.T) {
		store := &mockSecretStore{secrets: map[string]string{"gitlab-token": "x"}}
		_, ok := mcpTokensReport(enabledGitlab("openhub.mcp.gitlab.token"), store)
		assert.True(t, ok)
		assert.Equal(t, "x", store.secrets["openhub.mcp.gitlab.token"])
		assert.Equal(t, "x", store.secrets["gitlab-token"])
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("GITLAB_TOKEN", "from-env")
		detail, ok := mcpTokensReport(enabledGitlab("k"), &mockSecretStore{secrets: map[string]string{}})
		assert.True(t, ok)
		assert.Contains(t, detail, "gitlab (env)")
	})

	t.Run("nothing enabled", func(t *testing.T) {
		_, ok := mcpTokensReport(&config.Config{}, nil)
		assert.True(t, ok)
	})
}

// The setup writes under the current name; status, serve and the
// diagnostic read it back.
func TestMCPSetupTokenIsReadBack(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	store := &mockSecretStore{secrets: map[string]string{}}
	key, err := config.StoreMCPToken(t.Context(), store, "gitlab", "", "tok")
	assert.NoError(t, err)
	assert.Equal(t, config.MCPTokenKey("gitlab"), key)

	require.NoError(t, injectTokenFromStore(store, "gitlab", key))
	assert.Equal(t, "tok", os.Getenv("GITLAB_TOKEN"))

	// A hub config still pointing to the legacy name finds the value too.
	t.Setenv("GITLAB_TOKEN", "")
	require.NoError(t, injectTokenFromStore(store, "gitlab", "gitlab-token"))
	assert.Equal(t, "tok", os.Getenv("GITLAB_TOKEN"))

	t.Setenv("GITLAB_TOKEN", "")
	assert.Error(t, injectTokenFromStore(&mockSecretStore{secrets: map[string]string{}}, "gitlab", key), "an empty key is reported")
}
