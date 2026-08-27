package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDeployAgentConfigE2E(t *testing.T) {
	// Read test opencode.json fixture
	data, err := os.ReadFile(filepath.Join("testdata", "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}

	agentCfg, _ := config["agent"].(map[string]interface{})

	// Simulate what DeployAgentConfig does for orchestrator
	fm, err := ParseAgentFrontmatter(filepath.Join("testdata", "orchestrator.md"))
	if err != nil {
		t.Fatal(err)
	}

	block := buildAgentBlock(fm, "planning", "", nil, nil, nil, "amazon-bedrock")
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
		t.Log("documentarian IS in task after simulated deploy")
	}

	// Verify MCP section is preserved
	mcpSection := config["mcp"]
	if mcpSection == nil {
		t.Error("mcp section lost during agent config update")
	}
}
