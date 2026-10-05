package opencodev2

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

var update = flag.Bool("update", false, "update golden files")

func sampleBundle() sessionspec.BundleSpec {
	sonnet := sessionspec.ParseModelRef("amazon-bedrock/eu.anthropic.claude-sonnet-4-6")
	return sessionspec.BundleSpec{
		Hash:       "abc123",
		EntryAgent: "orchestrator-dev",
		Agents: []sessionspec.AgentDef{
			{
				ID: "orchestrator-dev", Description: "Drives developers", Mode: "primary", Body: "You are orchestrator-dev.",
				Permissions: []sessionspec.PermissionRule{
					{Action: "shell", Resource: "*", Effect: "deny"},
					{Action: "shell", Resource: "bd show *", Effect: "allow"},
					{Action: "skill", Resource: "*", Effect: "allow"},
					{Action: "subagent", Resource: "*", Effect: "allow"}, // ignored: graph decides
				},
			},
			{
				ID: "developer", Description: "Implements", Mode: "subagent", Body: "You are developer.", Model: &sonnet,
				Permissions: []sessionspec.PermissionRule{
					{Action: "skill", Resource: "*", Effect: "allow"},
					{Action: "skill", Resource: "orchestrator-*", Effect: "deny"},
				},
			},
			{
				ID: "reviewer", Description: "Reviews", Mode: "subagent", Body: "You are reviewer.",
				Permissions: []sessionspec.PermissionRule{{Action: "skill", Resource: "*", Effect: "deny"}},
			},
		},
		Skills: []sessionspec.SkillDef{
			{ID: "beads-dev", Dir: "/b/skills/beads-dev"},
			{ID: "orchestrator-protocol", Dir: "/b/skills/orchestrator-protocol"},
		},
		SkillsDir:     "/b/skills",
		MCP:           []sessionspec.MCPServerDef{{Name: "gitlab", Type: "local", Command: []string{"oh", "mcp", "serve", "gitlab"}}},
		SubagentGraph: map[string][]string{"orchestrator-dev": {"reviewer", "developer", "ghost"}},
		MaxDepth:      2,
		DefaultModel:  &sonnet,
	}
}

func sampleProvider() sessionspec.ProviderSpec {
	return sessionspec.ProviderSpec{ID: "amazon-bedrock", Region: "eu-west-1", BaseURL: "http://127.0.0.1:47000/bedrock", SessionToken: "tok"}
}

func goldenCompare(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, got, 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file (run with -update)")
	assert.JSONEq(t, string(want), string(got))
}

func TestRenderGoldenWithoutPlugin(t *testing.T) {
	rc, err := Render(sampleBundle(), sampleProvider(), DefaultNatives)
	require.NoError(t, err)
	goldenCompare(t, "bundle-system.json", rc.Files[ConfigFileName])
	assert.Equal(t, "tok", rc.Env["AWS_BEARER_TOKEN_BEDROCK"])
	assert.JSONEq(t, string(rc.Files[ConfigFileName]), rc.Env["OPENCODE_CONFIG_CONTENT"])
}

func TestRenderGoldenWithPluginAndCodeMode(t *testing.T) {
	b := sampleBundle()
	b.Plugins = []sessionspec.PluginDef{{ID: OhPluginID, Dir: "/b/plugin"}}
	b.CodeMode = true
	rc, err := Render(b, sampleProvider(), []string{"build", "plan", "general", "explore", "my-global-agent", "title"})
	require.NoError(t, err)
	goldenCompare(t, "bundle-plugin.json", rc.Files[ConfigFileName])
}

func TestRenderSemantics(t *testing.T) {
	cfg, err := BuildConfig(sampleBundle(), sampleProvider(), DefaultNatives)
	require.NoError(t, err)
	raw, _ := json.Marshal(cfg)
	var c struct {
		DefaultAgent string                     `json:"default_agent"`
		Agents       map[string]json.RawMessage `json:"agents"`
		Permissions  []Rule                     `json:"permissions"`
		Experimental struct {
			SubagentDepth int `json:"subagent_depth"`
		} `json:"experimental"`
		Providers map[string]struct {
			Settings map[string]string `json:"settings"`
		} `json:"providers"`
		MCP struct {
			Servers map[string]map[string]any `json:"servers"`
		} `json:"mcp"`
	}
	require.NoError(t, json.Unmarshal(raw, &c))

	assert.Equal(t, "orchestrator-dev", c.DefaultAgent)
	assert.Equal(t, 2, c.Experimental.SubagentDepth)
	assert.Equal(t, "eu-west-1", c.Providers["amazon-bedrock"].Settings["region"])
	assert.Equal(t, false, c.MCP.Servers["gitlab"]["codemode"])
	for _, n := range DefaultNatives {
		assert.JSONEq(t, `{"disabled":true}`, string(c.Agents[n]))
	}

	type agentCfg struct {
		System      string `json:"system"`
		Permissions []Rule `json:"permissions"`
	}
	agent := func(id string) agentCfg {
		var a agentCfg
		require.NoError(t, json.Unmarshal(c.Agents[id], &a))
		return a
	}
	allowed := func(rules []Rule, action, res string) bool {
		eff := "allow"
		for _, r := range rules {
			if wildcardMatch(r.Action, action) && wildcardMatch(r.Resource, res) {
				eff = r.Effect
			}
		}
		return eff == "allow"
	}
	global := c.Permissions
	eval := func(id, action, res string) bool {
		return allowed(append(append([]Rule(nil), global...), agent(id).Permissions...), action, res)
	}

	assert.Equal(t, "You are orchestrator-dev.", agent("orchestrator-dev").System)
	// skills: built-ins hidden, agent filters applied
	assert.False(t, eval("orchestrator-dev", "skill", "opencode"))
	assert.True(t, eval("orchestrator-dev", "skill", "beads-dev"))
	assert.True(t, eval("developer", "skill", "beads-dev"))
	assert.False(t, eval("developer", "skill", "orchestrator-protocol"))
	assert.False(t, eval("reviewer", "skill", "beads-dev"))
	// subagents: graph only, unknown target dropped
	assert.True(t, eval("orchestrator-dev", "subagent", "developer"))
	assert.True(t, eval("orchestrator-dev", "subagent", "reviewer"))
	assert.False(t, eval("orchestrator-dev", "subagent", "ghost"))
	assert.False(t, eval("orchestrator-dev", "subagent", "general"))
	assert.False(t, eval("developer", "subagent", "reviewer"))
	// non-skill agent rules kept
	assert.False(t, eval("orchestrator-dev", "shell", "rm -rf /"))
	assert.True(t, eval("orchestrator-dev", "shell", "bd show bd-1"))
	// code mode off by default
	assert.False(t, allowed(global, "execute", "*"))
}

func TestRenderErrors(t *testing.T) {
	_, err := Render(sessionspec.BundleSpec{}, sessionspec.ProviderSpec{}, nil)
	assert.Error(t, err)
	_, err = Render(sessionspec.BundleSpec{EntryAgent: "x"}, sessionspec.ProviderSpec{}, nil)
	assert.ErrorContains(t, err, "not part of the bundle")
}

func TestWildcardMatch(t *testing.T) {
	cases := []struct {
		p, s string
		ok   bool
	}{
		{"*", "", true}, {"*", "a/b", true}, {"git status *", "git status --short", true},
		{"orchestrator-*", "orchestrator-protocol", true},
		{"a?c", "abc", true}, {"a?c", "ac", false}, {"exact", "exact", true}, {"exact", "exactly", false},
		{"*.env", "x/.env", true}, {"*.env", "x.env.local", false},
	}
	for _, c := range cases {
		assert.Equal(t, c.ok, wildcardMatch(c.p, c.s), "%q vs %q", c.p, c.s)
	}
}

func TestParseVersion(t *testing.T) {
	assert.Equal(t, "2.0.20", parseVersion("opencode v2.0.20\n"))
	assert.Equal(t, "1.17.13", parseVersion("1.17.13"))
}

func TestProviderTokenEnv(t *testing.T) {
	assert.Equal(t, "ANTHROPIC_API_KEY", providerTokenEnv(sessionspec.ProviderSpec{ID: "anthropic"}))
	assert.Equal(t, "X", providerTokenEnv(sessionspec.ProviderSpec{ID: "anthropic", TokenEnv: "X"}))
	assert.Equal(t, "", providerTokenEnv(sessionspec.ProviderSpec{ID: "unknown"}))
}

func TestSkillVisibilityIgnoresAgentAllowOutsideBundle(t *testing.T) {
	b := sampleBundle()
	b.Agents[0].Permissions = append(b.Agents[0].Permissions, sessionspec.PermissionRule{Action: "skill", Resource: "opencode", Effect: "allow"})
	assert.Empty(t, skillVisibleTo(b, "opencode"), "built-in skills stay hidden even if an agent allowed them")
	assert.ElementsMatch(t, []string{"orchestrator-dev", "developer"}, skillVisibleTo(b, "beads-dev"))
}

func TestModelIDBedrockGeoPrefix(t *testing.T) {
	m := sessionspec.ParseModelRef("amazon-bedrock/anthropic.claude-haiku-4-5-20251001-v1:0")
	assert.Equal(t, "amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0", ModelID(m, "eu-west-1"))
	assert.Equal(t, "amazon-bedrock/us.anthropic.claude-haiku-4-5-20251001-v1:0", ModelID(m, "us-east-1"))
	assert.Equal(t, "amazon-bedrock/jp.anthropic.claude-haiku-4-5-20251001-v1:0", ModelID(m, "ap-northeast-1"))
	assert.Equal(t, "amazon-bedrock/anthropic.claude-haiku-4-5-20251001-v1:0", ModelID(m, ""))
	already := sessionspec.ParseModelRef("amazon-bedrock/global.anthropic.claude-haiku-4-5-20251001-v1:0")
	assert.Equal(t, already.String(), ModelID(already, "eu-west-1"))
	other := sessionspec.ParseModelRef("anthropic/claude-sonnet-4-5")
	assert.Equal(t, "anthropic/claude-sonnet-4-5", ModelID(other, "eu-west-1"))
}

func TestProviderPolicy(t *testing.T) {
	cfg, err := BuildConfig(sampleBundle(), sampleProvider(), DefaultNatives)
	require.NoError(t, err)
	exp := cfg["experimental"].(map[string]any)
	assert.Equal(t, []map[string]string{
		{"action": "provider.use", "resource": "*", "effect": "deny"},
		{"action": "provider.use", "resource": "amazon-bedrock", "effect": "allow"},
	}, exp["policies"])
}

// Annex paths in agent bodies are expanded with the bundle root, and every
// agent may read the bundle skill directories without an external_directory prompt.
func TestRenderExpandsBundleRootAndAllowsSkillAnnexes(t *testing.T) {
	b := sampleBundle()
	b.Root = "/b"
	b.Agents[0].Body = "Load `" + sessionspec.BundleRootVar + "/skills/x/templates/t.md`."
	cfg, err := BuildConfig(b, sampleProvider(), DefaultNatives)
	require.NoError(t, err)
	agent := cfg["agents"].(map[string]any)["orchestrator-dev"].(map[string]any)
	assert.Equal(t, "Load `/b/skills/x/templates/t.md`.", agent["system"])
	assert.Contains(t, agent["permissions"], Rule{Action: ActionExternalDir, Resource: "/b/skills/*", Effect: "allow"})

	dir := t.TempDir()
	require.NoError(t, installPlugin(dir, b.WithBundleRoot(b.Root)))
	data, err := os.ReadFile(filepath.Join(dir, "agents", "orchestrator-dev.md"))
	require.NoError(t, err)
	assert.Equal(t, "Load `/b/skills/x/templates/t.md`.", string(data))
}
