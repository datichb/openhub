package mcpresolve

import (
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func readTeam(t *testing.T, src string) *teamstate.TeamConfig {
	t.Helper()
	var cfg teamstate.TeamConfig
	require.NoError(t, toml.Unmarshal([]byte(src), &cfg))
	return &cfg
}

func writeTeam(t *testing.T, cfg *teamstate.TeamConfig) string {
	t.Helper()
	out, err := toml.Marshal(cfg)
	require.NoError(t, err)
	return string(out)
}

// A file written before the three states reads the same way: true is
// enabled, false is disabled, an absent key is inherited.
func TestTeamStateOfReadsTheFormerBooleans(t *testing.T) {
	cfg := readTeam(t, `
[mcp.gitlab]
enabled = true
[mcp.jira]
enabled = false
[mcp.figma]
url = "https://x.example"
[mcp.github]
enabled = true
enabled_enforced = true
`)
	assert.Equal(t, TeamEnabled, TeamStateOf(cfg.MCP["gitlab"]))
	assert.Equal(t, TeamDisabled, TeamStateOf(cfg.MCP["jira"]))
	assert.Equal(t, TeamInherited, TeamStateOf(cfg.MCP["figma"]), "an entry without the key")
	assert.Equal(t, TeamInherited, TeamStateOf(cfg.MCP["linear"]), "no entry at all")
	assert.Equal(t, TeamEnabled, TeamStateOf(cfg.MCP["github"]))
}

func TestTeamStateCycle(t *testing.T) {
	st := TeamInherited
	var seen []string
	for range 4 {
		seen = append(seen, st.Value())
		st = st.Next()
	}
	assert.Equal(t, []string{"", "true", "false", ""}, seen, "back to inherited after three steps")
}

func TestParseTeamState(t *testing.T) {
	assert.Equal(t, TeamEnabled, ParseTeamState("true"))
	assert.Equal(t, TeamDisabled, ParseTeamState("false"))
	assert.Equal(t, TeamInherited, ParseTeamState(""))
	assert.Equal(t, TeamInherited, ParseTeamState("maybe"))
}

// The file stays readable by every version: a state is written as the
// boolean it always was, and inherited removes the key.
func TestSetTeamStateWritesTheFormerFormat(t *testing.T) {
	cfg := &teamstate.TeamConfig{}

	SetTeamState(cfg, "linear", TeamEnabled)
	assert.Contains(t, writeTeam(t, cfg), "enabled = true")

	SetTeamState(cfg, "linear", TeamDisabled)
	out := writeTeam(t, cfg)
	assert.Contains(t, out, "enabled = false", "false is written, not dropped")
	again := readTeam(t, out)
	assert.Equal(t, TeamDisabled, TeamStateOf(again.MCP["linear"]))

	SetTeamState(cfg, "linear", TeamInherited)
	out = writeTeam(t, cfg)
	assert.NotContains(t, out, "enabled", "inherited removes the key")
	assert.NotContains(t, out, "linear", "and the empty table with it")
	assert.Empty(t, cfg.MCP)
}

func TestSetTeamStateKeepsTheRestOfTheEntry(t *testing.T) {
	cfg := readTeam(t, `
[mcp.jira]
enabled = true
enabled_enforced = true
url = "https://jira.example.com"
write_recommended = true
`)
	SetTeamState(cfg, "jira", TeamInherited)

	entry, ok := cfg.MCP["jira"]
	require.True(t, ok, "the URL stays: the entry stays")
	assert.Nil(t, entry.Enabled)
	assert.Nil(t, entry.EnabledEnforced, "an enforcement of nothing means nothing")
	assert.Equal(t, "https://jira.example.com", entry.URL)
	assert.True(t, entry.WriteRecommended)
	out := writeTeam(t, cfg)
	assert.NotContains(t, out, "enabled")
	assert.Contains(t, out, "https://jira.example.com")

	SetTeamState(cfg, "jira", TeamEnabled)
	assert.Equal(t, TeamEnabled, TeamStateOf(cfg.MCP["jira"]))
	assert.Equal(t, "https://jira.example.com", cfg.MCP["jira"].URL)
}

func TestSetTeamStateOnANilConfigOrMap(t *testing.T) {
	SetTeamState(nil, "linear", TeamEnabled) // no panic

	cfg := &teamstate.TeamConfig{}
	SetTeamState(cfg, "github", TeamInherited)
	assert.Empty(t, cfg.MCP, "nothing to remove, nothing created")

	SetTeamState(cfg, "github", TeamEnabled)
	assert.Equal(t, TeamEnabled, TeamStateOf(cfg.MCP["github"]))
}

// Inherited, written then read back by the patch the save uses, removes the
// key from the file the other members share.
func TestInheritedGoesThroughTheConfigPatch(t *testing.T) {
	base := readTeam(t, "[mcp.github]\nenabled = false\n[mcp.gitlab]\nenabled = true\nurl = \"https://g.example\"\n")
	edited := teamstate.CloneConfig(base)
	SetTeamState(edited, "github", TeamInherited)
	SetTeamState(edited, "gitlab", TeamInherited)

	patch := teamstate.DiffConfig(base, edited)
	assert.Equal(t, []string{"mcp.github", "mcp.gitlab.enabled"}, patch.Keys())

	onDisk := teamstate.CloneConfig(base)
	require.NoError(t, patch.Apply(onDisk))
	out := writeTeam(t, onDisk)
	assert.NotContains(t, out, "github")
	assert.NotContains(t, out, "enabled")
	assert.Contains(t, out, "https://g.example")
}

// The order of precedence of the Enabled setting, the same for every service:
// team enforced, project, personal (hub.toml, explicit), team recommended,
// off.
func TestEnabledPrecedence(t *testing.T) {
	enforced := func(on bool) *teamstate.SharedMCPConfig {
		return &teamstate.SharedMCPConfig{Enabled: flag(on), EnabledEnforced: flag(true)}
	}
	recommend := func(st TeamState) *teamstate.SharedMCPConfig {
		cfg := &teamstate.TeamConfig{}
		SetTeamState(cfg, "linear", st)
		s := cfg.MCP["linear"]
		return &s
	}
	hubOff := config.MCPServerConfig{}
	hubOn := config.MCPServerConfig{Enabled: true, Token: "k"}
	hubTokenOnly := config.MCPServerConfig{Token: "k"} // a key is an explicit choice: off
	projOn := &domain.ProjectMCPService{Enabled: flag(true)}
	projOff := &domain.ProjectMCPService{Enabled: flag(false)}

	tests := []struct {
		name    string
		shared  *teamstate.SharedMCPConfig
		hub     config.MCPServerConfig
		project *domain.ProjectMCPService
		want    bool
	}{
		{"nothing at all: off", nil, hubOff, nil, false},
		{"inherited: off for a member with no choice", recommend(TeamInherited), hubOff, nil, false},
		{"team enabled reaches a member with no choice", recommend(TeamEnabled), hubOff, nil, true},
		{"team disabled: off", recommend(TeamDisabled), hubOff, nil, false},
		{"inherited never changes a personal choice", recommend(TeamInherited), hubOn, nil, true},
		{"personal on beats team disabled", recommend(TeamDisabled), hubOn, nil, true},
		{"personal off (key set) beats team enabled", recommend(TeamEnabled), hubTokenOnly, nil, false},
		{"project off beats personal on", recommend(TeamInherited), hubOn, projOff, false},
		{"project on beats personal key without enabled", recommend(TeamDisabled), hubTokenOnly, projOn, true},
		{"project on beats team disabled", recommend(TeamDisabled), hubOff, projOn, true},
		{"enforced on beats project off", enforced(true), hubTokenOnly, projOff, true},
		{"enforced off beats project on", enforced(false), hubOn, projOn, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveFull(tc.shared, tc.hub, tc.project, "acme")
			assert.Equal(t, tc.want, got.Enabled)
			if tc.project == nil {
				assert.Equal(t, tc.want, Resolve(tc.shared, tc.hub).Enabled, "Resolve (two levels) agrees")
			}
		})
	}

	// an enforcement written without an opinion (enabled absent) binds nobody
	orphan := &teamstate.SharedMCPConfig{EnabledEnforced: flag(true)}
	assert.True(t, ResolveFull(orphan, hubOn, nil, "acme").Enabled)
	assert.False(t, ResolveFull(orphan, hubOn, nil, "acme").EnabledEnforced)
}

// oh team config set|unset mcp.<service>.enabled goes through the same
// three states: set writes the boolean, unset goes back to inherited.
func TestTeamConfigSettingsUseTheThreeStates(t *testing.T) {
	cfg := &teamstate.TeamConfig{}
	require.NoError(t, cfg.Set("mcp.linear.enabled", "false"))
	assert.Equal(t, TeamDisabled, TeamStateOf(cfg.MCP["linear"]))
	require.NoError(t, cfg.Set("mcp.linear.enabled", "true"))
	assert.Equal(t, TeamEnabled, TeamStateOf(cfg.MCP["linear"]))

	require.NoError(t, cfg.Unset("mcp.linear.enabled"))
	assert.Equal(t, TeamInherited, TeamStateOf(cfg.MCP["linear"]))
	assert.NotContains(t, writeTeam(t, cfg), "enabled", "the key is gone from the file")
}
