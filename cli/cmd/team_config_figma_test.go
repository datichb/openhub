package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The Figma server always calls api.figma.com, so the wizard no longer
// asks for a URL; a URL already recorded by the team is kept as it is and
// ignored.
func TestConfigureFigmaDoesNotAskForAnURL(t *testing.T) {
	repo, err := teamstate.InitSolo(t.Context(), t.TempDir()+"/state", teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	require.NoError(t, repo.UpdateConfig(t.Context(), func(c *teamstate.TeamConfig) error {
		c.MCP = map[string]teamstate.SharedMCPConfig{"figma": {URL: "https://recorded.example"}}
		return nil
	}))
	teamCfg, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	var out bytes.Buffer
	a := &app.App{Config: &config.Config{}, IO: &app.IOStreams{Out: &out, ErrOut: &out}}
	// the answers: the team uses Figma (choice 2); a second line that no prompt should read
	p := answerPrompts(t, a, "2", "https://figma.example.com")

	require.NoError(t, configureFigma(t.Context(), a, p, repo, teamCfg, true, false))

	assert.NotContains(t, out.String(), "URL", "no prompt for a URL")
	assert.Equal(t, "https://recorded.example", teamCfg.MCP["figma"].URL, "the recorded value is not changed")
	saved, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	require.NotNil(t, saved.MCP["figma"].Enabled)
	assert.True(t, *saved.MCP["figma"].Enabled)
	assert.Equal(t, "https://recorded.example", saved.MCP["figma"].URL)
}

// A Figma URL recorded by the team (even imposed) is ignored without any
// warning: oh mcp trust does not offer to approve it.
func TestRecordedFigmaURLIsIgnoredWithoutWarning(t *testing.T) {
	urlOK := true
	shared := map[string]teamstate.SharedMCPConfig{
		"figma": {URL: "http://figma.example.com", URLEnforced: &urlOK},
	}
	assert.Empty(t, enforcedURLNotices(&config.Config{}, shared), "no notice for the imposed URL of a service without URL")
	_, err := pinService("figma")
	assert.Error(t, err, "nothing to pin for figma")

	var out bytes.Buffer
	s := shared["figma"]
	printMCPStatus(&out, "figma", &s, config.MCPServerConfig{URL: "https://local.example"}, mcpresolve.Resolve(&s, config.MCPServerConfig{}))
	assert.NotContains(t, out.String(), "url:", "the status does not show a URL the server ignores")
	assert.NotContains(t, out.String(), "figma.example.com")
}
