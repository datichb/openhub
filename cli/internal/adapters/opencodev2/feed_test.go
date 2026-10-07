package opencodev2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Event shapes recorded on opencode 2.0.20 (shell call + subagent delegation).
func TestFeedDecoder(t *testing.T) {
	ev := func(typ, data string) Event {
		return Event{Type: typ, Created: 1791231056574, Data: json.RawMessage(data)}
	}
	f := newFeedDecoder()
	decode := func(e Event) *domain.FeedItem {
		it, _ := f.decode(e)
		return it
	}

	it := decode(ev("session.step.started", `{"sessionID":"ses_r","agent":"lead","model":{"id":"eu.x"}}`))
	require.NotNil(t, it)
	assert.Equal(t, domain.FeedItem{Time: it.Time, SessionID: "ses_r", Kind: domain.FeedAgent, Agent: "lead"}, *it)

	assert.Nil(t, decode(ev("session.text.delta", `{"sessionID":"ses_r","delta":"LE"}`)))
	it = decode(ev("session.text.ended", `{"sessionID":"ses_r","text":"LEAD: I'll run it."}`))
	assert.Equal(t, domain.FeedText, it.Kind)
	assert.Equal(t, "LEAD: I'll run it.", it.Text)

	assert.Nil(t, decode(ev("session.tool.input.started", `{"sessionID":"ses_r","id":"t1","name":"shell"}`)))
	it = decode(ev("session.tool.called", `{"sessionID":"ses_r","id":"t1","input":{"command":"go test ./...\nmore"}}`))
	assert.Equal(t, domain.FeedTool, it.Kind)
	assert.Equal(t, "shell", it.Tool)
	assert.Equal(t, "go test ./... …", it.Title)
	it = decode(ev("session.tool.success", `{"sessionID":"ses_r","id":"t1","content":[{"type":"text","text":"ok"}]}`))
	assert.Equal(t, "ok", it.Status)
	assert.Equal(t, "shell", it.Tool)

	decode(ev("session.tool.input.started", `{"sessionID":"ses_r","id":"t2","name":"subagent"}`))
	it = decode(ev("session.tool.called", `{"sessionID":"ses_r","id":"t2","input":{"agent":"helper","description":"Answer math question","prompt":"2+2?"}}`))
	assert.Equal(t, domain.FeedDelegate, it.Kind)
	assert.Equal(t, "helper", it.Agent)
	assert.Equal(t, "Answer math question", it.Title)
	assert.Nil(t, decode(ev("session.tool.success", `{"sessionID":"ses_r","id":"t2"}`)), "the delegation result is not repeated")

	_, parent := f.decode(ev("session.created", `{"sessionID":"ses_c","parentID":"ses_r","agent":"helper"}`))
	assert.Equal(t, "ses_r", parent)

	assert.Equal(t, "failed", decode(ev("session.execution.failed", `{"sessionID":"ses_r"}`)).Status)
	assert.Equal(t, 0.25, decode(ev("session.usage.updated", `{"sessionID":"ses_r","cost":0.25}`)).Cost)
	assert.Equal(t, domain.FeedDecision, decode(ev("permission.asked", `{"sessionID":"ses_r"}`)).Kind)
	assert.Nil(t, decode(ev("vcs.branch.updated", `{}`)))
	assert.Len(t, []rune(clip(string(make([]rune, 500)), 10)), 10)
}

func TestFeedToolCalls(t *testing.T) {
	ev := func(typ, data string) Event { return Event{Type: typ, Data: json.RawMessage(data)} }
	f := newFeedDecoder()
	step := func(e Event) *adapters.ToolCall {
		c := f.callOf(e)
		f.decode(e)
		return c
	}
	assert.Nil(t, step(ev("session.tool.input.started", `{"sessionID":"s","id":"c1","name":"workflow_checkpoint"}`)))
	c := step(ev("session.tool.called", `{"sessionID":"s","id":"c1","input":{"id":"cp-1","summary":"ok"}}`))
	require.NotNil(t, c)
	assert.Equal(t, sessionspec.MCPToolAction("workflow", "checkpoint"), c.Action)
	assert.Equal(t, adapters.CallCalled, c.Status)
	assert.Equal(t, "cp-1", c.Input["id"])
	c = step(ev("session.tool.success", `{"sessionID":"s","id":"c1"}`))
	require.NotNil(t, c)
	assert.Equal(t, adapters.CallOK, c.Status)

	step(ev("session.tool.input.started", `{"sessionID":"s","id":"c2","name":"subagent"}`))
	c = step(ev("session.tool.called", `{"sessionID":"s","id":"c2","input":{"agent":"developer"}}`))
	assert.Equal(t, sessionspec.ActionSubagent, c.Action)
	c = step(ev("session.tool.failed", `{"sessionID":"s","id":"c2","error":{"type":"permission.rejected","message":"Permission denied: subagent"}}`))
	assert.Equal(t, adapters.CallFailed, c.Status)
	assert.Equal(t, "Permission denied: subagent", c.Error)

	step(ev("session.tool.input.started", `{"sessionID":"s","id":"c3","name":"skill"}`))
	c = step(ev("session.tool.called", `{"sessionID":"s","id":"c3","input":{"name":"dev-standards-go"}}`))
	assert.Equal(t, "dev-standards-go", c.Skill, "QB2: skill loaded, for agent_events")

	assert.Nil(t, step(ev("session.tool.success", `{"sessionID":"s","id":"unknown"}`)), "call started before the stream")
	assert.Equal(t, "shell", NeutralAction("shell"))
	assert.Equal(t, "workflow_checkpoint", ToolAction(sessionspec.MCPToolAction("workflow", sessionspec.WorkflowToolCheckpoint)),
		"the model sees the name used by the agents (Q3-2)")
	assert.Equal(t, sessionspec.MCPToolAction("workflow", "checkpoint"), NeutralAction("workflow_workflow_checkpoint"),
		"a session started before the rename is still read")
}
