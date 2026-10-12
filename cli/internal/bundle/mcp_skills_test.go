package bundle

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// A tracker adapter (`mcp: gitlab`, `mcp: github`) is inlined only when the
// session has that MCP server: without it, the agent gets no instructions
// for tools it does not have.
func TestTrackerAdaptersFollowSessionMCP(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "cadrage"}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	body := func(mcp []sessionspec.MCPServerDef) map[string]string {
		b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, MCP: mcp})
		require.NoError(t, err)
		out := map[string]string{}
		for _, a := range b.Spec.Agents {
			out[a.ID] = a.Body
		}
		return out
	}
	const gitlab, github = "GitLab Planner Protocol", "GitHub Planner Protocol"

	none := body(nil)
	assert.NotContains(t, none["planner"], gitlab)
	assert.NotContains(t, none["planner"], github)

	withGitlab := body([]sessionspec.MCPServerDef{{Name: "gitlab", Type: "local", Command: []string{"oh", "mcp", "serve", "gitlab"}}})
	assert.Contains(t, withGitlab["planner"], gitlab)
	assert.NotContains(t, withGitlab["planner"], github)
	assert.True(t, strings.Contains(withGitlab["pathfinder"], "GitLab Pathfinder Protocol"), "pathfinder gets its GitLab adapter")

	withGithub := body([]sessionspec.MCPServerDef{{Name: "github", Type: "local", Command: []string{"oh", "mcp", "serve", "github"}}})
	assert.Contains(t, withGithub["planner"], github)
	assert.NotContains(t, withGithub["planner"], gitlab)
}

func TestRootFilterMCP(t *testing.T) {
	hub := repoHub(t)
	l := newSkillLoader(hub, nil)
	keep := rootFilter(l, nil, []sessionspec.MCPServerDef{{Name: "gitlab"}}, "")
	assert.True(t, keep("adapters/gitlab-planner-protocol"))
	assert.False(t, keep("adapters/github-planner-protocol"))
	assert.True(t, keep("planning/planner-workflow"), "a skill without mcp: is kept")
}
