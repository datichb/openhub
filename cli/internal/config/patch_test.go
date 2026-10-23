package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

func TestPatch_OnlyTheChangedSettingsAreCopied(t *testing.T) {
	before := config.MCPConfig{Gitlab: config.MCPServerConfig{Enabled: false, Token: "k"}}
	after := before
	after.Gitlab.Enabled = true
	after.Jira.URL = "https://jira.example"

	// what the file holds now: the member trusted a URL in the meantime
	disk := config.MCPConfig{Gitlab: config.MCPServerConfig{Token: "k", TrustedURL: "https://gl.example"}}
	config.Patch(before, after)(&disk)

	assert.True(t, disk.Gitlab.Enabled)
	assert.Equal(t, "https://gl.example", disk.Gitlab.TrustedURL, "an untouched setting keeps the file's value")
	assert.Equal(t, "https://jira.example", disk.Jira.URL)
	assert.Equal(t, "k", disk.Gitlab.Token)
}

func TestPatch_NoChangeIsANoOp(t *testing.T) {
	c := config.Config{Name: "a"}
	disk := config.Config{Name: "b", CLI: config.CLIConfig{SetupDone: true}}
	config.Patch(c, c)(&disk)
	assert.Equal(t, "b", disk.Name)
	assert.True(t, disk.CLI.SetupDone)
}

func TestPatch_ResetToTheZeroValueIsAChange(t *testing.T) {
	before := config.TrackerLocalConfig{TrackerTokenKey: "key"}
	after := config.TrackerLocalConfig{}
	disk := config.TrackerLocalConfig{TrackerTokenKey: "key", TrackerURL: "https://t"}
	config.Patch(before, after)(&disk)
	assert.Empty(t, disk.TrackerTokenKey)
	assert.Equal(t, "https://t", disk.TrackerURL)
}

func TestPatch_PointersAndListsDoNotAlias(t *testing.T) {
	yes := true
	before := config.Config{}
	after := config.Config{}
	after.Tracker.Enabled = &yes
	after.Deploy.InstructionFiles = []string{"a.md"}

	var disk config.Config
	patch := config.Patch(before, after)
	patch(&disk)
	require.NotNil(t, disk.Tracker.Enabled)
	assert.True(t, *disk.Tracker.Enabled)
	yes = false
	*after.Tracker.Enabled = false
	after.Deploy.InstructionFiles[0] = "changed"
	assert.True(t, *disk.Tracker.Enabled, "the patch copied the pointer's value when it was built")
	assert.Equal(t, []string{"a.md"}, disk.Deploy.InstructionFiles)

	var other config.Config
	patch(&other)
	assert.True(t, *other.Tracker.Enabled)
	assert.NotSame(t, disk.Tracker.Enabled, other.Tracker.Enabled)
}

func TestPatch_AWholeConfig(t *testing.T) {
	before := config.Config{Name: "x"}
	after := config.Config{Name: "x"}
	after.Models.Families = map[string]string{"fast": "m1"}
	after.Worktree.BranchPattern = "feat/*"

	disk := config.Config{Name: "x"}
	disk.CLI.SetupDone = true
	disk.LLM.DefaultProvider = "bedrock"
	config.Patch(before, after)(&disk)

	assert.Equal(t, "m1", disk.Models.Families["fast"])
	assert.Equal(t, "feat/*", disk.Worktree.BranchPattern)
	assert.True(t, disk.CLI.SetupDone)
	assert.Equal(t, "bedrock", disk.LLM.DefaultProvider)
}
