package cmd

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/bricks"
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
