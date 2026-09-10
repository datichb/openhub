package workflow

import (
	"testing"
)

func TestResolve_BaseOnly(t *testing.T) {
	base := BaseWorkflow()
	resolved, err := Resolve(base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", resolved.Version)
	}
	if len(resolved.Checkpoints) != 5 {
		t.Errorf("expected 5 checkpoints, got %d", len(resolved.Checkpoints))
	}
	if len(resolved.Agents) != 19 {
		t.Errorf("expected 19 agents, got %d", len(resolved.Agents))
	}
}

func TestResolve_ModifyCheckpointBehavior(t *testing.T) {
	base := BaseWorkflow()
	override := WorkflowOverride{
		CheckpointOverrides: []CheckpointOverride{
			{
				ID:     "cp-1",
				Action: ActionModify,
				Behavior: map[string]CheckpointBehavior{
					"manuel": BehaviorAuto, // change from pause to auto
				},
			},
		},
	}

	resolved, err := Resolve(base, override)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cp := resolved.FindCheckpoint("cp-1")
	if cp == nil {
		t.Fatal("cp-1 not found")
	}
	if cp.Behavior["manuel"] != BehaviorAuto {
		t.Errorf("expected cp-1 manuel=auto, got %s", cp.Behavior["manuel"])
	}
	// Other modes should be unchanged.
	if cp.Behavior["semi-auto"] != BehaviorAuto {
		t.Errorf("expected cp-1 semi-auto=auto, got %s", cp.Behavior["semi-auto"])
	}
}

func TestResolve_AddCheckpoint(t *testing.T) {
	base := BaseWorkflow()
	label := "Review Gate"
	afterCP := "cp-1"

	override := WorkflowOverride{
		CheckpointOverrides: []CheckpointOverride{
			{
				ID:          "cp-review-gate",
				Action:      ActionAdd,
				Label:       &label,
				InsertAfter: &afterCP,
			},
		},
	}

	resolved, err := Resolve(base, override)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resolved.Checkpoints) != 6 {
		t.Errorf("expected 6 checkpoints, got %d", len(resolved.Checkpoints))
	}

	// Verify insertion order.
	var cpIDs []string
	for _, cp := range resolved.Checkpoints {
		cpIDs = append(cpIDs, cp.ID)
	}
	expected := []string{"cp-0", "cp-routing", "cp-1", "cp-review-gate", "cp-2", "cp-3"}
	for i, id := range expected {
		if i >= len(cpIDs) || cpIDs[i] != id {
			t.Errorf("checkpoint order mismatch at %d: expected %s, got %v", i, id, cpIDs)
			break
		}
	}
}

func TestResolve_RemoveCheckpoint(t *testing.T) {
	base := BaseWorkflow()
	override := WorkflowOverride{
		CheckpointOverrides: []CheckpointOverride{
			{ID: "cp-3", Action: ActionRemove},
		},
	}

	resolved, err := Resolve(base, override)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resolved.Checkpoints) != 4 {
		t.Errorf("expected 4 checkpoints, got %d", len(resolved.Checkpoints))
	}
	if resolved.FindCheckpoint("cp-3") != nil {
		t.Error("cp-3 should have been removed")
	}
}

func TestResolve_CannotRemoveMandatoryCheckpoint(t *testing.T) {
	base := BaseWorkflow()
	override := WorkflowOverride{
		CheckpointOverrides: []CheckpointOverride{
			{ID: "cp-2", Action: ActionRemove}, // cp-2 is mandatory
		},
	}

	_, err := Resolve(base, override)
	if err == nil {
		t.Fatal("expected error when removing mandatory checkpoint")
	}
}

func TestResolve_DisableAgent(t *testing.T) {
	base := BaseWorkflow()
	disabled := true
	override := WorkflowOverride{
		AgentOverrides: []AgentOverride{
			{AgentID: "benchmarker", Disabled: &disabled},
		},
	}

	resolved, err := Resolve(base, override)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent := resolved.FindAgent("benchmarker")
	if agent == nil {
		t.Fatal("benchmarker not found")
	}
	if agent.Role != RoleDisabled {
		t.Errorf("expected benchmarker role=disabled, got %s", agent.Role)
	}
}

func TestResolve_CannotDisableMandatoryAgent(t *testing.T) {
	base := BaseWorkflow()
	disabled := true
	override := WorkflowOverride{
		AgentOverrides: []AgentOverride{
			{AgentID: "orchestrator", Disabled: &disabled}, // mandatory
		},
	}

	_, err := Resolve(base, override)
	if err == nil {
		t.Fatal("expected error when disabling mandatory agent")
	}
}

func TestResolve_ChangeAgentMode(t *testing.T) {
	base := BaseWorkflow()
	mode := ModeSubagent
	override := WorkflowOverride{
		AgentOverrides: []AgentOverride{
			{AgentID: "reviewer", Mode: &mode},
		},
	}

	resolved, err := Resolve(base, override)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent := resolved.FindAgent("reviewer")
	if agent == nil {
		t.Fatal("reviewer not found")
	}
	if agent.Mode != ModeSubagent {
		t.Errorf("expected reviewer mode=subagent, got %s", agent.Mode)
	}
}

func TestResolve_ChangeAgentRole(t *testing.T) {
	base := BaseWorkflow()
	role := RoleWorkflow
	override := WorkflowOverride{
		AgentOverrides: []AgentOverride{
			{AgentID: "database", Role: &role}, // independent → workflow
		},
	}

	resolved, err := Resolve(base, override)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	agent := resolved.FindAgent("database")
	if agent == nil {
		t.Fatal("database not found")
	}
	if agent.Role != RoleWorkflow {
		t.Errorf("expected database role=workflow, got %s", agent.Role)
	}
}

func TestResolve_ModeOverrides(t *testing.T) {
	base := BaseWorkflow()
	modes := []string{"manuel", "turbo"}
	def := "turbo"
	override := WorkflowOverride{
		ModeOverrides: &ModesOverride{
			Available: &modes,
			Default:   &def,
		},
	}

	resolved, err := Resolve(base, override)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resolved.Modes.Available) != 2 {
		t.Errorf("expected 2 modes, got %d", len(resolved.Modes.Available))
	}
	if resolved.Modes.Default != "turbo" {
		t.Errorf("expected default=turbo, got %s", resolved.Modes.Default)
	}

	// Checkpoints should have behaviors for new modes.
	for _, cp := range resolved.Checkpoints {
		if _, ok := cp.Behavior["turbo"]; !ok {
			t.Errorf("checkpoint %s missing behavior for mode 'turbo'", cp.ID)
		}
		if _, ok := cp.Behavior["semi-auto"]; ok {
			t.Errorf("checkpoint %s still has behavior for removed mode 'semi-auto'", cp.ID)
		}
	}
}

func TestResolve_EnforcedStopsSubsequentOverrides(t *testing.T) {
	base := BaseWorkflow()

	hubOverride := WorkflowOverride{
		CircuitBreakerOverride: intPtr(20),
	}
	teamOverride := WorkflowOverride{
		CircuitBreakerOverride: intPtr(8),
		Enforced:               true,
	}
	projectOverride := WorkflowOverride{
		CircuitBreakerOverride: intPtr(50), // should be ignored
	}

	resolved, err := Resolve(base, hubOverride, teamOverride, projectOverride)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.CircuitBreaker.MaxConsecutiveInvocations != 8 {
		t.Errorf("expected circuit breaker=8 (team enforced), got %d", resolved.CircuitBreaker.MaxConsecutiveInvocations)
	}
}

func TestResolve_MultipleOverrideLayers(t *testing.T) {
	base := BaseWorkflow()

	// Hub: change default mode
	def := "semi-auto"
	hubOverride := WorkflowOverride{
		ModeOverrides: &ModesOverride{Default: &def},
	}

	// Team: disable benchmarker
	disabled := true
	teamOverride := WorkflowOverride{
		AgentOverrides: []AgentOverride{
			{AgentID: "benchmarker", Disabled: &disabled},
		},
	}

	// Project: change cp-1 behavior
	projectOverride := WorkflowOverride{
		CheckpointOverrides: []CheckpointOverride{
			{
				ID:     "cp-1",
				Action: ActionModify,
				Behavior: map[string]CheckpointBehavior{
					"semi-auto": BehaviorPause, // override from auto to pause
				},
			},
		},
	}

	resolved, err := Resolve(base, hubOverride, teamOverride, projectOverride)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Modes.Default != "semi-auto" {
		t.Errorf("expected default=semi-auto, got %s", resolved.Modes.Default)
	}

	agent := resolved.FindAgent("benchmarker")
	if agent == nil || agent.Role != RoleDisabled {
		t.Error("expected benchmarker disabled")
	}

	cp := resolved.FindCheckpoint("cp-1")
	if cp == nil || cp.Behavior["semi-auto"] != BehaviorPause {
		t.Error("expected cp-1 semi-auto=pause")
	}
}

func intPtr(i int) *int {
	return &i
}
