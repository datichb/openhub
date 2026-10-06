package opencodev2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func TestRemoteOhMCP(t *testing.T) {
	b := sessionspec.BundleSpec{MCP: []sessionspec.MCPServerDef{
		sessionspec.WorkflowMCPDef(),
		{Name: "gitlab", Type: "local", Command: []string{"/usr/local/bin/oh", "mcp", "serve", "gitlab", "--token-key", "k"}, Environment: map[string]string{"GITLAB_URL": "https://gl"}},
		{Name: "fs", Type: "local", Command: []string{"npx", "fs-mcp"}},
	}}
	p := sessionspec.ProviderSpec{ID: "amazon-bedrock", BaseURL: "http://host.docker.internal:4242/amazon-bedrock", SessionToken: "ohs_x"}
	out := remoteOhMCP(b, p)
	require.Len(t, out.MCP, 3)
	assert.Equal(t, sessionspec.MCPServerDef{Name: "workflow", Type: "remote", URL: "http://host.docker.internal:4242/oh/v1/hooks/mcp/workflow",
		Headers: map[string]string{"Authorization": "Bearer {env:AWS_BEARER_TOKEN_BEDROCK}"}}, out.MCP[0])
	assert.Equal(t, "http://host.docker.internal:4242/oh/v1/hooks/mcp/gitlab", out.MCP[1].URL)
	assert.Empty(t, out.MCP[1].Environment, "service settings stay on the machine")
	assert.Equal(t, b.MCP[2], out.MCP[2], "other servers run in the runtime")
	assert.Equal(t, "local", b.MCP[0].Type, "the bundle is not modified")

	servers := renderMCP(out)
	gl := servers["gitlab"].(map[string]any)
	assert.Equal(t, "remote", gl["type"])
	assert.NotContains(t, gl, "command")
	assert.Equal(t, map[string]string{"Authorization": "Bearer {env:AWS_BEARER_TOKEN_BEDROCK}"}, gl["headers"])

	// Without a proxy the servers are left as is.
	assert.Equal(t, b.MCP, remoteOhMCP(b, sessionspec.ProviderSpec{}).MCP)
}
