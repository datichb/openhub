package opencodev2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallPluginAndOptions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), PluginDirName)
	b := sampleBundle()
	require.NoError(t, installPlugin(dir, b))
	for _, f := range []string{"package.json", "index.ts", "agents/developer.md"} {
		_, err := os.Stat(filepath.Join(dir, f))
		assert.NoError(t, err, f)
	}
	body, _ := os.ReadFile(filepath.Join(dir, "agents", "developer.md"))
	assert.Equal(t, "You are developer.", string(body))

	withP := withOhPlugin(b, dir, "/tmp/trace")
	require.Len(t, withP.Plugins, 1)
	opts := withP.Plugins[0].Options
	assert.Equal(t, filepath.Join(dir, "agents"), opts["agentsDir"])
	assert.Equal(t, b.AgentIDs(), opts["agents"])
	assert.Equal(t, b.SkillIDs(), opts["skills"])
	assert.Equal(t, "/tmp/trace", opts["traceFile"])
	assert.Empty(t, b.Plugins, "input bundle not mutated")

	cfg, err := BuildConfig(withP, sampleProvider(), DefaultNatives)
	require.NoError(t, err)
	agent := cfg["agents"].(map[string]any)["developer"].(map[string]any)
	assert.NotContains(t, agent, "system", "plugin mode keeps opencode's base prompt")
}

func TestFallbackBundle(t *testing.T) {
	b := sampleBundle()
	b.Plugins = append(b.Plugins, withOhPlugin(b, "/x", "").Plugins...)
	fb := withoutOhPlugin(b)
	assert.Empty(t, fb.Plugins)
	assert.True(t, strings.HasPrefix(fb.Agents[0].Body, "You are working inside an oh session"))
	assert.True(t, strings.HasSuffix(fb.Agents[0].Body, "You are orchestrator-dev."))
	assert.Equal(t, "You are orchestrator-dev.", b.Agents[0].Body, "input bundle not mutated")

	cfg, err := BuildConfig(fb, sampleProvider(), DefaultNatives)
	require.NoError(t, err)
	agent := cfg["agents"].(map[string]any)["orchestrator-dev"].(map[string]any)
	assert.Contains(t, agent["system"], "You are orchestrator-dev.")
}
