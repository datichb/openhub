package deploy

import (
	"encoding/json"
	"os"
	"testing"
)

func TestDeployAgentConfigE2E(t *testing.T) {
	// Read current opencode.json
	data, err := os.ReadFile("/Users/benjamin.datiche/workspace/transparence-sru/opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	
	agentCfg, _ := config["agent"].(map[string]interface{})
	
	// Simulate what DeployAgentConfig does for orchestrator
	fm, err := ParseAgentFrontmatter("/Users/benjamin.datiche/workspace/opencode-hub/agents/planning/orchestrator.md")
	if err != nil {
		t.Fatal(err)
	}
	
	block := buildAgentBlock(fm, "planning", nil, nil, nil, "amazon-bedrock")
	agentCfg["orchestrator"] = block
	config["agent"] = agentCfg
	
	// Check result
	result, _ := json.MarshalIndent(config["agent"].(map[string]interface{})["orchestrator"], "", "  ")
	t.Logf("orchestrator block after simulated deploy:\n%s", result)
	
	// Verify documentarian is in the task permissions
	orch := config["agent"].(map[string]interface{})["orchestrator"].(map[string]interface{})
	perm := orch["permission"].(map[string]interface{})
	task := perm["task"].(map[string]interface{})
	if _, ok := task["documentarian"]; !ok {
		t.Error("documentarian NOT in task after simulated deploy")
	} else {
		t.Log("documentarian IS in task after simulated deploy - writing would fix it")
	}
	
	// Now simulate DeployMCP reading and rewriting (without actually writing)
	// Check that DeployMCP preserves agent section
	mcpSection := config["mcp"]
	t.Logf("mcp section type: %T", mcpSection)
	
	// The critical question: does json.Marshal preserve map order?
	// Maps in Go have random iteration order, but JSON keys are alphabetical with MarshalIndent
	// The real issue might be something else entirely
}
