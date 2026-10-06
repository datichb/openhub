package sessionspec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMCPToolAction(t *testing.T) {
	a := MCPToolAction(WorkflowMCPServer, WorkflowToolCheckpoint)
	assert.Equal(t, "mcp:workflow/workflow_checkpoint", a)
	s, tool, ok := ParseMCPToolAction(a)
	assert.True(t, ok)
	assert.Equal(t, "workflow", s)
	assert.Equal(t, "workflow_checkpoint", tool)
	for _, bad := range []string{"shell", "mcp:", "mcp:x", "mcp:/t", "mcp:s/"} {
		_, _, ok := ParseMCPToolAction(bad)
		assert.False(t, ok, bad)
	}
}

func TestWithOhBinary(t *testing.T) {
	b := BundleSpec{MCP: []MCPServerDef{WorkflowMCPDef(), {Name: "gitlab", Type: "local", Command: []string{"oh", "mcp", "serve", "gitlab"}}}}
	out := b.WithOhBinary("/opt/oh")
	assert.Equal(t, []string{"/opt/oh", "mcp", "serve", "workflow"}, out.MCP[0].Command)
	assert.Equal(t, []string{"oh", "mcp", "serve", "gitlab"}, out.MCP[1].Command)
	assert.Equal(t, OhBinVar, b.MCP[0].Command[0], "the original bundle is not modified")
	assert.Nil(t, BundleSpec{}.WithOhBinary("/opt/oh").MCP)
}

func TestCheckpointDefBehavior(t *testing.T) {
	c := CheckpointDef{ID: "cp-1", Behaviors: map[string]string{"auto": "skip", "semi-auto": "auto"}}
	assert.Equal(t, CheckpointAuto, c.Behavior("semi-auto"))
	assert.Equal(t, CheckpointSkip, c.Behavior("auto"))
	assert.Equal(t, CheckpointPause, c.Behavior("manuel"), "unset = pause")
	c.Mandatory = true
	assert.Equal(t, CheckpointPause, c.Behavior("auto"), "mandatory never skipped")
	assert.Equal(t, "cp-1", c.LabelFor("fr"))
	c.Label = map[string]string{"fr": "Démarrer", "en": "Start"}
	assert.Equal(t, "Démarrer", c.LabelFor("fr"))
	assert.Equal(t, "Start", c.LabelFor("de"))
}

func TestCheckpointLookupIgnoresCase(t *testing.T) {
	wf := &WorkflowRuntime{Checkpoints: []CheckpointDef{{ID: "cp-1"}}}
	c, ok := wf.Checkpoint(" CP-1 ")
	assert.True(t, ok)
	assert.Equal(t, "cp-1", c.ID)
}
