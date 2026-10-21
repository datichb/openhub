package bundle

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// LD-J: a workflow bundle for a project with a wiki gets the wiki MCP
// server; every agent reads and proposes, only the documentarian applies
// (asked), and the session contract says so.
func TestWikiMCPInBundle(t *testing.T) {
	libre := workflow.Ref{Layer: workflow.LayerHub, ID: "libre"}
	_, b := shippedBuild(t, sharedInstructionsFixture(t, true), libre)
	spec := b.Spec
	assert.True(t, slices.ContainsFunc(spec.MCP, func(m sessionspec.MCPServerDef) bool { return m.Name == sessionspec.WikiMCPServer }))
	apply := sessionspec.MCPToolAction(sessionspec.WikiMCPServer, sessionspec.WikiToolApply)
	propose := sessionspec.MCPToolAction(sessionspec.WikiMCPServer, sessionspec.WikiToolPropose)
	assert.Equal(t, sessionspec.EffectDeny, ToolEffect(spec.Permissions, apply, "*"), "refused in the bundle")
	assert.Equal(t, sessionspec.EffectAllow, ToolEffect(spec.Permissions, propose, "*"))
	seen := 0
	for _, a := range spec.Agents {
		eff := ToolEffect(append(slices.Clone(spec.Permissions), a.Permissions...), apply, "*")
		contract := a.Body[strings.Index(a.Body, contractHeading):]
		assert.Contains(t, contract, "`wiki_propose` met une découverte en file", a.ID)
		if a.ID == sessionspec.WikiApplier {
			assert.Equal(t, sessionspec.EffectAsk, eff, "the documentarian applies, asked to the user")
			assert.Contains(t, contract, "`wiki_apply` applique des propositions", a.ID)
			seen++
			continue
		}
		assert.Equal(t, sessionspec.EffectDeny, eff, a.ID)
		assert.NotContains(t, contract, "wiki_apply", a.ID)
	}
	assert.Equal(t, 1, seen, "feature ships the documentarian")

	// Without docs/wiki/: no wiki server.
	_, b = shippedBuild(t, sharedInstructionsFixture(t, false), libre)
	assert.False(t, slices.ContainsFunc(b.Spec.MCP, func(m sessionspec.MCPServerDef) bool { return m.Name == sessionspec.WikiMCPServer }))
	for _, a := range b.Spec.Agents {
		assert.NotContains(t, a.Body, "wiki_propose` met", a.ID)
	}
}

// p3-remote-wiki: the bundle of a remote session is the one built on the
// machine (the job runs it as is): every shipped workflow that runs
// remotely gets the wiki MCP server with the same tools and rules as in a
// local session; the job daemon serves them on its clone.
func TestWikiMCPInRemoteWorkflows(t *testing.T) {
	_, cat, env := shippedWorkflows(t)
	project := sharedInstructionsFixture(t, true)
	propose := sessionspec.MCPToolAction(sessionspec.WikiMCPServer, sessionspec.WikiToolPropose)
	n := 0
	for _, ref := range cat.Refs() {
		r, diags := workflow.Check(cat, ref, nil, env)
		require.False(t, diags.HasErrors(), "%v", diags)
		if !r.Spec.AllowsRuntime(workflow.RuntimeRemote) {
			continue
		}
		n++
		_, b := shippedBuild(t, project, ref)
		i := slices.IndexFunc(b.Spec.MCP, func(m sessionspec.MCPServerDef) bool { return m.Name == sessionspec.WikiMCPServer })
		require.GreaterOrEqual(t, i, 0, "%s: wiki MCP server", ref.ID)
		assert.Equal(t, sessionspec.WikiMCPDef(), b.Spec.MCP[i], ref.ID)
		assert.Equal(t, sessionspec.EffectAllow, ToolEffect(b.Spec.Permissions, propose, "*"), ref.ID)
	}
	assert.GreaterOrEqual(t, n, 3, "audit, review, ticket… run remotely")
}
