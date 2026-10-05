package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func TestResolveModelCascade(t *testing.T) {
	full := Request{
		WorkflowModels:   &deploy.ModelOverrides{Default: "wf", Agents: map[string]string{"dev": "wf-agent"}},
		ProjectOverrides: &deploy.ModelOverrides{Default: "proj", Families: map[string]string{"developer": "proj-family"}, Agents: map[string]string{"dev": "proj-agent"}},
		HubOverrides:     &deploy.ModelOverrides{Default: "hub", Families: map[string]string{"developer": "hub-family"}, Agents: map[string]string{"dev": "hub-agent"}},
		TeamOverrides:    &deploy.ModelOverrides{Default: "team", Families: map[string]string{"developer": "team-family"}, Agents: map[string]string{"dev": "team-agent"}},
	}
	// Each step removes the winning level and checks the next one wins.
	steps := []struct {
		want string
		drop func(r *Request)
	}{
		{"wf-agent", func(r *Request) { r.WorkflowModels.Agents = nil }},
		{"wf", func(r *Request) { r.WorkflowModels = nil }},
		{"proj-agent", func(r *Request) { r.ProjectOverrides.Agents = nil }},
		{"proj-family", func(r *Request) { r.ProjectOverrides.Families = nil }},
		{"proj", func(r *Request) { r.ProjectOverrides = nil }},
		{"hub-agent", func(r *Request) { r.HubOverrides.Agents = nil }},
		{"hub-family", func(r *Request) { r.HubOverrides.Families = nil }},
		{"hub", func(r *Request) { r.HubOverrides = nil }},
		{"team-agent", func(r *Request) { r.TeamOverrides.Agents = nil }},
		{"team-family", func(r *Request) { r.TeamOverrides.Families = nil }},
		{"team", func(r *Request) { r.TeamOverrides = nil }},
		{"frontmatter", func(r *Request) {}},
	}
	req := full
	for _, s := range steps {
		assert.Equal(t, s.want, req.ResolveModel("dev", "developer", "frontmatter"))
		s.drop(&req)
	}
	assert.Equal(t, "", req.ResolveModel("dev", "developer", ""))
}

func TestResolveModelWorkflowLevelOtherAgent(t *testing.T) {
	req := Request{
		WorkflowModels: &deploy.ModelOverrides{Agents: map[string]string{"reviewer": "wf-agent"}},
		HubOverrides:   &deploy.ModelOverrides{Default: "hub"},
	}
	assert.Equal(t, "wf-agent", req.ResolveModel("reviewer", "", ""))
	assert.Equal(t, "hub", req.ResolveModel("developer", "", ""), "no workflow default: lower levels apply")
}

func TestResolveModelNormalizesWorkflowRefs(t *testing.T) {
	req := Request{Provider: "bedrock", WorkflowModels: WorkflowModels(&workflow.Models{
		Default: "amazon-bedrock/eu.anthropic.claude-sonnet-4-6",
		Agents:  map[string]string{"reviewer": "amazon-bedrock/eu.anthropic.claude-opus-4-6-v1#high"},
	})}
	assert.Equal(t, "amazon-bedrock/anthropic.claude-sonnet-4-6", req.ResolveModel("developer", "", ""))
	assert.Equal(t, "amazon-bedrock/anthropic.claude-opus-4-6-v1#high", req.ResolveModel("reviewer", "", ""))

	ref := req.modelRef(req.ResolveModel("reviewer", "", ""))
	assert.Equal(t, "amazon-bedrock", ref.Provider)
	assert.Equal(t, "anthropic.claude-opus-4-6-v1", ref.Model)
	assert.Equal(t, "high", ref.Variant)
}

func TestWorkflowModels(t *testing.T) {
	assert.Nil(t, WorkflowModels(nil))
	assert.Nil(t, WorkflowModels(&workflow.Models{}))
	m := map[string]string{"a": "x"}
	got := WorkflowModels(&workflow.Models{Default: "d", Agents: m})
	require.NotNil(t, got)
	assert.Equal(t, "d", got.Default)
	m["a"] = "changed"
	assert.Equal(t, "x", got.Agents["a"], "agents map is copied")
}

func TestBuildAppliesWorkflowModels(t *testing.T) {
	b, err := Build(Request{
		HubDir: repoHub(t), OutDir: t.TempDir(), EntryAgent: "orchestrator-dev", Provider: "bedrock",
		WorkflowModels: WorkflowModels(&workflow.Models{
			Default: "claude-haiku-4-5",
			Agents:  map[string]string{"reviewer": "claude-opus-4-6"},
		}),
		ProjectOverrides: &deploy.ModelOverrides{Agents: map[string]string{"developer": "claude-sonnet-4-5"}},
	})
	require.NoError(t, err)
	dev := findAgent(b.Spec.Agents, "developer")
	require.NotNil(t, dev.Model)
	assert.Contains(t, dev.Model.Model, "claude-haiku-4-5", "workflow default beats project agent override (O9)")
	rev := findAgent(b.Spec.Agents, "reviewer")
	require.NotNil(t, rev.Model)
	assert.Contains(t, rev.Model.Model, "claude-opus-4-6")
	require.NotNil(t, b.Spec.DefaultModel)
	assert.Contains(t, b.Spec.DefaultModel.Model, "claude-haiku-4-5")
}
