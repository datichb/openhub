package opencodev2

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
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
