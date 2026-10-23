package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func workflowMCP(t *testing.T, src string) sessionspec.MCPServerDef {
	t.Helper()
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, src), Provider: "bedrock"})
	require.NoError(t, err)
	for _, m := range b.Spec.MCP {
		if m.Name == sessionspec.WorkflowMCPServer {
			return m
		}
	}
	require.Fail(t, "no workflow MCP server")
	return sessionspec.MCPServerDef{}
}

// Bug 15: the workflow MCP server of a workflow without checkpoint is told
// not to offer workflow_checkpoint; with checkpoints it is unchanged.
func TestWorkflowMCPWithoutCheckpoints(t *testing.T) {
	t.Parallel()
	m := workflowMCP(t, `apiVersion: oh/v1
kind: Workflow
id: review
risk: read
entry: { agent: reviewer }
agents:
  reviewer: { role: workflow, calls: [reviewer] }
`)
	assert.Equal(t, map[string]string{sessionspec.EnvNoCheckpoints: "1"}, m.Environment)

	m = workflowMCP(t, `apiVersion: oh/v1
kind: Workflow
id: one
risk: read
entry: { agent: reviewer }
agents:
  reviewer: { role: workflow, calls: [] }
checkpoints:
  cp-1: { mode: { manuel: pause, semi-auto: pause, auto: pause } }
`)
	assert.Empty(t, m.Environment)
}
