package deploy

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDeployAgentConfigOrchestrator(t *testing.T) {
	hubDir := "/Users/benjamin.datiche/workspace/opencode-hub"
	srcDir := hubDir + "/agents"
	
	// Simulate what DeployAgentConfig does
	fm, err := ParseAgentFrontmatter(srcDir + "/planning/orchestrator.md")
	if err != nil {
		t.Fatal(err)
	}
	
	block := buildAgentBlock(fm, "planning", nil, nil, nil, "bedrock")
	b, _ := json.MarshalIndent(block, "", "  ")
	fmt.Println("Built block for orchestrator:")
	fmt.Println(string(b))
	
	// Check task permissions specifically
	perm, ok := block["permission"].(map[string]interface{})
	if !ok {
		t.Fatal("no permission in block")
	}
	task, ok := perm["task"].(map[string]interface{})
	if !ok {
		t.Fatal("no task in permission")
	}
	fmt.Println("\ntask permissions in built block:")
	for k, v := range task {
		fmt.Printf("  %q: %v\n", k, v)
	}
	if _, found := task["documentarian"]; !found {
		t.Error("documentarian NOT in built block task permissions")
	}
}
