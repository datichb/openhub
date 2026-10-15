package session

import (
	"fmt"
	"strings"
	"testing"
	"time"
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

// r9b (retro): the digest gives the duration, the cost per agent, the
// checkpoints asked several times and the instruction of a refusal.
func TestTraceDigestRetroFacts(t *testing.T) {
	start := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	end := start.Add(42*time.Minute + 5*time.Second)
	tr := &Trace{SessionID: "ses_2", Events: []TraceEvent{
		{Kind: TraceSession, ToolSession: "ses_2", Data: map[string]any{"workflow": "ticket", "state": "completed", "title": "ev-1",
			"started_at": start, "ended_at": end}},
		{Kind: TraceCheckpoint, Checkpoint: "cp-2", Event: "waiting"},
		{Kind: TraceDecision, DecisionKind: "checkpoint", Checkpoint: "cp-2", Resolution: "reject", Data: map[string]any{"message": "ajoute le test\ndu cas vide"}},
		{Kind: TraceCheckpoint, Checkpoint: "cp-2", Event: "waiting"},
		{Kind: TraceCheckpoint, Checkpoint: "cp-2", Event: "passed"},
		{Kind: TraceSummary, Data: map[string]any{"cost_usd": 1.2, "per_agent": map[string]map[string]any{
			"developer": {"cost": 0.9}, "reviewer": {"cost": 0.3}}}},
	}}
	d := tr.Digest()
	for _, want := range []string{"Titre : ev-1", "État : completed", "Durée : 42m5s", "Coût par agent : developer 0.90 $, reviewer 0.30 $",
		"Checkpoints demandés plusieurs fois :\n- cp-2 ×2", "checkpoint cp-2 → reject — « ajoute le test … »"} {
		if !strings.Contains(d, want) {
			t.Errorf("digest lacks %q:\n%s", want, d)
		}
	}
}
