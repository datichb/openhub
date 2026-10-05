package opencodev2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func TestInnerBundleTranslatesPaths(t *testing.T) {
	m := ohruntime.PathMap{{Host: "/h/bundle", Inner: "/opt/oh/bundle"}, {Host: "/h/data", Inner: "/opt/oh/data"}}
	b := sessionspec.BundleSpec{
		Root: "/h/bundle", SkillsDir: "/h/bundle/skills",
		Skills:  []sessionspec.SkillDef{{ID: "a", Dir: "/h/bundle/skills/a"}},
		Plugins: []sessionspec.PluginDef{{ID: OhPluginID, Dir: "/h/data/oh-plugin", Options: map[string]any{"agentsDir": "/h/data/oh-plugin/agents", "agents": []string{"x"}, "rel": "not/abs"}}},
	}
	out, err := innerBundle(b, m)
	require.NoError(t, err)
	assert.Equal(t, "/opt/oh/bundle", out.Root)
	assert.Equal(t, "/opt/oh/bundle/skills", out.SkillsDir)
	assert.Equal(t, "/opt/oh/bundle/skills/a", out.Skills[0].Dir)
	assert.Equal(t, "/opt/oh/data/oh-plugin", out.Plugins[0].Dir)
	assert.Equal(t, "/opt/oh/data/oh-plugin/agents", out.Plugins[0].Options["agentsDir"])
	assert.Equal(t, "not/abs", out.Plugins[0].Options["rel"])
	assert.Equal(t, "/h/bundle/skills/a", b.Skills[0].Dir, "the original bundle is not modified")
	assert.Equal(t, "/h/data/oh-plugin/agents", b.Plugins[0].Options["agentsDir"])

	b.SkillsDir = "/elsewhere/skills"
	_, err = innerBundle(b, m)
	assert.ErrorContains(t, err, "not visible")
}

func TestRuntimeEnvIsComplete(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "leak")
	t.Setenv("PATH", "/machine/bin")
	env := runtimeEnv(ServerOptions{ConfigContent: "{\n  \"a\": 1\n}", Env: map[string]string{"AWS_BEARER_TOKEN_BEDROCK": "ohs_x"}}, "pw", "/opt/oh/data")
	assert.Equal(t, map[string]string{
		"OPENCODE_SERVER_PASSWORD":        "pw",
		"OPENCODE_DISABLE_PROJECT_CONFIG": "1",
		"OPENCODE_DISABLE_AUTOUPDATE":     "true",
		"XDG_DATA_HOME":                   "/opt/oh/data",
		"OPENCODE_CONFIG_CONTENT":         `{"a":1}`,
		"AWS_BEARER_TOKEN_BEDROCK":        "ohs_x",
	}, env, "no machine variable is inherited; config compacted to one line")
}

func TestStartServerExpandsBundleRootInRuntimeView(t *testing.T) {
	g := adaptersGroup("/h/bundle", ohruntime.PathMap{{Host: "/h/bundle", Inner: "/opt/oh/bundle"}})
	assert.Equal(t, "/opt/oh/bundle", innerDir(g, "/h/bundle"))
	b := sessionspec.BundleSpec{Root: "/h/bundle", Agents: []sessionspec.AgentDef{{ID: "a", Body: "see " + sessionspec.BundleRootVar + "/skills/x/annex.md"}}}
	assert.Equal(t, "see /opt/oh/bundle/skills/x/annex.md", b.WithBundleRoot(innerDir(g, b.Root)).Agents[0].Body)
	assert.Equal(t, "/h/bundle", innerDir(adaptersGroup("/h/bundle", nil), "/h/bundle"), "local: machine path")
}

func adaptersGroup(root string, m ohruntime.PathMap) adapters.ServerGroup {
	g := adapters.ServerGroup{Bundle: sessionspec.BundleSpec{Root: root}}
	if m != nil {
		g.Prepared = &ohruntime.Prepared{Paths: m}
	}
	return g
}
