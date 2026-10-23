package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// withStdin feeds the prompts of the wizard (they read the standard input).
func withStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, err = w.WriteString(input)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	prev := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = prev; _ = r.Close() })
}

// Bug 10: the Figma server always calls api.figma.com, so the wizard no longer
// asks for a URL; a URL already recorded by the team is kept as it is and
// ignored.
func TestConfigureFigmaDoesNotAskForAnURL(t *testing.T) {
	repo, err := teamstate.InitSolo(t.Context(), t.TempDir()+"/state", teamstate.Member{ID: "alice"})
	require.NoError(t, err)
	teamCfg := &teamstate.TeamConfig{MCP: map[string]teamstate.SharedMCPConfig{"figma": {URL: "https://recorded.example"}}}
	// the answers: the team uses Figma; a second line that no prompt should read
	withStdin(t, "y\nhttps://figma.example.com\n")
	var out bytes.Buffer
	a := &app.App{Config: &config.Config{}, IO: &app.IOStreams{Out: &out, ErrOut: &out}}

	require.NoError(t, configureFigma(t.Context(), a, repo, teamCfg, true, false))

	assert.NotContains(t, out.String(), "URL", "no prompt for a URL")
	assert.Equal(t, "https://recorded.example", teamCfg.MCP["figma"].URL, "the recorded value is not changed")
	saved, err := repo.LoadConfig()
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
