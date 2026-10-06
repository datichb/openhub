package team

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

// chdir changes the working directory for the duration of the test and restores
// it afterwards via t.Cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
}

// useHub writes a hub.toml in a temporary OH_HOME (teams given as TOML).
func useHub(t *testing.T, teams string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	t.Setenv("HOME", home)
	require.NoError(t, os.WriteFile(filepath.Join(home, "hub.toml"), []byte(teams), 0o600))
	config.Reset()
	t.Cleanup(config.Reset)
	resetRepoCache()
}

// useSessionTeam configures the team of the session project as oh does at
// launch (OH_TEAM_ID in the server environment, team in hub.toml).
func useSessionTeam(t *testing.T, tc effectiveTeam) {
	t.Helper()
	useHub(t, fmt.Sprintf("[[teams]]\nid = \"acme\"\nenabled = %t\nstate_repo = %q\nstate_path = %q\nmember_id = %q\n",
		tc.Enabled, tc.StateRepo, tc.StatePath, tc.MemberID))
	t.Setenv(EnvTeamID, "acme")
	t.Setenv(EnvProjectID, "myproject")
}

func TestLoadEffectiveTeamConfig_SessionTeam(t *testing.T) {
	want := effectiveTeam{Enabled: true, StateRepo: "git@gitlab.com:acme/team-state.git",
		StatePath: "/home/alice/.oh/team-states/team-state", MemberID: "alice"}
	useSessionTeam(t, want)

	got, err := loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLoadEffectiveTeamConfig_DisabledTeam(t *testing.T) {
	useSessionTeam(t, effectiveTeam{Enabled: false, StateRepo: "x", StatePath: "/x"})
	got, err := loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.False(t, got.Enabled)
}

func TestLoadEffectiveTeamConfig_UnknownTeam(t *testing.T) {
	useHub(t, "")
	t.Setenv(EnvTeamID, "ghost")
	_, err := loadEffectiveTeamConfig()
	assert.ErrorContains(t, err, "ghost")
}

func TestLoadEffectiveTeamConfig_NoSessionTeam_FallsBackToHub(t *testing.T) {
	useHub(t, "")
	t.Setenv(EnvTeamID, "")
	got, err := loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.False(t, got.Enabled, "hub has no team → Enabled=false")
}

// A leftover .opencode/team.json of a former deploy is ignored.
func TestLoadEffectiveTeamConfig_IgnoresDeployedTeamJSON(t *testing.T) {
	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".opencode"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, ".opencode", "team.json"), []byte(`{"enabled":true,"state_repo":"old"}`), 0o600))
	chdir(t, projectDir)
	useHub(t, "")
	t.Setenv(EnvTeamID, "")
	got, err := loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.False(t, got.Enabled)
}
