package team

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
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

// useHub writes a hub.toml in a temporary hub (teams given as TOML).
func useHub(t *testing.T, teams string) {
	t.Helper()
	configtest.Home(t)
	require.NoError(t, os.MkdirAll(config.HubDir(), 0o755))
	require.NoError(t, os.WriteFile(config.ConfigPath(), []byte(teams), 0o600))
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

// Outside of a session (no OH_TEAM_ID) the server takes the default team by
// the rule of the commands: the one marked `default = true`, else the first
// enabled one; an archived team or a solo space is never taken.
func TestLoadEffectiveTeamConfig_NoSessionTeam_TakesTheDefaultTeam(t *testing.T) {
	const teams = `
[[teams]]
id = "old"
enabled = false
state_repo = "git@x:o/old.git"
member_id = "me-old"

[[teams]]
id = "mine"
enabled = true
solo = true
state_path = "/solo"
member_id = "me-solo"

[[teams]]
id = "first"
enabled = true
state_repo = "git@x:o/first.git"
member_id = "me-first"

[[teams]]
id = "chosen"
enabled = true
default = true
state_repo = "git@x:o/chosen.git"
member_id = "me-chosen"
`
	useHub(t, teams)
	t.Setenv(EnvTeamID, "")
	got, err := loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.Equal(t, "me-chosen", got.MemberID, "the default team")
	assert.True(t, got.Enabled)

	// With no `default = true`, the first enabled team that is neither
	// archived nor a solo space.
	useHub(t, strings.Replace(teams, "default = true", "", 1))
	t.Setenv(EnvTeamID, "")
	got, err = loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.Equal(t, "me-first", got.MemberID)

	// The team of the session project is the one OH_TEAM_ID names, an
	// archived one included (the project says so), but its features stay off.
	t.Setenv(EnvTeamID, "old")
	got, err = loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.Equal(t, "me-old", got.MemberID)
	assert.False(t, got.Enabled)

	t.Setenv(EnvTeamID, "mine")
	got, err = loadEffectiveTeamConfig()
	require.NoError(t, err)
	assert.False(t, got.Enabled, "a solo space never enables the team features")
}
