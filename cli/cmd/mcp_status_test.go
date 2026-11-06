package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// statusApp is an app with the given hub config and a fake keychain.
func statusApp(t *testing.T, secrets map[string]string, edit func(*config.Config)) *app.App {
	t.Helper()
	i18ntest.Set(t, "en")
	isolatedHub(t)
	for _, svc := range mcpresolve.Services() { // no token variable of the machine leaks in
		t.Setenv(svc.TokenEnv, "")
	}
	a := newMockApp(secrets, nil)
	if edit != nil {
		edit(a.Config)
	}
	prev := application
	application = a
	t.Cleanup(func() { application = prev })
	return a
}

func runMCPStatusCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := mcpStatusCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetContext(context.Background())
	if err := cmd.Args(cmd, args); err != nil {
		return out.String(), err
	}
	err := cmd.RunE(cmd, args)
	return out.String(), err
}

// oh mcp status shows every known service (it ignored Jira, GitHub and
// Linear), plus the team.
func TestMCPStatusShowsEveryService(t *testing.T) {
	statusApp(t, nil, nil)
	out, err := runMCPStatusCmd(t)
	require.NoError(t, err)
	for _, label := range []string{"Figma", "GitHub", "GitLab", "Google Slides", "Jira", "Linear", "Team"} {
		assert.Contains(t, out, label)
	}
}

// With a service name, only that service is shown.
func TestMCPStatusOfOneService(t *testing.T) {
	statusApp(t, map[string]string{config.MCPTokenKey("linear"): "x"}, func(c *config.Config) {
		c.MCP.Linear = config.MCPServerConfig{Enabled: true, Token: config.MCPTokenKey("linear")}
	})
	out, err := runMCPStatusCmd(t, " Linear ")
	require.NoError(t, err)
	assert.Contains(t, out, "Linear")
	assert.Contains(t, out, "keychain")
	for _, other := range []string{"Figma", "GitHub", "GitLab", "Google Slides", "Jira", "Team"} {
		assert.NotContains(t, out, other)
	}

	out, err = runMCPStatusCmd(t, "team")
	require.NoError(t, err)
	assert.Contains(t, out, "Team")
	assert.NotContains(t, out, "Linear")
}

// An unknown service is an error that lists the valid ones, with a non-zero
// exit code, and prints no table.
func TestMCPStatusUnknownService(t *testing.T) {
	statusApp(t, nil, nil)
	out, err := runMCPStatusCmd(t, "slack")
	require.Error(t, err)
	assert.NotZero(t, ExitCode(err))
	assert.Contains(t, err.Error(), `"slack"`)
	for _, valid := range []string{"figma", "github", "gitlab", "gslides", "jira", "linear", "team"} {
		assert.Contains(t, err.Error(), valid)
	}
	assert.NotContains(t, out, "SERVICE")

	_, err = runMCPStatusCmd(t, "gitlab", "jira")
	assert.Error(t, err, "one service at most")
}

// The status follows the same resolution as the sessions: a service the team
// enables reaches a member who made no choice, and the source says so.
func TestResolveMCPStatesFollowsTheTeam(t *testing.T) {
	on := true
	shared := map[string]teamstate.SharedMCPConfig{
		"linear": {Enabled: &on},
		"jira":   {Enabled: &on, EnabledEnforced: &on},
	}
	cfg := &config.Config{}
	states := resolveMCPStatesWith(cfg, nil, config.ResolvedTeamConfig{Enabled: true, TeamID: "acme"}, shared)

	byName := map[string]mcpState{}
	for _, st := range states {
		byName[st.Service.Name] = st
	}
	assert.True(t, byName["linear"].Eff.Enabled)
	assert.Equal(t, mcpSourceTeam, byName["linear"].Source)
	assert.True(t, byName["jira"].Eff.EnabledEnforced)
	assert.Equal(t, mcpSourceTeam, byName["jira"].Source)
	assert.False(t, byName["gitlab"].Eff.Enabled)
	assert.Equal(t, mcpSourceHub, byName["gitlab"].Source)

	// A personal choice wins, and a project override is the source.
	cfg.MCP.Linear = config.MCPServerConfig{Token: "k"} // a key without enabled: personal "off"
	states = resolveMCPStatesWith(cfg, &domain.ProjectMCPConfig{Services: []domain.ProjectMCPService{{Name: "gitlab"}}}, config.ResolvedTeamConfig{}, shared)
	for _, st := range states {
		switch st.Service.Name {
		case "linear":
			assert.False(t, st.Eff.Enabled)
			assert.Equal(t, mcpSourceHub, st.Source)
		case "gitlab":
			assert.Equal(t, mcpSourceProject, st.Source)
		}
	}
}
