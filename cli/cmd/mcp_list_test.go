package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// QB2: oh mcp list shows every oh MCP server (it listed figma, gitlab and
// gslides only), with a description.
func TestMCPServerListHasEveryServer(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir()) // no custom server
	var names []string
	for _, s := range mcpServerList() {
		names = append(names, s.Name)
		assert.NotEmpty(t, s.Description, s.Name)
		assert.NotContains(t, s.Description, "cmd.mcp.list.", "translated")
		assert.Equal(t, "oh mcp serve "+s.Name, s.Command)
	}
	assert.Equal(t, []string{"figma", "github", "gitlab", "gslides", "jira", "linear", "team"}, names)
}

// QB2: oh mcp setup <service> configures that service (the argument was
// ignored and a list always shown); an unknown service is refused.
func TestMCPSetupServiceArgument(t *testing.T) {
	name, err := mcpSetupService([]string{"Jira"})
	require.NoError(t, err)
	assert.Equal(t, "jira", name)
	name, err = mcpSetupService(nil)
	require.NoError(t, err)
	assert.Empty(t, name, "no argument: chosen from a list")
	_, err = mcpSetupService([]string{"slack"})
	assert.ErrorContains(t, err, "slack")
	for _, s := range mcpSetupServices {
		assert.NotEmpty(t, mcpServiceEnvVar[s], "token variable of %s", s)
	}
}
