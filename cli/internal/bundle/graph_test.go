package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

func parseSpec(t *testing.T, src string) *workflow.Spec {
	t.Helper()
	doc, diags := workflow.Parse([]byte(src), workflow.Source{Layer: workflow.LayerHub})
	require.False(t, diags.HasErrors(), "%v", diags)
	return doc.Spec
}

const ticketSpec = `apiVersion: oh/v1
kind: Workflow
id: ticket
risk: write
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow, calls: [developer, reviewer] }
  developer: { role: workflow, calls: [] }
  reviewer: { role: workflow, mode: subagent }
  documentarian: { role: independent }
`

func TestBuildFromWorkflowSpec(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, ticketSpec), Provider: "bedrock"})
	require.NoError(t, err)
	s := b.Spec

	assert.Equal(t, "orchestrator-dev", s.EntryAgent, "entry taken from the workflow")
	assert.Equal(t, []string{"orchestrator-dev", "developer", "reviewer", "documentarian"}, s.AgentIDs(),
		"every workflow member is shipped, independent agents included")
	assert.Equal(t, map[string][]string{
		"orchestrator-dev": {"developer", "reviewer"}, // calls, not the broader task permission
		"reviewer":         {"documentarian"},         // derived from reviewer's task permission
	}, s.SubagentGraph)
	assert.Equal(t, 2, s.MaxDepth)
	assert.Equal(t, "subagent", findAgent(s.Agents, "reviewer").Mode, "workflow mode applied")
	assert.Equal(t, "primary", findAgent(s.Agents, "documentarian").Mode, "agent mode kept")
}

func TestBuildFromWorkflowSpec_Errors(t *testing.T) {
	_, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), EntryAgent: "developer", Spec: parseSpec(t, ticketSpec)})
	assert.ErrorContains(t, err, "differs from the workflow entry")

	_, err = Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, ticketSpec+"  ghost: { role: independent }\n")})
	assert.ErrorContains(t, err, `unknown agent "ghost"`)
}

// The reviewer launches parallel reviewer sessions (standard+adversarial,
// all): its explicit `task: { reviewer: allow }` is kept, in the phase 0
// bundle as in the oh/v1 one (calls: [reviewer]).
func TestReviewerSelfDelegation(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: agentSpec(t, repoHub(t), "reviewer")})
	require.NoError(t, err)
	assert.Contains(t, b.Spec.SubagentGraph["reviewer"], "reviewer")
	assert.GreaterOrEqual(t, b.Spec.MaxDepth, 2)
}

func TestBuildStrictIsolationFromSpec(t *testing.T) {
	out := t.TempDir()
	std, err := Build(Request{HubDir: repoHub(t), OutDir: out, Spec: parseSpec(t, ticketSpec)})
	require.NoError(t, err)
	assert.False(t, std.Spec.StrictIsolation)

	strict, err := Build(Request{HubDir: repoHub(t), OutDir: out, Spec: parseSpec(t, ticketSpec+"isolation: strict\n")})
	require.NoError(t, err)
	assert.True(t, strict.Spec.StrictIsolation)
	assert.NotEqual(t, std.Spec.Hash, strict.Spec.Hash, "the isolation level is part of the bundle hash")
	loaded, err := Load(out, strict.Spec.Hash)
	require.NoError(t, err)
	assert.True(t, loaded.Spec.StrictIsolation, "stored in bundle.json (read by the SessionService)")
}
