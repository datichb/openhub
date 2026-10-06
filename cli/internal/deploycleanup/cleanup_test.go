package deploycleanup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// deployedProject writes a project as left by `oh deploy` (fixture): agents,
// skills, state files with the snapshot of opencode.json taken right after
// the deployment; mutate changes opencode.json afterwards (user edits).
func deployedProject(t *testing.T, mutate func(cfg map[string]any)) string {
	t.Helper()
	dir := t.TempDir()
	cfg, err := os.ReadFile(filepath.Join("testdata", "deployed-opencode.json"))
	require.NoError(t, err)
	var snap map[string]any
	require.NoError(t, json.Unmarshal(cfg, &snap))
	for _, f := range []string{".opencode/agents/developer.md", ".opencode/agents/reviewer.md", ".opencode/skills/x/SKILL.md"} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, f)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644))
	}
	state, _ := json.Marshal(map[string]any{"deployed_at": "2026-09-01T10:00:00Z", "config_snapshot": snap, "selected_agents": []string{"developer", "reviewer"}, "provider": "bedrock"})
	require.NoError(t, os.WriteFile(filepath.Join(dir, DeployStateFile), state, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ManifestFile), []byte(`{"files":{}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, TeamFile), []byte(`{"enabled":true}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".opencode", "package.json"), []byte(`{}`), 0o644)) // not oh's
	if mutate != nil {
		mutate(snap)
		cfg, _ = json.MarshalIndent(snap, "", "  ")
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, ConfigFile), cfg, 0o644))
	return dir
}

// Critère 5: after the cleanup no deployment file is left and the user keys
// of opencode.json are kept, in their order.
func TestCleanupKeepsUserKeys(t *testing.T) {
	dir := deployedProject(t, nil)
	p, err := Scan(dir, Options{AgentIDs: []string{"developer", "reviewer", "orchestrator"}})
	require.NoError(t, err)
	assert.Equal(t, []Item{
		{Rel: AgentsDir, Dir: true, Count: 2}, {Rel: SkillsDir, Dir: true, Count: 1},
		{Rel: DeployStateFile}, {Rel: ManifestFile}, {Rel: TeamFile},
	}, p.Items)
	assert.Equal(t, []string{
		"agent.build", "agent.developer", "agent.plan", "compaction", "enabled_providers",
		"instructions[ONBOARDING.md]", "mcp.gitlab", "permission.webfetch", "permission.websearch",
		"plugin[context-mode]", "provider.amazon-bedrock", "subagent_depth",
	}, p.Removed)
	assert.Empty(t, p.Kept)
	assert.False(t, p.DeleteConfig)
	assert.Contains(t, p.Diff(), `-    "amazon-bedrock": {}`)

	require.NoError(t, p.Apply())
	for _, rel := range []string{AgentsDir, SkillsDir, DeployStateFile, ManifestFile, TeamFile} {
		assert.NoFileExists(t, filepath.Join(dir, rel))
	}
	assert.FileExists(t, filepath.Join(dir, ".opencode", "package.json"), "files oh did not write stay")
	got, err := os.ReadFile(filepath.Join(dir, ConfigFile))
	require.NoError(t, err)
	assert.Equal(t, `{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "litellm": {
      "options": {
        "baseURL": "http://localhost:4000"
      }
    }
  },
  "agent": {
    "mon-agent": {
      "description": "agent personnel",
      "mode": "primary"
    }
  },
  "mcp": {
    "custom": {
      "type": "local",
      "command": [
        "npx",
        "my-mcp"
      ]
    }
  },
  "plugin": [
    "my-plugin"
  ],
  "instructions": [
    "docs/extra.md"
  ]
}
`, string(got))

	again, err := Scan(dir, Options{AgentIDs: []string{"developer"}})
	require.NoError(t, err)
	assert.True(t, again.Empty(), "idempotent")
}

// A key oh wrote and the user changed since is kept (and reported).
func TestCleanupKeepsChangedKeys(t *testing.T) {
	dir := deployedProject(t, func(cfg map[string]any) {
		cfg["compaction"] = map[string]any{"auto": false}
		cfg["agent"].(map[string]any)["developer"] = map[string]any{"mode": "primary", "model": "mine"}
	})
	p, err := Scan(dir, Options{AgentIDs: []string{"developer"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"agent.developer", "compaction"}, p.Kept)
	require.NoError(t, p.Apply())
	data, _ := os.ReadFile(filepath.Join(dir, ConfigFile))
	assert.Contains(t, string(data), `"model": "mine"`)
	assert.Contains(t, string(data), `"compaction"`)
}

// Without .deploy-state, opencode.json is never touched.
func TestCleanupWithoutStateLeavesConfig(t *testing.T) {
	dir := deployedProject(t, nil)
	require.NoError(t, os.Remove(filepath.Join(dir, DeployStateFile)))
	before, _ := os.ReadFile(filepath.Join(dir, ConfigFile))
	p, err := Scan(dir, Options{AgentIDs: []string{"developer"}})
	require.NoError(t, err)
	assert.Equal(t, "no_deploy_state", p.ConfigUntouched)
	assert.Empty(t, p.Removed)
	require.NoError(t, p.Apply())
	after, _ := os.ReadFile(filepath.Join(dir, ConfigFile))
	assert.Equal(t, before, after)
	assert.NoDirExists(t, filepath.Join(dir, AgentsDir))
}

// An opencode.json oh created alone (nothing of the user) is removed.
func TestCleanupDeletesConfigCreatedByOh(t *testing.T) {
	dir := deployedProject(t, nil)
	cfg := `{"$schema": "https://opencode.ai/config.json", "agent": {"build": {"disable": true}}, "subagent_depth": 3}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ConfigFile), []byte(cfg), 0o644))
	var snap map[string]any
	require.NoError(t, json.Unmarshal([]byte(cfg), &snap))
	state, _ := json.Marshal(map[string]any{"config_snapshot": snap})
	require.NoError(t, os.WriteFile(filepath.Join(dir, DeployStateFile), state, 0o644))
	require.NoError(t, os.Remove(filepath.Join(dir, ".opencode", "package.json")))

	p, err := Scan(dir, Options{})
	require.NoError(t, err)
	assert.True(t, p.DeleteConfig)
	require.NoError(t, p.Apply())
	assert.NoFileExists(t, filepath.Join(dir, ConfigFile))
	assert.NoDirExists(t, filepath.Join(dir, ".opencode"), "empty .opencode removed")
}

func TestCleanupCleanProject(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ConfigFile), []byte(`{"model":"x"}`), 0o644))
	p, err := Scan(dir, Options{})
	require.NoError(t, err)
	assert.True(t, p.Empty())
	assert.Empty(t, p.ConfigUntouched)
	assert.False(t, strings.Contains(p.Diff(), "model"))
}

// Only the block of the deployed provider goes: another empty provider
// block is the user's.
func TestCleanupProviderBlocks(t *testing.T) {
	dir := t.TempDir()
	cfg := `{"provider": {"amazon-bedrock": {}, "litellm": {}}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ConfigFile), []byte(cfg), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".opencode"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, DeployStateFile), []byte(`{"provider": "bedrock", "config_snapshot": `+cfg+`}`), 0o644))
	p, err := Scan(dir, Options{})
	require.NoError(t, err)
	assert.Equal(t, []string{"provider.amazon-bedrock"}, p.Removed)

	require.NoError(t, os.WriteFile(filepath.Join(dir, DeployStateFile), []byte(`{"config_snapshot": `+cfg+`}`), 0o644))
	p, err = Scan(dir, Options{})
	require.NoError(t, err)
	assert.Empty(t, p.Removed, "provider unknown: nothing removed")
}
