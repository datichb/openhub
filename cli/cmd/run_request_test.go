package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/mcp/gitlab"
	"github.com/datichb/openhub/cli/internal/mcp/team"
)

// The write switch of an MCP server is its own variable, and only
// servers that have write tools get one (GITLAB_WRITE_ENABLED was set for
// every write_enabled service).
func TestSessionMCPEnvWriteSwitch(t *testing.T) {
	cases := map[string]string{
		"gitlab": "GITLAB_WRITE_ENABLED",
		"github": "GITHUB_WRITE_ENABLED",
		"jira":   "JIRA_WRITE_ENABLED",
		"linear": "LINEAR_WRITE_ENABLED",
		"figma":  "",
	}
	for name, want := range cases {
		env := sessionMCPEnv(bricks.MCPServerDef{Name: name, WriteEnabled: true})
		if want == "" {
			if len(env) != 0 {
				t.Errorf("%s: env = %v, want none", name, env)
			}
			continue
		}
		if len(env) != 1 || env[want] != "true" {
			t.Errorf("%s: env = %v, want %s=true only", name, env, want)
		}
		if env := sessionMCPEnv(bricks.MCPServerDef{Name: name}); len(env) != 0 {
			t.Errorf("%s read-only: env = %v", name, env)
		}
	}
}

// The team model recommendations ([models] of the team-state
// config.toml) are a level of the launch cascade (they were never applied).
func TestTeamModelOverrides(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[models]\ndefault = \"claude-sonnet-5-5\"\n[models.agents]\nreviewer = \"claude-opus-4-6\"\n[models.families]\nquality = \"claude-haiku-4-5\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	tc := config.ResolvedTeamConfig{Enabled: true, TeamID: "t", StatePath: dir}
	got := teamModelOverrides(t.Context(), tc)
	if got == nil || got.Default != "claude-sonnet-5-5" || got.Agents["reviewer"] != "claude-opus-4-6" || got.Families["quality"] != "claude-haiku-4-5" {
		t.Fatalf("overrides = %+v", got)
	}
	req := bundle.Request{TeamOverrides: got}
	if m := req.ResolveModel("reviewer", "quality", "claude-haiku-4-5"); m != "claude-opus-4-6" {
		t.Fatalf("reviewer model = %q, want the team recommendation", m)
	}
	req.HubOverrides = &bricks.ModelOverrides{Default: "hub-model"}
	if m := req.ResolveModel("reviewer", "quality", ""); m != "hub-model" {
		t.Fatalf("hub must win over the team recommendation, got %q", m)
	}
	if teamModelOverrides(t.Context(), config.ResolvedTeamConfig{}) != nil {
		t.Fatal("no team: no level")
	}
}

// serverSource is the Go source of an oh MCP server (non-test files).
func serverSource(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join("..", "internal", "mcp", name)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, name)
	var sb strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		sb.Write(data)
	}
	return sb.String()
}

// Every variable a session sets for an oh MCP server is one that server
// reads: GITHUB_URL and LINEAR_URL were set and read by nobody (the servers
// call a fixed address), as FIGMA_URL and GSLIDES_URL could be. The table
// mcpServerEnv declares what each server reads; this test holds the
// session to it, and the table to the source of the servers.
func TestEverySessionVariableIsReadByAServer(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir()) // no custom server
	for _, name := range append(mcpServiceNames(), "team") {
		declared := mcpServerEnv[name]
		require.NotEmpty(t, declared, "%s has no entry in mcpServerEnv", name)

		def := bricks.MCPServerDef{Name: name, Enabled: true, WriteEnabled: true, URL: "https://svc.example"}
		for k := range sessionMCPServerEnv(def, "acme", "p1") {
			assert.Contains(t, declared, k, "%s: %s is set for the session and read by no one", name, k)
		}

		src := serverSource(t, name)
		for _, v := range declared {
			assert.Contains(t, src, `"`+v+`"`, "%s: %s is declared as read, but the server does not mention it", name, v)
		}
	}
}

func TestEveryBuiltInServerIsDeclared(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	for _, name := range validMCPServiceNames() {
		_, ok := mcpServerEnv[name]
		assert.True(t, ok, "the server %s has no entry in mcpServerEnv", name)
	}
}

// The URL of a service is passed only to a server that reads one (GitLab,
// Jira), in the variable that server reads.
func TestSessionMCPEnvURL(t *testing.T) {
	for name, want := range map[string]string{"gitlab": "GITLAB_URL", "jira": "JIRA_URL"} {
		env := sessionMCPEnv(bricks.MCPServerDef{Name: name, URL: "https://svc.example"})
		assert.Equal(t, map[string]string{want: "https://svc.example"}, env, name)
	}
	for _, name := range []string{"github", "linear", "figma", "gslides", "team"} {
		env := sessionMCPEnv(bricks.MCPServerDef{Name: name, URL: "https://svc.example"})
		assert.Empty(t, env, "%s reads no URL: none is set", name)
	}
	env := sessionMCPEnv(bricks.MCPServerDef{Name: "gitlab", Environment: map[string]string{"X": "1"}})
	assert.Equal(t, map[string]string{"X": "1"}, env, "extra variables stay")
}

// The session adds the team and the project to the team server, and the
// project to GitLab, and nothing else.
func TestSessionMCPServerEnvAddsTheSessionVariables(t *testing.T) {
	assert.Equal(t, map[string]string{team.EnvTeamID: "acme", team.EnvProjectID: "p1"},
		sessionMCPServerEnv(bricks.MCPServerDef{Name: "team"}, "acme", "p1"))
	assert.Equal(t, map[string]string{gitlab.EnvOhProject: "p1"},
		sessionMCPServerEnv(bricks.MCPServerDef{Name: "gitlab"}, "acme", "p1"))
	assert.Empty(t, sessionMCPServerEnv(bricks.MCPServerDef{Name: "linear"}, "acme", "p1"))
}
