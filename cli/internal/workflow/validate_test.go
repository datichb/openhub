package workflow

import (
	"testing"
)

func TestValidate_BaseWorkflow(t *testing.T) {
	base := BaseWorkflow()
	if err := Validate(&base); err != nil {
		t.Fatalf("base workflow should be valid: %v", err)
	}
}

func TestValidate_EmptyCheckpoints(t *testing.T) {
	wf := WorkflowDefinition{
		Checkpoints: nil,
		Agents:      BaseWorkflow().Agents,
		Modes: ModesConfig{
			Available: []string{"manuel"},
			Default:   "manuel",
		},
	}
	if err := Validate(&wf); err == nil {
		t.Fatal("expected error for empty checkpoints")
	}
}

func TestValidate_DuplicateCheckpointID(t *testing.T) {
	base := BaseWorkflow()
	base.Checkpoints = append(base.Checkpoints, Checkpoint{
		ID:    "cp-0", // duplicate
		Label: "Duplicate",
		Behavior: map[string]CheckpointBehavior{
			"manuel": BehaviorPause, "semi-auto": BehaviorPause, "auto": BehaviorPause,
		},
	})
	if err := Validate(&base); err == nil {
		t.Fatal("expected error for duplicate checkpoint ID")
	}
}

func TestValidate_MissingBehaviorForMode(t *testing.T) {
	base := BaseWorkflow()
	// Remove a behavior from cp-0.
	delete(base.Checkpoints[0].Behavior, "auto")
	if err := Validate(&base); err == nil {
		t.Fatal("expected error for missing behavior")
	}
}

func TestValidate_ConditionalWithoutCondition(t *testing.T) {
	base := BaseWorkflow()
	base.Checkpoints[0].Behavior["manuel"] = BehaviorConditional
	base.Checkpoints[0].Condition = "" // empty
	if err := Validate(&base); err == nil {
		t.Fatal("expected error for conditional without condition")
	}
}

func TestValidate_UnknownAgentInCheckpoint(t *testing.T) {
	base := BaseWorkflow()
	base.Checkpoints[0].Agents = append(base.Checkpoints[0].Agents, "nonexistent-agent")
	if err := Validate(&base); err == nil {
		t.Fatal("expected error for unknown agent in checkpoint")
	}
}

func TestValidate_DuplicateAgentID(t *testing.T) {
	base := BaseWorkflow()
	base.Agents = append(base.Agents, AgentSlot{
		AgentID: "orchestrator", // duplicate
		Role:    RoleWorkflow,
		Mode:    ModePrimary,
	})
	if err := Validate(&base); err == nil {
		t.Fatal("expected error for duplicate agent ID")
	}
}

func TestValidate_EmptyModes(t *testing.T) {
	base := BaseWorkflow()
	base.Modes.Available = nil
	if err := Validate(&base); err == nil {
		t.Fatal("expected error for empty modes")
	}
}

func TestValidate_DefaultModeNotInAvailable(t *testing.T) {
	base := BaseWorkflow()
	base.Modes.Default = "nonexistent"
	if err := Validate(&base); err == nil {
		t.Fatal("expected error for default mode not in available")
	}
}

func TestValidate_NegativeCircuitBreaker(t *testing.T) {
	base := BaseWorkflow()
	base.CircuitBreaker.MaxConsecutiveInvocations = -1
	if err := Validate(&base); err == nil {
		t.Fatal("expected error for negative circuit breaker")
	}
}

func TestValidate_InvalidAgentPermReference(t *testing.T) {
	base := BaseWorkflow()
	agent := base.FindAgent("database")
	if agent == nil {
		t.Fatal("database agent not found")
	}
	agent.TaskPermissions.CanInvoke = append(agent.TaskPermissions.CanInvoke, "ghost-agent")
	if err := Validate(&base); err == nil {
		t.Fatal("expected error for unknown agent in can_invoke")
	}
}
