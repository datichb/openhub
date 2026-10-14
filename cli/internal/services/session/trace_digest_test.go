package session

import (
	"fmt"
	"strings"
	"testing"
)

// R5: the digest of a trace (source session.trace()) names the session, its
// checkpoints, refusals, skills, delegations and outputs, in bounded lists.
func TestTraceDigest(t *testing.T) {
	no := false
	tr := &Trace{SessionID: "ses_1", Events: []TraceEvent{
		{Kind: TraceSession, ToolSession: "ses_1", Data: map[string]any{"workflow": "spec", "mode": "semi-auto"}},
		{Kind: TraceCheckpoint, Checkpoint: "cp-spec", Event: "passed", Text: "spec\nvalidée"},
		{Kind: TraceDecision, DecisionKind: "checkpoint", Checkpoint: "cp-tickets", Resolution: "reject"},
		{Kind: TraceDecision, DecisionKind: "permission", Title: "bash", Resolution: "once", Granted: &no},
		{Kind: TraceTool, Tool: "bash", Denied: true, Input: map[string]any{"command": "git push"}},
		{Kind: TraceSkill, Skill: "grilling"}, {Kind: TraceSkill, Skill: "grilling"},
		{Kind: TraceSubagent, Subagent: "planner"},
		{Kind: TraceOutput, Title: "tickets", Data: map[string]any{"value": []string{"ev-1"}}},
		{Kind: TraceSummary, Data: map[string]any{"cost_usd": 0.4321, "tool_calls": 12, "subagent_calls": 1, "decisions": 2, "denied": 1}},
	}}
	d := tr.Digest()
	for _, want := range []string{"Session ses_1 (workflow spec, mode semi-auto)", "Totaux : 0.43 $", "cp-spec passed — spec …",
		"checkpoint cp-tickets → reject", "bash git push", "grilling ×2", "planner", "tickets = [ev-1]"} {
		if !strings.Contains(d, want) {
			t.Errorf("digest lacks %q:\n%s", want, d)
		}
	}
	if strings.Contains(d, "permission bash → once") {
		t.Errorf("a granted decision is not a refusal:\n%s", d)
	}

	for i := range 20 {
		tr.Events = append(tr.Events, TraceEvent{Kind: TraceSkill, Skill: fmt.Sprintf("s%02d", i)})
	}
	if d := tr.Digest(); !strings.Contains(d, "… (9 de plus)") {
		t.Errorf("lists are bounded:\n%s", d)
	}
	if (&Trace{}).Digest() != "" {
		t.Error("empty trace")
	}
}
