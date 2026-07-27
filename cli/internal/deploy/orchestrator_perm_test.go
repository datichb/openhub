package deploy

import (
	"fmt"
	"testing"
)

func TestOrchestratorDocumentarianPerm(t *testing.T) {
	fm, err := ParseAgentFrontmatter("/Users/benjamin.datiche/workspace/opencode-hub/agents/planning/orchestrator.md")
	if err != nil {
		t.Fatal(err)
	}
	task, ok := fm.Permission["task"]
	if !ok {
		t.Fatal("no task permission in orchestrator frontmatter")
	}
	taskMap, ok := task.(map[string]interface{})
	if !ok {
		t.Fatalf("task is not a map: %T", task)
	}
	fmt.Println("task permissions:")
	for k, v := range taskMap {
		fmt.Printf("  %q: %v\n", k, v)
	}
	if _, found := taskMap["documentarian"]; !found {
		t.Error("documentarian NOT found in task permissions")
	} else {
		t.Log("documentarian FOUND in task permissions")
	}
}
