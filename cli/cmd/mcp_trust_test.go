package cmd

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func urlEnforced(url string) *teamstate.SharedMCPConfig {
	t := true
	return &teamstate.SharedMCPConfig{URL: url, URLEnforced: &t}
}

// isolatedHub points the hub directory (hub.toml) to a temporary one.
func isolatedHub(t *testing.T) {
	t.Helper()
	t.Setenv("OH_HOME", t.TempDir())
	config.Reset()
	t.Cleanup(config.Reset)
}

func fakeDNS(t *testing.T, ips map[string][]string) {
	t.Helper()
	old := lookupIPs
	t.Cleanup(func() { lookupIPs = old })
	lookupIPs = func(_ context.Context, host string) ([]net.IP, error) {
		list, ok := ips[host]
		if !ok {
			return nil, errors.New("no such host")
		}
		var out []net.IP
		for _, s := range list {
			out = append(out, net.ParseIP(s))
		}
		return out, nil
	}
}

func pinned(t *testing.T, service string) string {
	t.Helper()
	config.Reset()
	c, err := config.Load()
	require.NoError(t, err)
	return c.MCPServer(service).TrustedURL
}

func TestPlanTrust(t *testing.T) {
	fakeDNS(t, map[string][]string{"jira.example.com": {"93.184.216.34"}, "jira.corp.example.com": {"10.1.2.3", "93.184.216.34"}, "meta.example.com": {"169.254.169.254"}, "localhost": {"127.0.0.1"}})
	ctx := context.Background()

	_, err := planTrust(ctx, "jira", nil, config.MCPServerConfig{})
	assert.ErrorContains(t, err, "jira", "no team")
	_, err = planTrust(ctx, "jira", &teamstate.SharedMCPConfig{URL: "https://jira.example.com"}, config.MCPServerConfig{})
	assert.Error(t, err, "a recommendation is not imposed: nothing to approve")
	for _, bad := range []string{"http://jira.example.com", "https://user:pw@jira.example.com", "ftp://jira.example.com"} {
		_, err = planTrust(ctx, "jira", urlEnforced(bad), config.MCPServerConfig{})
		assert.ErrorContains(t, err, bad, "refused, nothing can be pinned")
	}

	p, err := planTrust(ctx, "jira", urlEnforced("https://Jira.Example.com/"), config.MCPServerConfig{})
	require.NoError(t, err)
	assert.Equal(t, "https://jira.example.com", p.URL)
	assert.False(t, p.Sensitive())

	p, err = planTrust(ctx, "jira", urlEnforced("https://jira.corp.example.com"), config.MCPServerConfig{TrustedURL: "https://old.example.com"})
	require.NoError(t, err)
	assert.True(t, p.Sensitive(), "a private address among the answers")
	assert.Equal(t, "https://old.example.com", p.Pinned)

	p, err = planTrust(ctx, "jira", urlEnforced("https://meta.example.com"), config.MCPServerConfig{})
	require.NoError(t, err)
	assert.True(t, p.Sensitive(), "link-local")

	p, err = planTrust(ctx, "jira", urlEnforced("https://unknown.example.com"), config.MCPServerConfig{})
	require.NoError(t, err)
	assert.Error(t, p.DNSErr)

	p, err = planTrust(ctx, "jira", urlEnforced("http://localhost:8080"), config.MCPServerConfig{})
	require.NoError(t, err)
	assert.True(t, p.Sensitive(), "loopback is flagged too")
}

// trustTestCmd is oh mcp trust with its flags parsed, writing to out.
func trustTestCmd(t *testing.T, out *bytes.Buffer, args ...string) *cobra.Command {
	t.Helper()
	cmd := mcpTrustCmd()
	require.NoError(t, cmd.ParseFlags(args))
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetContext(context.Background())
	return cmd
}

func TestTrustURLConfirmsThenPins(t *testing.T) {
	isolatedHub(t)
	i18ntest.Set(t, "en")
	fakeDNS(t, map[string][]string{"jira.corp.example.com": {"10.1.2.3"}})
	shared := urlEnforced("https://Jira.Corp.example.com/")
	terminalIs(t, true)

	// Refused answer: nothing is pinned.
	fa := stubConfirmForm(t, false)
	var out bytes.Buffer
	require.NoError(t, trustURL(trustTestCmd(t, &out), "jira", shared, config.MCPServerConfig{}))
	assert.Empty(t, pinned(t, "jira"))
	assert.Equal(t, []bool{false}, fa.initial, "the answer is no by default")
	assert.Contains(t, fa.questions[0], "token to https://jira.corp.example.com")
	assert.Contains(t, out.String(), "https://jira.corp.example.com")
	assert.Contains(t, out.String(), "10.1.2.3", "the resolution is shown")
	assert.Contains(t, out.String(), "⚠", "a private address is flagged")
	assert.Contains(t, out.String(), "not pinned")

	// Ctrl+C is a no.
	stubConfirmForm(t, huh.ErrUserAborted)
	require.NoError(t, trustURL(trustTestCmd(t, &out), "jira", shared, config.MCPServerConfig{}))
	assert.Empty(t, pinned(t, "jira"))

	// Accepted in the form.
	stubConfirmForm(t, true)
	require.NoError(t, trustURL(trustTestCmd(t, &out), "jira", shared, config.MCPServerConfig{}))
	assert.Equal(t, "https://jira.corp.example.com", pinned(t, "jira"))

	// Already pinned: nothing to confirm.
	stubConfirmForm(t) // any question fails the test
	out.Reset()
	require.NoError(t, trustURL(trustTestCmd(t, &out), "jira", shared, config.MCPServerConfig{TrustedURL: "https://jira.corp.example.com/"}))
	assert.Contains(t, out.String(), "jira")
}

// Without a terminal and without --yes: refused with ExitUsage, never pinned.
func TestTrustURLWithoutTerminalNeedsYes(t *testing.T) {
	isolatedHub(t)
	i18ntest.Set(t, "en")
	fakeDNS(t, map[string][]string{"gitlab.example.com": {"93.184.216.34"}})
	terminalIs(t, false)
	stubConfirmForm(t)
	var out bytes.Buffer
	err := trustURL(trustTestCmd(t, &out), "gitlab", urlEnforced("https://gitlab.example.com"), config.MCPServerConfig{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "--yes")
	assert.Equal(t, ExitUsage, ExitCode(err))
	assert.Empty(t, pinned(t, "gitlab"))

	require.NoError(t, trustURL(trustTestCmd(t, &out, "--yes"), "gitlab", urlEnforced("https://gitlab.example.com"), config.MCPServerConfig{}))
	assert.Equal(t, "https://gitlab.example.com", pinned(t, "gitlab"))
}

func TestTrustURLYesAndOtherServices(t *testing.T) {
	isolatedHub(t)
	fakeDNS(t, map[string][]string{"gitlab.example.com": {"93.184.216.34"}})
	terminalIs(t, false)
	stubConfirmForm(t)
	var out bytes.Buffer
	require.NoError(t, trustURL(trustTestCmd(t, &out, "-y"), "gitlab", urlEnforced("https://gitlab.example.com"), config.MCPServerConfig{}))
	assert.Equal(t, "https://gitlab.example.com", pinned(t, "gitlab"))
	assert.Empty(t, pinned(t, "jira"), "pinned for the service only")
	assert.NotContains(t, out.String(), "⚠")

	// The pin is replaced by another approval.
	require.NoError(t, trustURL(trustTestCmd(t, &out, "--yes"), "gitlab", urlEnforced("https://gitlab2.example.com"), config.MCPServerConfig{TrustedURL: "https://gitlab.example.com"}))
	assert.Equal(t, "https://gitlab2.example.com", pinned(t, "gitlab"))
	assert.Contains(t, out.String(), "https://gitlab.example.com")

	// What the team writes is refused even with --yes.
	err := trustURL(trustTestCmd(t, &out, "--yes"), "gitlab", urlEnforced("http://gitlab3.example.com"), config.MCPServerConfig{})
	require.Error(t, err)
	assert.Equal(t, "https://gitlab2.example.com", pinned(t, "gitlab"))
}

func TestUntrust(t *testing.T) {
	isolatedHub(t)
	require.NoError(t, pinMCPURL("jira", "https://jira.example.com"))
	require.Equal(t, "https://jira.example.com", pinned(t, "jira"))

	cmd := mcpUntrustCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, runMCPUntrust(cmd, []string{"Jira"}))
	assert.Empty(t, pinned(t, "jira"))
	assert.Contains(t, out.String(), "https://jira.example.com")

	out.Reset()
	require.NoError(t, runMCPUntrust(cmd, []string{"jira"}))
	assert.Contains(t, out.String(), "jira", "nothing pinned: said")

	assert.Error(t, runMCPUntrust(cmd, []string{"team"}))
	assert.Error(t, runMCPUntrust(cmd, []string{"slack"}))
}

func TestPinServiceAndRegistration(t *testing.T) {
	for _, s := range []string{"jira", "Gitlab", " figma", "GSLIDES"} {
		got, err := pinService(s)
		require.NoError(t, err, s)
		assert.Equal(t, strings.ToLower(strings.TrimSpace(s)), got)
	}
	for _, s := range []string{"team", "github", "linear", "workflow", ""} {
		_, err := pinService(s)
		assert.Error(t, err, s)
	}
	var names []string
	for _, c := range mcpCmd.Commands() {
		names = append(names, c.Name())
	}
	assert.Contains(t, names, "trust")
	assert.Contains(t, names, "untrust")
}

func TestTrustedURLRoundTripsInHubToml(t *testing.T) {
	isolatedHub(t)
	require.NoError(t, pinMCPURL("jira", "https://jira.example.com/jira"))
	data, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	assert.Contains(t, string(data), "trusted_url = 'https://jira.example.com/jira'")
	assert.Equal(t, "https://jira.example.com/jira", pinned(t, "jira"))
}

func TestEnforcedURLNotices(t *testing.T) {
	cfg := &config.Config{}
	cfg.MCP.Gitlab.TrustedURL = "https://gitlab.example.com"
	cfg.MCP.Jira.TrustedURL = "https://other.example.com"
	shared := map[string]teamstate.SharedMCPConfig{
		"gitlab":  *urlEnforced("https://gitlab.example.com/"),
		"jira":    *urlEnforced("https://jira.example.com"),
		"figma":   *urlEnforced("http://figma.example.com"),
		"gslides": {URL: "https://slides.example.com"}, // a recommendation
	}
	notices := enforcedURLNotices(cfg, shared)
	require.Len(t, notices, 2)
	assert.Equal(t, "figma", notices[0].Service)
	assert.Equal(t, mcpresolve.PendingInvalid, notices[0].Reason)
	assert.Equal(t, "jira", notices[1].Service)
	assert.Equal(t, mcpresolve.PendingPinnedOther, notices[1].Reason)
	assert.Contains(t, notices[1].Message(), "oh mcp trust jira")
	assert.Contains(t, notices[1].Message(), "https://other.example.com")
	assert.Contains(t, notices[0].Message(), "https")

	cfg.MCP.Jira.TrustedURL = ""
	notices = enforcedURLNotices(cfg, shared)
	assert.Equal(t, mcpresolve.PendingNotPinned, notices[1].Reason)
	assert.Contains(t, notices[1].Message(), "oh mcp trust jira")

	var out bytes.Buffer
	printEnforcedURLNotices(&out, nil)
	assert.Empty(t, out.String())
	printEnforcedURLNotices(&out, notices)
	assert.Contains(t, out.String(), "https://jira.example.com")
}

func TestMCPURLDoctorChecks(t *testing.T) {
	cfg := &config.Config{}
	load := func(tc config.ResolvedTeamConfig) map[string]teamstate.SharedMCPConfig {
		if tc.TeamID != "acme" {
			return nil
		}
		return map[string]teamstate.SharedMCPConfig{"jira": *urlEnforced("https://jira.example.com")}
	}
	teams := []config.ResolvedTeamConfig{{TeamID: "acme", Enabled: true}, {TeamID: "acme", Enabled: true}, {TeamID: "other", Enabled: true}, {}}
	checks := mcpURLChecks(cfg, teams, load)
	require.Len(t, checks, 1, "one warning per team and service")
	assert.True(t, checks[0].OK)
	assert.True(t, checks[0].Warn)
	assert.Contains(t, checks[0].Name, "jira")
	assert.Contains(t, checks[0].Detail, "oh mcp trust jira")

	cfg.MCP.Jira.TrustedURL = "https://jira.example.com"
	assert.Empty(t, mcpURLChecks(cfg, teams, load), "pinned: nothing to say")
}

// S-31 end to end: the environment of the Jira MCP server of a session does
// not carry the URL a team imposes until the member pinned it.
func TestEnforcedURLIsNotInTheServerEnvironmentUntilPinned(t *testing.T) {
	state := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(state, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(state, "config.toml"), []byte("[mcp.jira]\nurl = \"https://evil.example.com\"\nurl_enforced = true\n"), 0o644))
	tc := config.ResolvedTeamConfig{Enabled: true, TeamID: "acme", StateRepo: "git@example.com:acme/state.git", StatePath: state}
	cfg := &config.Config{}
	cfg.MCP.Jira = config.MCPServerConfig{Enabled: true, Token: "jira-token", URL: "https://mine.example.com"}
	a := &app.App{Config: cfg}

	jira := func() map[string]string {
		for _, s := range buildMCPServersForProject(a, (*domain.ProjectMCPConfig)(nil), tc) {
			if s.Name == "jira" {
				return sessionMCPEnv(s)
			}
		}
		t.Fatal("no jira server")
		return nil
	}
	assert.Equal(t, "https://mine.example.com", jira()["JIRA_URL"], "the token goes to the personal URL")

	cfg.MCP.Jira.URL = ""
	_, has := jira()["JIRA_URL"]
	assert.False(t, has, "no URL at all rather than the one of the team")

	cfg.MCP.Jira.TrustedURL = "https://evil.example.com/"
	assert.Equal(t, "https://evil.example.com", jira()["JIRA_URL"], "approved by the member")
}

// The MCP view reads the enforced URLs off the event loop while the loop
// replaces the config when it saves: the reader only works on the snapshot
// taken on the loop (run with -race).
func TestTUIEnforcedURLsReadsASnapshot(t *testing.T) {
	statePath := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(statePath, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(statePath, "config.toml"),
		[]byte("[mcp.jira]\nurl = \"https://jira.example.com\"\nurl_enforced = true\n"), 0o644))

	a := &app.App{Config: &config.Config{}}
	a.Config.Team = config.TeamConfig{Enabled: true, StateRepo: "git@example.com:team/state.git", StatePath: statePath}
	a.Config.MCP.Jira.TrustedURL = "https://other.example.com"

	read := tuiEnforcedURLs(a) // on the loop

	done := make(chan struct{})
	go func() {
		// The loop saves the config meanwhile: *a.Config = *newCfg.
		defer close(done)
		for i := 0; i < 200; i++ {
			next := config.Config{}
			next.MCP.Jira.TrustedURL = "https://jira.example.com"
			*a.Config = next
		}
	}()
	var got map[string]*views.MCPEnforcedURL
	for i := 0; i < 50; i++ {
		got = read()
	}
	<-done

	require.Contains(t, got, "jira")
	assert.Contains(t, got["jira"].Warning, "https://other.example.com", "the pin of the snapshot, not the one saved after")
	assert.Equal(t, "https://jira.example.com", got["jira"].URL)
}
