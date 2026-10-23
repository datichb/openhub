package opencodev2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

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

func TestRenderGoldenWithoutPlugin(t *testing.T) {
	rc, err := Render(sampleBundle(), sampleProvider(), DefaultNatives)
	require.NoError(t, err)
	golden.Assert(t, "golden/bundle-system.json", rc.Files[ConfigFileName])
	assert.Equal(t, "tok", rc.Env["AWS_BEARER_TOKEN_BEDROCK"])
	assert.JSONEq(t, string(rc.Files[ConfigFileName]), rc.Env["OPENCODE_CONFIG_CONTENT"])
}

func TestRenderGoldenWithPluginAndCodeMode(t *testing.T) {
	b := sampleBundle()
	b.Plugins = []sessionspec.PluginDef{{ID: OhPluginID, Dir: "/b/plugin"}}
	b.CodeMode = true
	rc, err := Render(b, sampleProvider(), []string{"build", "plan", "general", "explore", "my-global-agent", "title"})
	require.NoError(t, err)
	golden.Assert(t, "golden/bundle-plugin.json", rc.Files[ConfigFileName])
}

// P1-T15: plugins declared by the workflow (npm specs, with or without
// options) are rendered next to the oh plugin.
func TestRenderGoldenWorkflowPlugins(t *testing.T) {
	b := sampleBundle()
	b.Plugins = []sessionspec.PluginDef{
		{ID: "context-mode", Dir: "context-mode@latest"},
		{ID: "@acme/probe", Dir: "@acme/probe", Options: map[string]any{"verbose": true}},
		{ID: OhPluginID, Dir: "/b/plugin"},
	}
	rc, err := Render(b, sampleProvider(), DefaultNatives)
	require.NoError(t, err)
	golden.Assert(t, "golden/bundle-workflow-plugins.json", rc.Files[ConfigFileName])
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

// L2: an agent with a catalogue sees only its skills, its own rules still
// filter them; an agent without catalogue (older bundle) sees every bundle
// skill its rules allow.
func TestRenderAgentCatalogue(t *testing.T) {
	b := sampleBundle()
	b.Agents[0].Skills = []string{"orchestrator-protocol"} // orchestrator-dev
	b.Agents[1].Skills = []string{}                        // developer
	assert.Equal(t, []string{"orchestrator-dev"}, skillVisibleTo(b, "orchestrator-protocol"))
	assert.Empty(t, skillVisibleTo(b, "beads-dev"), "in no catalogue")

	b.Agents[1].Skills = []string{"beads-dev", "orchestrator-protocol"}
	assert.Equal(t, []string{"developer"}, skillVisibleTo(b, "beads-dev"))
	assert.Equal(t, []string{"orchestrator-dev"}, skillVisibleTo(b, "orchestrator-protocol"), "the developer's own rules deny orchestrator-*")

	rules := agentRules(b.Agents[0], b, b.SkillIDs())
	var skills []string
	for _, r := range rules {
		if r.Action == sessionspec.ActionSkill {
			skills = append(skills, r.Resource+" "+string(r.Effect))
		}
	}
	assert.Equal(t, []string{"* deny", "orchestrator-protocol allow"}, skills)

	b.Agents[1].Skills = nil
	assert.Equal(t, []string{"developer"}, skillVisibleTo(b, "beads-dev"), "nil: every bundle skill")
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

func TestRenderMCPToolActions(t *testing.T) {
	b := sampleBundle()
	cp := sessionspec.MCPToolAction(sessionspec.WorkflowMCPServer, sessionspec.WorkflowToolCheckpoint)
	b.Permissions = append(b.Permissions, sessionspec.PermissionRule{Action: cp, Resource: "*", Effect: sessionspec.EffectAsk})
	b.MCP = []sessionspec.MCPServerDef{sessionspec.WorkflowMCPDef()}
	cfg, err := BuildConfig(b, sampleProvider(), DefaultNatives)
	require.NoError(t, err)
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	assert.Contains(t, string(data), `{"action":"workflow_checkpoint","resource":"*","effect":"ask"}`)
	assert.Contains(t, string(data), `"command":["{{oh.bin}}","mcp","serve","workflow"]`, "expanded when the server starts, not in the rendered bundle")

	assert.Equal(t, "shell", ToolAction("shell"))
	assert.Equal(t, "gitlab_issues", ToolAction(sessionspec.MCPToolAction("gitlab", "issues")))
}

// N2 (v5 critical): driving the tool server from the shell is refused to
// every agent, even one allowed to run anything.
func TestControlPlaneGuardLastOfEveryAgent(t *testing.T) {
	b := sessionspec.BundleSpec{EntryAgent: "a", Agents: []sessionspec.AgentDef{{ID: "a",
		Permissions: []sessionspec.PermissionRule{{Action: sessionspec.ActionShell, Resource: "*", Effect: sessionspec.EffectAllow}}}}}
	rules := append(agentRules(b.Agents[0], b, nil), AgentGuard(b)...)
	for _, line := range []string{
		"printenv OPENCODE_SERVER_PASSWORD",
		"echo $OPENCODE_CONFIG_CONTENT",
		"curl -u opencode:x http://127.0.0.1:4096/api/session/ses_1/permission/per_1/reply -d '{\"decision\":\"once\"}'",
		"curl http://127.0.0.1:4096/api/permission/request",
		"curl -X PUT http://127.0.0.1:4096/api/session/ses_1/environment",
	} {
		assert.Equal(t, sessionspec.EffectDeny, bundle.ShellEffect(rules, line), line)
	}
	assert.Equal(t, sessionspec.EffectAllow, bundle.ShellEffect(rules, "go test ./internal/adapters/opencodev2/..."), "developing oh itself stays possible")

	// Rendered once in the plugin options, per agent without the plugin.
	b.AgentGuard = []sessionspec.PermissionRule{{Action: sessionspec.ActionShell, Resource: "*g*", Effect: sessionspec.EffectDeny}}
	withP := withOhPlugin(b, "/p", "", nil, nil)
	cfg, err := BuildConfig(withP, sessionspec.ProviderSpec{}, nil)
	require.NoError(t, err)
	perms := cfg["agents"].(map[string]any)["a"].(map[string]any)["permissions"].([]Rule)
	assert.NotContains(t, perms, Rule{Action: "shell", Resource: "*g*", Effect: "deny"})
	opts := withP.Plugins[len(withP.Plugins)-1].Options
	assert.Contains(t, opts["guard"], Rule{Action: "shell", Resource: "*g*", Effect: "deny"})
	cfg, err = BuildConfig(withoutOhPlugin(b), sessionspec.ProviderSpec{}, nil)
	require.NoError(t, err)
	perms = cfg["agents"].(map[string]any)["a"].(map[string]any)["permissions"].([]Rule)
	assert.Equal(t, Rule{Action: "shell", Resource: "*--user opencode:*", Effect: "deny"}, perms[len(perms)-1], "fallback: last of the agent")
	assert.Contains(t, perms, Rule{Action: "shell", Resource: "*g*", Effect: "deny"})

}

// N23 (v5 critical, audit S-10): the bundle is readable from the agents
// (skill annexes) but never editable.
func TestBundleReadOnlyForAgents(t *testing.T) {
	b := sessionspec.BundleSpec{EntryAgent: "a", Root: "/oh/bundles/h", SkillsDir: "/oh/bundles/h/skills",
		Agents: []sessionspec.AgentDef{{ID: "a", Permissions: []sessionspec.PermissionRule{{Action: sessionspec.ActionEdit, Resource: "*", Effect: sessionspec.EffectAllow}}}}}
	rules := agentRules(b.Agents[0], b, nil)
	assert.Equal(t, sessionspec.EffectAllow, bundle.ToolEffect(rules, ActionExternalDir, "/oh/bundles/h/skills/x/annex.md"))
	assert.Equal(t, sessionspec.EffectDeny, bundle.ToolEffect(rules, sessionspec.ActionEdit, "/oh/bundles/h/skills/x/SKILL.md"))
	assert.Equal(t, sessionspec.EffectDeny, bundle.ToolEffect(rules, sessionspec.ActionEdit, "/oh/bundles/h/agents/a.md"))
	assert.Equal(t, sessionspec.EffectAllow, bundle.ToolEffect(rules, sessionspec.ActionEdit, "src/a.go"))
}

// opencode 2.x reads `mode` as subagent | primary | all (Config.Agent) and
// only refuses to launch a `primary` agent through task: `all` (a primary
// agent that a workflow agent may call, reviewer → reviewer) is rendered as
// written.
func TestRenderAgentModeAll(t *testing.T) {
	b := sampleBundle()
	b.Agents = append(b.Agents, sessionspec.AgentDef{ID: "reviewer-all", Description: "Reviews", Mode: "all", Body: "You are reviewer."})
	cfg, err := BuildConfig(b, sampleProvider(), DefaultNatives)
	require.NoError(t, err)
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	var c struct {
		Agents map[string]struct {
			Mode string `json:"mode"`
		} `json:"agents"`
	}
	require.NoError(t, json.Unmarshal(raw, &c))
	assert.Equal(t, "all", c.Agents["reviewer-all"].Mode)
	assert.Equal(t, "primary", c.Agents["orchestrator-dev"].Mode)
	assert.Equal(t, "subagent", c.Agents["developer"].Mode)
}
