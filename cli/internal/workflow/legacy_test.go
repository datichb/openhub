package workflow

import (
	"strings"
	"testing"
)

func TestTranslateLegacyOverride(t *testing.T) {
	str := func(s string) *string { return &s }
	yes, no := true, false
	role := RoleIndependent
	mode := ModeSubagent
	modes := []string{"manuel", "semi-auto"}
	cb := 8
	agents := []string{"orchestrator-dev"}
	ov := &WorkflowOverride{
		CheckpointOverrides: []CheckpointOverride{
			{ID: "cp-1", Action: ActionModify, Label: str("Go, vraiment ?"), Behavior: map[string]CheckpointBehavior{"semi-auto": BehaviorPause}, Agents: &agents},
			{ID: "cp-3", Action: ActionRemove},
			{ID: "cp-security", Action: ActionAdd, Label: str("Sécurité"), Condition: str("si auth"), Behavior: map[string]CheckpointBehavior{"manuel": BehaviorConditional}, InsertAfter: str("cp-2")},
			{ID: "cp-routing", Action: ActionModify, Label: str("x")},
		},
		AgentOverrides: []AgentOverride{
			{AgentID: "documentarian", Disabled: &yes},
			{AgentID: "designer", Role: &role, Mode: &mode},
			{AgentID: "auditor", Disabled: &no},
			{AgentID: "developer", Position: &WorkflowPosition{AfterCheckpoint: "cp-security"}, TaskPermissions: &TaskPermOverride{CanInvoke: []string{"reviewer"}}},
		},
		ModeOverrides:          &ModesOverride{Available: &modes, Default: str("manuel")},
		CircuitBreakerOverride: &cb,
	}
	p := TranslateLegacyOverride(ov, "feature", "hub:feature", true, []string{"cp-0", "cp-1", "cp-2", "cp-3"})
	doc, diags := Parse(p.YAML, Source{Layer: LayerTeam})
	if diags.HasErrors() {
		t.Fatalf("%v\n%s", diags, p.YAML)
	}
	s := doc.Spec
	if s.ID != "feature" || s.Extends != "hub:feature" || len(s.Enforce) != 1 || s.Enforce[0] != EnforceAll {
		t.Fatalf("header: %+v", s)
	}
	cp1, _ := s.Checkpoints.Get("cp-1")
	if cp1.Label.Text("fr") != "Go, vraiment ?" || cp1.Mode["semi-auto"] != BehaviorPause {
		t.Fatalf("cp-1 = %+v", cp1)
	}
	if cp3, _ := s.Checkpoints.Get("cp-3"); !cp3.Disabled {
		t.Fatal("cp-3 not removed")
	}
	if sec, ok := s.Checkpoints.Get("cp-security"); !ok || sec.Condition != "si auth" {
		t.Fatalf("cp-security = %+v", sec)
	}
	if _, ok := s.Checkpoints.Get("cp-routing"); ok {
		t.Fatal("unknown checkpoint translated")
	}
	if a, _ := s.Agents.Get("documentarian"); a.Role != RoleDisabled {
		t.Fatalf("documentarian = %+v", a)
	}
	if a, _ := s.Agents.Get("designer"); a.Role != RoleIndependent || a.Mode != ModeSubagent {
		t.Fatalf("designer = %+v", a)
	}
	if a, _ := s.Agents.Get("auditor"); a.Role != RoleWorkflow {
		t.Fatalf("auditor = %+v", a)
	}
	if a, _ := s.Agents.Get("developer"); a.After != "cp-security" {
		t.Fatalf("developer = %+v", a)
	}
	if s.Modes.Default != "manuel" || len(s.Modes.Allowed) != 2 || *s.CircuitBreaker.MaxConsecutiveSubagents != 8 {
		t.Fatalf("modes/cb: %+v %+v", s.Modes, s.CircuitBreaker)
	}
	joined := strings.Join(p.Untranslated, "\n")
	for _, want := range []string{"cp-routing", "insert_after", "task_permissions", "agents [orchestrator-dev]"} {
		if !strings.Contains(joined, want) {
			t.Errorf("untranslated misses %q: %s", want, joined)
		}
	}
	if !strings.Contains(string(p.YAML), "# Not translated: checkpoint cp-routing") {
		t.Errorf("comments missing:\n%s", p.YAML)
	}

	empty := TranslateLegacyOverride(nil, "feature", "team:feature", false, nil)
	if _, d := Parse(empty.YAML, Source{Layer: LayerProject}); d.HasErrors() {
		t.Fatal(d)
	}
}
