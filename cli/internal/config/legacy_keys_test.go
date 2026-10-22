package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FromFile(t *testing.T) {
	tmpDir := configtest.Home(t)

	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))

	tomlContent := `
[cli]
language = "fr"

[opencode]
version = "1.17.2"
channel = "beta"
auto_update = true
`
	require.NoError(t, os.WriteFile(filepath.Join(ohDir, "hub.toml"), []byte(tomlContent), 0o644))

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "fr", cfg.CLI.Language)
	// keys of the former managed install (removed in v5) are ignored
	assert.Empty(t, cfg.LLM.DefaultProvider)
}

// QB7 (D19): the former tool-named keys of hub.toml are still read into
// the neutral settings, and written back under the neutral keys.
func TestLegacyKeysAreRead(t *testing.T) {
	tmpDir := configtest.Home(t)
	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ohDir, "hub.toml"), []byte(`
[opencode]
default_provider = "anthropic"

[execution]
opencode_version = "2.0.19"
`), 0o644))
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "anthropic", cfg.LLM.DefaultProvider)
	assert.Equal(t, "2.0.19", cfg.Execution.ToolVersion)
	assert.Equal(t, "llm.default_provider", config.LegacyKeyAliases["opencode.default_provider"])

	require.NoError(t, config.Save(cfg))
	data, err := os.ReadFile(filepath.Join(ohDir, "hub.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "[llm]")
	assert.Contains(t, string(data), "tool_version = '2.0.19'")
	assert.NotContains(t, string(data), "opencode")
}

// A-08: reading a former key is announced once, with the key that replaces
// it and where it is; the value is read all the same.
func TestLegacyKeysAreAnnounced(t *testing.T) {
	cfg, notes, err := loadHub(t, `
[opencode]
default_provider = "anthropic"

[execution]
opencode_version = "2.0.19"
`)
	require.NoError(t, err)
	assert.Equal(t, "anthropic", cfg.LLM.DefaultProvider)
	assert.Equal(t, "2.0.19", cfg.Execution.ToolVersion)

	require.Len(t, notes, 2)
	assert.Equal(t, config.Notice{Kind: config.NoticeLegacyKey, Key: "opencode.default_provider", Replacement: "llm.default_provider",
		File: config.ConfigPath(), Line: 3, Column: 1}, notes[0])
	assert.Equal(t, config.Notice{Kind: config.NoticeLegacyKey, Key: "execution.opencode_version", Replacement: "execution.tool_version",
		File: config.ConfigPath(), Line: 6, Column: 1}, notes[1])
	assert.Contains(t, notes[0].String(), "support ends in v6")

	config.Reset()
	_, err = config.Load()
	require.NoError(t, err)
	assert.Len(t, notes, 2, "announced once per process")
}

func TestLegacyKeysWithCapitalsAreRead(t *testing.T) {
	cfg, notes, err := loadHub(t, "[OpenCode]\nDefault_Provider = \"anthropic\"\n")
	require.NoError(t, err)
	assert.Equal(t, "anthropic", cfg.LLM.DefaultProvider)
	require.Len(t, notes, 1)
}

// The current key wins; the former one is still announced.
func TestCurrentKeyWinsOverLegacyKey(t *testing.T) {
	cfg, notes, err := loadHub(t, "[llm]\ndefault_provider = \"bedrock\"\n[opencode]\ndefault_provider = \"anthropic\"\n")
	require.NoError(t, err)
	assert.Equal(t, "bedrock", cfg.LLM.DefaultProvider)
	require.Len(t, notes, 1)
}

// Accepted before and still silent: what earlier versions wrote and nothing
// reads any more (managed install of the tool, native agents switch, former
// workflow overrides). Not a warning on every command.
func TestHistoricalKeysAreIgnoredQuietly(t *testing.T) {
	cfg, notes, err := loadHub(t, `
[opencode]
version = "1.17.2"
channel = "beta"
auto_update = true
install_dir = "/x"

[deploy]
disable_native_agents = ["build"]
instruction_files = ["A.md"]

[workflow.overrides]
[[workflow.overrides.checkpoint_overrides]]
id = "cp-1"
action = "modify"
`)
	require.NoError(t, err)
	assert.Equal(t, []string{"A.md"}, cfg.Deploy.InstructionFiles)
	assert.Empty(t, notes)
}

func TestLegacyKeyOfTheWrongTypeIsRefused(t *testing.T) {
	_, _, err := loadHub(t, "[opencode]\ndefault_provider = 1\n")
	var ce *config.ConfigError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, "opencode.default_provider", ce.Key)
	assert.Equal(t, 2, ce.Line)
	assert.Equal(t, "string", ce.Expected)
	assert.Equal(t, "integer", ce.Found)
}

// Saving writes the current keys only: the warning ends with the next write.
func TestSaveDropsTheLegacyKeys(t *testing.T) {
	cfg, _, err := loadHub(t, "[opencode]\ndefault_provider = \"anthropic\"\n")
	require.NoError(t, err)
	require.NoError(t, config.Save(cfg))
	notes, err := config.Inspect()
	require.NoError(t, err)
	assert.Empty(t, notes)
}
