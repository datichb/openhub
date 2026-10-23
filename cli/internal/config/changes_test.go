package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLaunchChangesKeepsTheLaunchSettings(t *testing.T) {
	a, b := defaultConfig(), defaultConfig()
	assert.Empty(t, LaunchChanges(a, b))

	b.Models.Default = "anthropic/claude-sonnet-5-5"
	b.Models.Agents = map[string]string{"reviewer": "claude-opus-4-6"}
	b.Provider.Bedrock.AWSRegion = "eu-west-3"
	b.Deploy.InstructionFiles = []string{"AGENTS.md", "CLAUDE.md"}
	b.CLI.Language = "fr" // not a launch setting
	b.Name = "other"

	got := LaunchChanges(a, b)
	keys := make([]string, len(got))
	for i, c := range got {
		keys[i] = c.Key
	}
	assert.Equal(t, []string{"deploy.instruction_files", "models.agents.reviewer", "models.default", "provider.bedrock.aws_region"}, keys)
	assert.Equal(t, SettingChange{Key: "models.default", Old: "", New: "anthropic/claude-sonnet-5-5"}, got[2])
	assert.Equal(t, "AGENTS.md, CLAUDE.md", got[0].New)

	a.Models.Default = "anthropic/claude-sonnet-4-6"
	got = LaunchChanges(a, b)
	assert.Contains(t, got, SettingChange{Key: "models.default", Old: "anthropic/claude-sonnet-4-6", New: "anthropic/claude-sonnet-5-5"})
}

func TestLaunchChangesTeams(t *testing.T) {
	a, b := defaultConfig(), defaultConfig()
	b.Teams = []TeamConfig{{ID: "t1", StateRepo: "git@host:t1.git"}}
	got := LaunchChanges(a, b)
	require.NotEmpty(t, got)
	assert.Contains(t, got, SettingChange{Key: "teams[0].id", Old: "", New: "t1"})
}

func TestChangedOnDisk(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	Reset()
	t.Cleanup(Reset)
	require.NoError(t, Update(func(c *Config) error { c.Models.Default = "a/b"; return nil }))

	c, err := Read()
	require.NoError(t, err)
	changed, err := c.ChangedOnDisk()
	require.NoError(t, err)
	assert.False(t, changed)

	other, err := Read()
	require.NoError(t, err)
	other.Models.Default = "anthropic/claude-sonnet-5-5"
	require.NoError(t, Save(other))
	changed, err = c.ChangedOnDisk()
	require.NoError(t, err)
	assert.True(t, changed, "written by someone else")
	changed, err = other.ChangedOnDisk()
	require.NoError(t, err)
	assert.False(t, changed, "the writer knows its own write")

	changed, err = (&Config{}).ChangedOnDisk()
	require.NoError(t, err)
	assert.False(t, changed, "built by hand: nothing to compare")

	require.NoError(t, os.Remove(ConfigPath()))
	changed, err = c.ChangedOnDisk()
	require.NoError(t, err)
	assert.False(t, changed, "a removed file is not a change")
}
