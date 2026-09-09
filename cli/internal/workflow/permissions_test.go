package workflow

import (
	"testing"
)

func TestDeriveTaskPermissions_Base(t *testing.T) {
	base := BaseWorkflow()
	perms := DeriveTaskPermissions(&base)

	// Orchestrator should be able to invoke many agents.
	orchPerms := perms["orchestrator"]
	if len(orchPerms) == 0 {
		t.Fatal("orchestrator should have task permissions")
	}

	// Check specific permissions from the explicit CanInvoke.
	found := map[string]bool{}
	for _, p := range orchPerms {
		found[p] = true
	}
	for _, expected := range []string{"pathfinder", "planner", "onboarder", "designer", "orchestrator-dev", "debugger", "documentarian"} {
		if !found[expected] {
			t.Errorf("orchestrator should be able to invoke %s", expected)
		}
	}
}

func TestDeriveTaskPermissions_DisabledAgentExcluded(t *testing.T) {
	base := BaseWorkflow()
	// Disable benchmarker.
	agent := base.FindAgent("benchmarker")
	if agent == nil {
		t.Fatal("benchmarker not found")
	}
	agent.Role = RoleDisabled

	perms := DeriveTaskPermissions(&base)

	// No agent should list benchmarker in their permissions.
	for agentID, targets := range perms {
		for _, target := range targets {
			if target == "benchmarker" {
				t.Errorf("agent %s should not be able to invoke disabled benchmarker", agentID)
			}
		}
	}
}

func TestDeriveTaskPermissions_IndependentAgentExplicit(t *testing.T) {
	base := BaseWorkflow()
	perms := DeriveTaskPermissions(&base)

	// Database is independent with explicit CanInvoke: [documentarian].
	dbPerms := perms["database"]
	if len(dbPerms) == 0 {
		t.Fatal("database should have task permissions")
	}
	found := false
	for _, p := range dbPerms {
		if p == "documentarian" {
			found = true
		}
	}
	if !found {
		t.Error("database should be able to invoke documentarian")
	}
}

func TestDeriveTaskPermissions_CanBeInvokedByHonored(t *testing.T) {
	base := BaseWorkflow()
	perms := DeriveTaskPermissions(&base)

	// Documentarian has CanBeInvokedBy listing many agents.
	// Those agents should have documentarian in their permissions.
	doc := base.FindAgent("documentarian")
	if doc == nil || doc.TaskPermissions == nil {
		t.Fatal("documentarian not found or no task permissions")
	}

	for _, invokerID := range doc.TaskPermissions.CanBeInvokedBy {
		invokerPerms := perms[invokerID]
		found := false
		for _, p := range invokerPerms {
			if p == "documentarian" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("agent %s should be able to invoke documentarian (via CanBeInvokedBy)", invokerID)
		}
	}
}

func TestBuildTaskPermissionMap(t *testing.T) {
	m := BuildTaskPermissionMap([]string{"planner", "developer"})

	if m["*"] != "deny" {
		t.Errorf("expected *=deny, got %s", m["*"])
	}
	if m["planner"] != "allow" {
		t.Errorf("expected planner=allow, got %s", m["planner"])
	}
	if m["developer"] != "allow" {
		t.Errorf("expected developer=allow, got %s", m["developer"])
	}
	if _, ok := m["reviewer"]; ok {
		t.Error("reviewer should not be in the map")
	}
}

func TestBuildTaskPermissionMap_Empty(t *testing.T) {
	m := BuildTaskPermissionMap(nil)
	if len(m) != 1 || m["*"] != "deny" {
		t.Errorf("expected only *=deny, got %v", m)
	}
}
