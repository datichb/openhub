package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

func TestSkillCatalog_Closure(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"skills/a/one.md":   skillMD("one", "requires: [a/two]\n", "one"),
		"skills/a/two.md":   skillMD("two", "", "two"),
		"skills/a/loop1.md": skillMD("loop1", "requires: [a/loop2]\n", "x"),
		"skills/a/loop2.md": skillMD("loop2", "requires: [a/loop1]\n", "x"),
		"skills/a/bad.md":   skillMD("bad", "requires: [a/nope]\n", "x"),
		"skills/b/two.md":   skillMD("two", "", "other two"),
	})
	c := NewSkillCatalog(hub)
	assert.True(t, c.HasSkill("a/one"))
	assert.False(t, c.HasSkill("a/nope"))

	refs, issues := c.Closure([]string{"a/one"})
	assert.Empty(t, issues)
	assert.Equal(t, []string{"a/two", "a/one"}, refs, "requirements first")

	_, issues = c.Closure([]string{"a/one", "b/two", "a/loop1", "a/bad", "a/ghost"})
	kinds := map[string]string{}
	for _, is := range issues {
		kinds[is.Skill] = is.Kind
	}
	assert.Equal(t, map[string]string{"two": "duplicate", "a/loop1": "cycle", "a/bad": "missing", "a/ghost": "missing"}, kinds)
}

func TestSkillCatalog_ValidatesWorkflowSkills(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"agents/solo.md":   agentMD("solo", "", "a/one"),
		"skills/a/one.md":  skillMD("one", "requires: [a/two]\n", "one"),
		"skills/a/two.md":  skillMD("two", "", "two"),
		"skills/a/free.md": skillMD("free", "", "free"),
	})
	cat := workflow.NewMemCatalog()
	doc, diags := workflow.Parse([]byte("apiVersion: oh/v1\nkind: Workflow\nid: x\nrisk: write\nentry: { agent: solo }\nskills: { extra: [a/free], deny: [two] }\n"),
		workflow.Source{Layer: workflow.LayerHub})
	require.False(t, diags.HasErrors())
	cat.Add(doc)
	agents := fakeAgentCatalog{"solo": {Mode: workflow.ModePrimary, Skills: []string{"a/one"}}}
	_, diags = workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "x"}, nil, workflow.Env{Agents: agents, Skills: NewSkillCatalog(hub)})
	assert.Equal(t, []string{"skill_denied_required"}, diags.Codes(), "a denied requirement blocks the build, so validation refuses it")
}

type fakeAgentCatalog map[string]workflow.AgentInfo

func (f fakeAgentCatalog) Agent(id string) (workflow.AgentInfo, bool) {
	a, ok := f[id]
	return a, ok
}

func TestBuildFromWorkflowSpec_AppliesModelsAndSkills(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"agents/solo.md":   agentMD("solo", "", ""),
		"skills/a/free.md": skillMD("free", "", "free"),
	})
	spec := parseSpec(t, "apiVersion: oh/v1\nkind: Workflow\nid: x\nrisk: write\nentry: { agent: solo }\nmodels: { default: amazon-bedrock/eu.anthropic.claude-haiku-4-5-20251001-v1:0 }\nskills: { extra: [a/free] }\n")
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: spec, Provider: "bedrock"})
	require.NoError(t, err)
	solo := findAgent(b.Spec.Agents, "solo")
	require.NotNil(t, solo)
	require.NotNil(t, solo.Model)
	assert.Contains(t, solo.Model.Model, "claude-haiku-4-5", "workflow model level applied")
	var ids []string
	for _, s := range b.Spec.Skills {
		ids = append(ids, s.ID)
	}
	assert.Contains(t, ids, "free", "workflow extra skill shipped")
}
