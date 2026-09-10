package deploy

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// TestWorkflowDeployBaseNoOverrides verifies that resolving the base workflow
// (no overrides) produces a valid result with all expected agents and skills.
func TestWorkflowDeployBaseNoOverrides(t *testing.T) {
	result, err := ResolveAndPrepareWorkflow(workflow.BaseWorkflow())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolved workflow should have the base version
	if result.Resolved.Version != "1.0" {
		t.Errorf("expected version 1.0, got %s", result.Resolved.Version)
	}

	// Should have 5 checkpoints
	if len(result.Resolved.Checkpoints) != 5 {
		t.Errorf("expected 5 checkpoints, got %d", len(result.Resolved.Checkpoints))
	}

	// Should have 19 agents
	if len(result.Resolved.Agents) != 19 {
		t.Errorf("expected 19 agents, got %d", len(result.Resolved.Agents))
	}

	// No agents should be disabled in the base workflow
	if len(result.DisabledAgents) != 0 {
		t.Errorf("expected 0 disabled agents, got %d: %v", len(result.DisabledAgents), result.DisabledAgents)
	}

	// Generated skills should be present
	if len(result.GeneratedSkills) < 2 {
		t.Errorf("expected at least 2 generated skills, got %d", len(result.GeneratedSkills))
	}

	// Verify specific generated skill keys
	for _, key := range []string{
		"orchestrator/orchestrator-workflow-modes",
		"shared/hub-workflow-reference",
	} {
		if _, ok := result.GeneratedSkills[key]; !ok {
			t.Errorf("missing generated skill: %s", key)
		}
	}
}

// TestWorkflowDeployPermissionsMatchBase verifies that the workflow-derived
// task permissions for the base workflow are consistent with the frontmatter
// declarations. This is the critical retrocompatibility test.
func TestWorkflowDeployPermissionsMatchBase(t *testing.T) {
	base := workflow.BaseWorkflow()
	perms := workflow.DeriveTaskPermissions(&base)

	// Orchestrator should have permissions for its known targets
	orchPerms := perms["orchestrator"]
	orchSet := toSet(orchPerms)

	expectedOrch := []string{"pathfinder", "planner", "onboarder", "designer", "orchestrator-dev", "debugger", "documentarian"}
	for _, exp := range expectedOrch {
		if !orchSet[exp] {
			t.Errorf("orchestrator should be able to invoke %s", exp)
		}
	}

	// Orchestrator-dev should have its known targets
	odevPerms := perms["orchestrator-dev"]
	odevSet := toSet(odevPerms)
	expectedOdev := []string{"developer", "developer-refactor", "developer-migrator", "reviewer", "documentarian"}
	for _, exp := range expectedOdev {
		if !odevSet[exp] {
			t.Errorf("orchestrator-dev should be able to invoke %s", exp)
		}
	}

	// No disabled agents in the base workflow
	disabled := DisabledAgentIDs(&base)
	if len(disabled) != 0 {
		t.Errorf("base workflow should have no disabled agents, got %v", disabled)
	}
}

// TestWorkflowDeployWithOverride verifies that a simple override produces
// the expected changes while keeping the rest intact.
func TestWorkflowDeployWithOverride(t *testing.T) {
	disabled := true
	override := workflow.WorkflowOverride{
		AgentOverrides: []workflow.AgentOverride{
			{AgentID: "benchmarker", Disabled: &disabled},
		},
	}

	result, err := ResolveAndPrepareWorkflow(workflow.BaseWorkflow(), override)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Benchmarker should be in disabled list
	if len(result.DisabledAgents) != 1 || result.DisabledAgents[0] != "benchmarker" {
		t.Errorf("expected [benchmarker] in disabled agents, got %v", result.DisabledAgents)
	}

	// Generated skills should still be valid
	if len(result.GeneratedSkills) < 2 {
		t.Errorf("expected at least 2 generated skills, got %d", len(result.GeneratedSkills))
	}

	// Permissions should not include benchmarker
	perms := workflow.DeriveTaskPermissions(&result.Resolved)
	for agentID, targets := range perms {
		for _, target := range targets {
			if target == "benchmarker" {
				t.Errorf("agent %s should not have benchmarker in permissions", agentID)
			}
		}
	}
}

// TestWorkflowDeployHelpers tests IsSkillGenerated and IsAgentDisabled.
func TestWorkflowDeployHelpers(t *testing.T) {
	result, _ := ResolveAndPrepareWorkflow(workflow.BaseWorkflow())

	if !result.IsSkillGenerated("orchestrator/orchestrator-workflow-modes") {
		t.Error("orchestrator-workflow-modes should be generated")
	}
	if result.IsSkillGenerated("nonexistent/skill") {
		t.Error("nonexistent skill should not be generated")
	}
	if result.IsAgentDisabled("orchestrator") {
		t.Error("orchestrator should not be disabled in base")
	}

	// Nil result should not panic
	var nilResult *WorkflowDeployResult
	if nilResult.IsSkillGenerated("anything") {
		t.Error("nil result should return false")
	}
	if nilResult.IsAgentDisabled("anything") {
		t.Error("nil result should return false")
	}
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
