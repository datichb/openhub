package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployMCPAllEnabled(t *testing.T) {
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "opencode.json"), []byte(`{"model": "test"}`), 0o644)

	// Set env var for figma
	t.Setenv("FIGMA_TOKEN", "faketoken")
	t.Setenv("GITLAB_TOKEN", "faketoken")

	servers := []MCPServerDef{
		{Name: "figma", Enabled: true, TokenEnv: "FIGMA_TOKEN"},
		{Name: "gitlab", Enabled: true, TokenEnv: "GITLAB_TOKEN"},
	}

	plan := &Plan{ProjectPath: projectDir}
	ctx := &Context{Plan: plan}
	phase := DeployMCP(servers, "oh")

	err := phase.Execute(ctx)
	require.NoError(t, err)

	// Verify opencode.json
	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	// Original key preserved
	assert.Equal(t, "test", config["model"])

	// MCP servers injected
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	require.True(t, ok)
	assert.Len(t, mcpServers, 2)

	figma, ok := mcpServers["figma"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "oh", figma["command"])
	args := figma["args"].([]interface{})
	assert.Equal(t, []interface{}{"mcp", "serve", "figma"}, args)
}

func TestDeployMCPDisabledSkipped(t *testing.T) {
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "opencode.json"), []byte(`{}`), 0o644)

	t.Setenv("FIGMA_TOKEN", "faketoken")

	servers := []MCPServerDef{
		{Name: "figma", Enabled: true, TokenEnv: "FIGMA_TOKEN"},
		{Name: "gitlab", Enabled: false, TokenEnv: "GITLAB_TOKEN"}, // disabled
	}

	plan := &Plan{ProjectPath: projectDir}
	ctx := &Context{Plan: plan}
	phase := DeployMCP(servers, "oh")

	err := phase.Execute(ctx)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	mcpServers := config["mcpServers"].(map[string]interface{})
	assert.Len(t, mcpServers, 1)
	assert.Contains(t, mcpServers, "figma")
	assert.NotContains(t, mcpServers, "gitlab")
}

func TestDeployMCPTokenMissing(t *testing.T) {
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "opencode.json"), []byte(`{}`), 0o644)

	// No FIGMA_TOKEN set, no keychain key
	servers := []MCPServerDef{
		{Name: "figma", Enabled: true, TokenEnv: "FIGMA_TOKEN", TokenKey: ""},
	}

	plan := &Plan{ProjectPath: projectDir}
	ctx := &Context{Plan: plan}
	phase := DeployMCP(servers, "oh")

	err := phase.Execute(ctx)
	require.NoError(t, err) // should not fail, just skip

	// Verify warning was added to context results
	require.Len(t, ctx.Results, 1)
	assert.Contains(t, ctx.Results[0].Message, "skipped")
	assert.Contains(t, ctx.Results[0].Message, "FIGMA_TOKEN")

	// Verify nothing was written to mcpServers
	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))
	assert.Nil(t, config["mcpServers"]) // not present since no MCP deployed
}

func TestDeployMCPPreservesExisting(t *testing.T) {
	projectDir := t.TempDir()
	existing := `{
  "mcpServers": {
    "custom-server": {"command": "node", "args": ["custom.js"]}
  }
}`
	os.WriteFile(filepath.Join(projectDir, "opencode.json"), []byte(existing), 0o644)

	t.Setenv("FIGMA_TOKEN", "faketoken")

	servers := []MCPServerDef{
		{Name: "figma", Enabled: true, TokenEnv: "FIGMA_TOKEN"},
	}

	plan := &Plan{ProjectPath: projectDir}
	ctx := &Context{Plan: plan}
	phase := DeployMCP(servers, "oh")

	err := phase.Execute(ctx)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	mcpServers := config["mcpServers"].(map[string]interface{})
	assert.Len(t, mcpServers, 2)
	assert.Contains(t, mcpServers, "custom-server") // preserved
	assert.Contains(t, mcpServers, "figma")         // added
}

func TestDeployMCPTokenKeychain(t *testing.T) {
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "opencode.json"), []byte(`{}`), 0o644)

	// No env var, but keychain key is configured
	servers := []MCPServerDef{
		{Name: "gitlab", Enabled: true, TokenEnv: "GITLAB_TOKEN", TokenKey: "gitlab-token-default"},
	}

	plan := &Plan{ProjectPath: projectDir}
	ctx := &Context{Plan: plan}
	phase := DeployMCP(servers, "oh")

	err := phase.Execute(ctx)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	mcpServers := config["mcpServers"].(map[string]interface{})
	assert.Contains(t, mcpServers, "gitlab") // keychain key is sufficient
}

func TestDeployMCPNoServersEnabled(t *testing.T) {
	projectDir := t.TempDir()
	os.WriteFile(filepath.Join(projectDir, "opencode.json"), []byte(`{"model": "test"}`), 0o644)

	servers := []MCPServerDef{
		{Name: "figma", Enabled: false},
		{Name: "gitlab", Enabled: false},
	}

	plan := &Plan{ProjectPath: projectDir}
	ctx := &Context{Plan: plan}
	phase := DeployMCP(servers, "oh")

	err := phase.Execute(ctx)
	require.NoError(t, err)

	// File should be unchanged (no write)
	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"model": "test"`)
	assert.NotContains(t, string(data), "mcpServers")
}
