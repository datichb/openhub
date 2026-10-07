package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FromFile(t *testing.T) {
	Reset()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

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

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "fr", cfg.CLI.Language)
	// keys of the former managed install (removed in v5) are ignored
	assert.Empty(t, cfg.LLM.DefaultProvider)
}

// QB7 (D19): the former tool-named keys of hub.toml are still read into
// the neutral settings, and written back under the neutral keys.
func TestLegacyKeysAreRead(t *testing.T) {
	Reset()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ohDir, "hub.toml"), []byte(`
[opencode]
default_provider = "anthropic"

[execution]
opencode_version = "2.0.19"
`), 0o644))
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "anthropic", cfg.LLM.DefaultProvider)
	assert.Equal(t, "2.0.19", cfg.Execution.ToolVersion)
	assert.Equal(t, "llm.default_provider", LegacyKeyAliases["opencode.default_provider"])

	require.NoError(t, Save(cfg))
	data, err := os.ReadFile(filepath.Join(ohDir, "hub.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "[llm]")
	assert.Contains(t, string(data), "tool_version = '2.0.19'")
	assert.NotContains(t, string(data), "opencode")
}
