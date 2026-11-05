package config_test

import (
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSave_DoesNotLoseTeams(t *testing.T) {
	tmpDir := configtest.Home(t)
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
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Len(t, cfg.Teams, 1)
	assert.Equal(t, "acme", cfg.Teams[0].ID)

	// Simulate SettingsView: mutate a non-team field and save
	cfg.CLI.Language = "fr"
	require.NoError(t, config.Save(cfg))

	// Reload and verify teams survived
	config.Reset()
	cfg2, err := config.Load()
	require.NoError(t, err)
	require.Len(t, cfg2.Teams, 1, "teams should survive a Save that only changes language")
	assert.Equal(t, "acme", cfg2.Teams[0].ID)
	assert.Equal(t, "fr", cfg2.CLI.Language)
}

func TestSave_KeepsTheDefaultTeam(t *testing.T) {
	tmpDir := configtest.Home(t)
	ohDir := filepath.Join(tmpDir, ".oh")
	require.NoError(t, os.MkdirAll(ohDir, 0o755))
	toml := `[[teams]]
id = "a"
enabled = true
state_repo = "git@gitlab.com:a/ts.git"
member_id = "alice"

[[teams]]
id = "b"
enabled = true
state_repo = "git@gitlab.com:b/ts.git"
member_id = "bob"
default = true
`
	require.NoError(t, os.WriteFile(filepath.Join(ohDir, "hub.toml"), []byte(toml), 0o644))
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "b", cfg.DefaultTeam().ID)

	require.NoError(t, cfg.SetDefaultTeam("a"))
	require.NoError(t, config.Save(cfg))
	config.Reset()
	cfg2, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "a", cfg2.DefaultTeam().ID)
	assert.False(t, cfg2.Teams[1].Default)

	raw, err := os.ReadFile(filepath.Join(ohDir, "hub.toml"))
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(raw), "default = true"))
}
