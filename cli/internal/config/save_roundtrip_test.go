package config

import (
	"os"
	"path/filepath"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSave_DoesNotLoseTeams(t *testing.T) {
	Reset()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))

	// Start with a hub.toml that has teams
	toml := `[cli]
language = "en"

[[teams]]
id = "acme"
name = "Equipe ACME"
enabled = true
state_repo = "git@gitlab.com:acme/ts.git"
member_id = "alice"
`
	require.NoError(t, os.WriteFile(filepath.Join(ohDir, "hub.toml"), []byte(toml), 0o644))

	// Load
	cfg, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg.Teams, 1)
	assert.Equal(t, "acme", cfg.Teams[0].ID)

	// Simulate SettingsView: mutate a non-team field and save
	cfg.CLI.Language = "fr"
	require.NoError(t, Save(cfg))

	// Reload and verify teams survived
	Reset()
	cfg2, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg2.Teams, 1, "teams should survive a Save that only changes language")
	assert.Equal(t, "acme", cfg2.Teams[0].ID)
	assert.Equal(t, "fr", cfg2.CLI.Language)
}
