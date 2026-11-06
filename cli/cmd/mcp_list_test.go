package cmd

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
)

// oh mcp list shows every oh MCP server (it listed figma, gitlab and
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

// oh mcp setup <service> configures that service (the argument was
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

// oh mcp setup configures every known service, Linear and GitHub included
// (the list was figma, gitlab, gslides, jira), and the one list serves
// the registry of the servers, setup, status and the screens.
func TestMCPSetupOffersEveryKnownService(t *testing.T) {
	assert.ElementsMatch(t, []string{"figma", "github", "gitlab", "gslides", "jira", "linear"}, mcpSetupServices)
	assert.ElementsMatch(t, mcpresolve.Names(), mcpSetupServices)

	for _, name := range []string{"linear", "github", "Linear"} {
		got, err := mcpSetupService([]string{name})
		require.NoError(t, err)
		assert.Equal(t, strings.ToLower(name), got)
	}
	_, err := mcpSetupService([]string{"team"})
	assert.Error(t, err, "the team server has no token to set up")

	t.Setenv("OH_HOME", t.TempDir())
	var servers []string
	for _, n := range validMCPServiceNames() {
		if n != "team" {
			servers = append(servers, n)
		}
	}
	assert.ElementsMatch(t, servers, mcpresolve.Names(), "the services are the servers of the registry (but the team)")

	assert.Equal(t, mcpresolve.Names(), mcpresolveSorted(mcpServiceNames()))

	i18ntest.Set(t, "en")
	for _, name := range mcpSetupServices {
		key := "v55m.mcp.setup.option_" + name
		assert.NotEqual(t, key, i18n.T(key), "label of %s in the menu", name)
	}
}

func mcpresolveSorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
