package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	// Reset cached config
	Reset()

	// Point to a temp dir with no config file
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "en", cfg.CLI.Language)
	assert.Equal(t, "stable", cfg.Opencode.Channel)
	assert.Equal(t, false, cfg.Opencode.AutoUpdate)
}

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
	assert.Equal(t, "1.17.2", cfg.Opencode.Version)
	assert.Equal(t, "beta", cfg.Opencode.Channel)
	assert.Equal(t, true, cfg.Opencode.AutoUpdate)
}
