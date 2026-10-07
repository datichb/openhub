package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datichb/openhub/cli/internal/bricks"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
)

// QB1: the write switch of an MCP server is its own variable, and only
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

// QB2: the team model recommendations ([models] of the team-state
// config.toml) are a level of the launch cascade (they were never applied).
func TestTeamModelOverrides(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[models]\ndefault = \"claude-sonnet-4-6\"\n[models.agents]\nreviewer = \"claude-opus-4-6\"\n[models.families]\nquality = \"claude-haiku-4-5\"\n"
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	tc := config.ResolvedTeamConfig{Enabled: true, TeamID: "t", StatePath: dir}
	got := teamModelOverrides(tc)
	if got == nil || got.Default != "claude-sonnet-4-6" || got.Agents["reviewer"] != "claude-opus-4-6" || got.Families["quality"] != "claude-haiku-4-5" {
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
	if teamModelOverrides(config.ResolvedTeamConfig{}) != nil {
		t.Fatal("no team: no level")
	}
}
