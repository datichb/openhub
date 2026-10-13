package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

func TestPrintTrace(t *testing.T) {
	granted := false
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	tr := &sessionsvc.Trace{SessionID: "ses_a", Transcript: sessionsvc.TranscriptLive, Events: []sessionsvc.TraceEvent{
		{Seq: 1, Time: at, Kind: sessionsvc.TraceSession},
		{Seq: 2, Time: at, Kind: sessionsvc.TraceSubagent, Agent: "lead", Subagent: "developer", Input: map[string]any{"description": "fix\nmore"}},
		{Seq: 3, Time: at, Kind: sessionsvc.TraceTool, Agent: "developer", InSubagent: true, Tool: "shell", Status: "ok", Verification: true, Input: map[string]any{"command": "go test ./..."}},
		{Seq: 4, Time: at, Kind: sessionsvc.TraceDecision, DecisionKind: "permission", Action: "shell", Resources: []string{"git push"}, Resolution: "reject", Granted: &granted},
		{Seq: 5, Time: at, Kind: sessionsvc.TraceCheckpoint, Event: "passed", Checkpoint: "cp-2", By: "oh"},
		{Seq: 6, Time: at, Kind: sessionsvc.TraceSummary, Data: map[string]any{"cost_usd": 0.25, "tokens_in": 10, "tokens_out": 5, "tool_calls": 1, "subagent_calls": 1, "decisions": 1, "denied": 1}},
	}}
	var b bytes.Buffer
	printTrace(&b, tr)
	out := b.String()
	for _, want := range []string{"ses_a", "→ developer  fix …", "shell  go test ./...  ok", "permission  shell git push  → reject", "passed  cp-2  (oh)", "$0.2500"} {
		assert.Contains(t, out, want)
	}
}
