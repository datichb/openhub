package team

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/deploy"
)

// chdir changes the working directory for the duration of the test and restores
// it afterwards via t.Cleanup. This is required because loadEffectiveTeamConfig
// calls os.Getwd() to locate .opencode/team.json.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
}

func writeTeamJSON(t *testing.T, projectDir string, tc deploy.DeployedTeamConfig) {
	t.Helper()
	opencodeDir := filepath.Join(projectDir, ".opencode")
	require.NoError(t, os.MkdirAll(opencodeDir, 0o755))
	data, err := json.MarshalIndent(tc, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(opencodeDir, deploy.TeamConfigFile),
		data, 0o600,
	))
}

func TestLoadEffectiveTeamConfig_WithTeamJSON(t *testing.T) {
	projectDir := t.TempDir()

	want := deploy.DeployedTeamConfig{
		Enabled:   true,
		StateRepo: "git@gitlab.com:acme/team-state.git",
		StatePath: "/home/alice/.oh/team-states/team-state",
		MemberID:  "alice",
	}
	writeTeamJSON(t, projectDir, want)
	chdir(t, projectDir)

	got, err := loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.True(t, got.Enabled)
	assert.Equal(t, want.StateRepo, got.StateRepo)
	assert.Equal(t, want.StatePath, got.StatePath)
	assert.Equal(t, want.MemberID, got.MemberID)
}

func TestLoadEffectiveTeamConfig_DisabledTeamJSON(t *testing.T) {
	projectDir := t.TempDir()
	writeTeamJSON(t, projectDir, deploy.DeployedTeamConfig{Enabled: false})
	chdir(t, projectDir)

	got, err := loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.False(t, got.Enabled, "enabled=false in team.json should be respected")
}

func TestLoadEffectiveTeamConfig_InvalidJSON_FallsBackToHub(t *testing.T) {
	projectDir := t.TempDir()

	// Write invalid JSON in .opencode/team.json
	opencodeDir := filepath.Join(projectDir, ".opencode")
	require.NoError(t, os.MkdirAll(opencodeDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(opencodeDir, deploy.TeamConfigFile),
		[]byte(`{not valid json`), 0o600,
	))
	chdir(t, projectDir)

	// Point hub config to a temp dir so config.Load() doesn't fail
	t.Setenv("HOME", t.TempDir())

	// Invalid JSON → fallback to hub.toml (hub has no team → Enabled=false)
	got, err := loadEffectiveTeamConfig()
	require.NoError(t, err, "invalid team.json should not error — fallback to hub")
	assert.False(t, got.Enabled, "hub has no team → fallback should return Enabled=false")
}

func TestLoadEffectiveTeamConfig_NoFile_FallsBackToHub(t *testing.T) {
	projectDir := t.TempDir()
	// No .opencode/ directory at all
	chdir(t, projectDir)

	// Hub has no team configured
	t.Setenv("HOME", t.TempDir())

	got, err := loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.False(t, got.Enabled, "no team.json + hub has no team → Enabled=false")
}
