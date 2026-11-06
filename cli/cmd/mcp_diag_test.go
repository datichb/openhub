package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// A service enabled with no token key and no variable is left out of the
// sessions: status says so on a warning line, never what a token is.
func TestMCPStatusWarnsAboutAServiceWithoutToken(t *testing.T) {
	statusApp(t, nil, func(c *config.Config) {
		c.MCP.Linear.Enabled = true
		c.MCP.Github = config.MCPServerConfig{Enabled: true, Token: config.MCPTokenKey("github")}
	})
	out, err := runMCPStatusCmd(t)
	require.NoError(t, err)
	assert.Contains(t, out, "oh mcp setup linear")
	assert.Contains(t, out, "left out of the sessions")
	assert.Contains(t, out, "LINEAR_API_KEY")
	assert.NotContains(t, out, "oh mcp setup github", "it has a key")
	assert.Contains(t, out, "token missing", "a key whose value is not in the keychain")

	out, err = runMCPStatusCmd(t, "gitlab")
	require.NoError(t, err)
	assert.NotContains(t, out, "left out of the sessions", "nothing to warn about for another service")

	t.Setenv("LINEAR_API_KEY", "from-the-environment")
	out, err = runMCPStatusCmd(t, "linear")
	require.NoError(t, err)
	assert.NotContains(t, out, "oh mcp setup linear", "the variable is a token source")
	assert.Contains(t, out, "env:LINEAR_API_KEY")
}

// A service the team enables, for a member with no token, is reported too.
func TestDiagnosticsFollowTheTeam(t *testing.T) {
	on := true
	shared := map[string]teamstate.SharedMCPConfig{
		"linear": {Enabled: &on},
		"jira":   {Enabled: &on, EnabledEnforced: &on},
	}
	states := resolveMCPStatesWith(&config.Config{}, nil, config.ResolvedTeamConfig{Enabled: true, TeamID: "acme"}, shared)
	t.Setenv("LINEAR_API_KEY", "")
	t.Setenv("JIRA_TOKEN", "")
	diags := mcpDiagnostics(states)
	var names []string
	for _, d := range diags {
		names = append(names, d.Service)
	}
	assert.ElementsMatch(t, []string{"linear", "jira"}, names, "the team enables them, the member has no token")
}
