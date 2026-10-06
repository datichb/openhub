package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

const checkpointSpec = `apiVersion: oh/v1
kind: Workflow
id: ticket
risk: write
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow, calls: [developer, reviewer] }
  developer: { role: workflow, after: cp-1, calls: [] }
  reviewer: { role: workflow, after: developer, mode: subagent }
checkpoints:
  cp-1:
    label: { fr: Démarrer, en: Start }
    mode: { manuel: pause, semi-auto: auto }
  cp-2:
    label: Commit
    mandatory: true
    mode: { manuel: pause, semi-auto: skip }
modes: { default: semi-auto, allowed: [manuel, semi-auto] }
circuit_breaker: { max_consecutive_subagents: 12 }
outputs:
  - { id: branch, type: branch }
`

func TestBuildInjectsWorkflowRuntime(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, checkpointSpec), Provider: "bedrock"})
	require.NoError(t, err)

	require.Len(t, b.Spec.MCP, 1)
	assert.Equal(t, sessionspec.WorkflowMCPDef(), b.Spec.MCP[0], "workflow MCP server with the unexpanded oh binary")

	wf := b.Spec.Workflow
	require.NotNil(t, wf)
	assert.Equal(t, "ticket", wf.ID)
	require.Len(t, wf.Checkpoints, 2)
	assert.Equal(t, "cp-1", wf.Checkpoints[0].ID, "declaration order")
	assert.Equal(t, map[string]string{"manuel": "pause", "semi-auto": "auto"}, wf.Checkpoints[0].Behaviors)
	assert.Equal(t, "Start", wf.Checkpoints[0].LabelFor("en"))
	assert.Equal(t, "Commit", wf.Checkpoints[1].LabelFor("fr"))
	assert.True(t, wf.Checkpoints[1].Mandatory)
	assert.Equal(t, "pause", wf.Checkpoints[1].Behavior("semi-auto"), "a mandatory checkpoint is never skipped")
	assert.Equal(t, []sessionspec.AgentGate{{Agent: "developer", After: "cp-1"}, {Agent: "reviewer", After: "developer"}}, wf.Gates)
	assert.Equal(t, 12, wf.MaxConsecutiveSubagents)
	assert.Equal(t, []sessionspec.OutputDef{{ID: "branch", Type: "branch"}}, wf.Outputs)

	loaded, err := Load(b.Dir[:len(b.Dir)-len(b.Spec.Hash)-1], b.Spec.Hash)
	require.NoError(t, err)
	assert.Equal(t, wf, loaded.Spec.Workflow, "persisted in bundle.json")
}

func TestBuildWithoutSpecHasNoWorkflowRuntime(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), EntryAgent: "developer", Provider: "bedrock"})
	require.NoError(t, err)
	assert.Nil(t, b.Spec.Workflow)
	assert.Empty(t, b.Spec.MCP)
}
