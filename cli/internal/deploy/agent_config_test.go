package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAgentConfigTest(t *testing.T) (hubDir, projectDir string) {
	t.Helper()
	hubDir = t.TempDir()
	projectDir = t.TempDir()

	// Create agent files with frontmatter
	planningDir := filepath.Join(hubDir, "agents", "planning")
	require.NoError(t, os.MkdirAll(planningDir, 0o755))

	orchestrator := `---
id: orchestrator
label: Orchestrator
description: AI project manager
mode: primary
permission:
  question: allow
  skill: allow
  bash: deny
  read: deny
  task:
    "*": deny
    "planner": allow
    "debugger": allow
model: anthropic/claude-sonnet-4-6
skills: [skill-a]
---

# Orchestrator
`
	require.NoError(t, os.WriteFile(filepath.Join(planningDir, "orchestrator.md"), []byte(orchestrator), 0o644))

	devDir := filepath.Join(hubDir, "agents", "developer")
	require.NoError(t, os.MkdirAll(devDir, 0o755))

	developer := `---
id: developer
label: Developer
description: Dev assistant
mode: subagent
permission:
  question: deny
  skill: allow
  bash:
    "*": deny
    "git *": allow
    "npm *": allow
  read: allow
  edit: allow
  write: allow
  task:
    "*": deny
    "documentarian": allow
skills: [dev-standards]
---

# Developer
`
	require.NoError(t, os.WriteFile(filepath.Join(devDir, "developer.md"), []byte(developer), 0o644))

	qualityDir := filepath.Join(hubDir, "agents", "quality")
	require.NoError(t, os.MkdirAll(qualityDir, 0o755))

	reviewer := `---
id: reviewer
label: Reviewer
description: Code reviewer
mode: primary
permission:
  read: allow
  glob: allow
  grep: allow
  edit: deny
  write: deny
model: anthropic/claude-opus-4
skills: [review-protocol]
---

# Reviewer
`
	require.NoError(t, os.WriteFile(filepath.Join(qualityDir, "reviewer.md"), []byte(reviewer), 0o644))

	// Create a minimal opencode.json (as DeployConfig would have left it)
	initialConfig := map[string]interface{}{
		"agent": map[string]interface{}{
			"build":   map[string]interface{}{"disable": true},
			"plan":    map[string]interface{}{"disable": true},
			"general": map[string]interface{}{"disable": true},
		},
	}
	data, _ := json.MarshalIndent(initialConfig, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "opencode.json"), data, 0o644))

	return hubDir, projectDir
}

func TestDeployAgentConfig_AllAgents(t *testing.T) {
	hubDir, projectDir := setupAgentConfigTest(t)

	plan := &Plan{
		ProjectPath: projectDir,
		HubDir:      hubDir,
	}

	phase := DeployAgentConfig(hubDir, nil, nil, nil, "anthropic")
	ctx := &Context{Plan: plan}
	err := phase.Execute(ctx)
	require.NoError(t, err)

	// Read result
	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	agentCfg := config["agent"].(map[string]interface{})

	// Verify disabled agents are preserved
	assert.Contains(t, agentCfg, "build")
	assert.Contains(t, agentCfg, "plan")
	assert.Contains(t, agentCfg, "general")

	// Verify orchestrator
	orch := agentCfg["orchestrator"].(map[string]interface{})
	assert.Equal(t, "anthropic/claude-sonnet-4-6", orch["model"])
	assert.NotContains(t, orch, "mode") // primary = no mode field
	orchPerm := orch["permission"].(map[string]interface{})
	assert.Equal(t, "allow", orchPerm["question"])
	assert.Equal(t, "deny", orchPerm["bash"])
	taskPerm := orchPerm["task"].(map[string]interface{})
	assert.Equal(t, "deny", taskPerm["*"])
	assert.Equal(t, "allow", taskPerm["planner"])

	// Verify developer (subagent)
	dev := agentCfg["developer"].(map[string]interface{})
	assert.Equal(t, "subagent", dev["mode"])
	assert.NotContains(t, dev, "model") // no model in frontmatter
	devPerm := dev["permission"].(map[string]interface{})
	assert.Equal(t, "deny", devPerm["question"])
	assert.Equal(t, "allow", devPerm["read"])
	bashPerm := devPerm["bash"].(map[string]interface{})
	assert.Equal(t, "deny", bashPerm["*"])
	assert.Equal(t, "allow", bashPerm["git *"])

	// Verify reviewer
	rev := agentCfg["reviewer"].(map[string]interface{})
	assert.Equal(t, "anthropic/claude-opus-4", rev["model"])
	assert.NotContains(t, rev, "mode") // primary
	revPerm := rev["permission"].(map[string]interface{})
	assert.Equal(t, "allow", revPerm["read"])
	assert.Equal(t, "deny", revPerm["edit"])
}

func TestDeployAgentConfig_FilteredAgents(t *testing.T) {
	hubDir, projectDir := setupAgentConfigTest(t)

	plan := &Plan{
		ProjectPath: projectDir,
		HubDir:      hubDir,
	}

	// Only deploy orchestrator
	phase := DeployAgentConfig(hubDir, []string{"orchestrator"}, nil, nil, "anthropic")
	ctx := &Context{Plan: plan}
	err := phase.Execute(ctx)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	agentCfg := config["agent"].(map[string]interface{})

	// Only orchestrator should be configured
	assert.Contains(t, agentCfg, "orchestrator")
	assert.NotContains(t, agentCfg, "developer")
	assert.NotContains(t, agentCfg, "reviewer")

	// Disabled agents still present
	assert.Contains(t, agentCfg, "build")
}

func TestDeployAgentConfig_ModelCascade(t *testing.T) {
	hubDir, projectDir := setupAgentConfigTest(t)

	plan := &Plan{
		ProjectPath: projectDir,
		HubDir:      hubDir,
	}

	// Hub override: reviewer gets a specific model
	hubOverrides := &ModelOverrides{
		Default:  "claude-sonnet-4-5",
		Families: map[string]string{"planning": "claude-sonnet-4-6"},
		Agents:   map[string]string{"reviewer": "claude-opus-4"},
	}

	phase := DeployAgentConfig(hubDir, nil, nil, hubOverrides, "anthropic")
	ctx := &Context{Plan: plan}
	err := phase.Execute(ctx)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	agentCfg := config["agent"].(map[string]interface{})

	// Orchestrator: hub family "planning" override = claude-sonnet-4-6
	orch := agentCfg["orchestrator"].(map[string]interface{})
	assert.Equal(t, "anthropic/claude-sonnet-4-6", orch["model"])

	// Reviewer: hub agent override = claude-opus-4
	rev := agentCfg["reviewer"].(map[string]interface{})
	assert.Equal(t, "anthropic/claude-opus-4", rev["model"])

	// Developer: hub default = claude-sonnet-4-5 (no agent/family override, no frontmatter model)
	dev := agentCfg["developer"].(map[string]interface{})
	assert.Equal(t, "anthropic/claude-sonnet-4-5", dev["model"])
}

func TestDeployAgentConfig_ProjectOverrideBeatsHub(t *testing.T) {
	hubDir, projectDir := setupAgentConfigTest(t)

	plan := &Plan{
		ProjectPath: projectDir,
		HubDir:      hubDir,
	}

	projectOverrides := &ModelOverrides{
		Agents: map[string]string{"reviewer": "claude-sonnet-4-5"},
	}
	hubOverrides := &ModelOverrides{
		Agents: map[string]string{"reviewer": "claude-opus-4"},
	}

	phase := DeployAgentConfig(hubDir, []string{"reviewer"}, projectOverrides, hubOverrides, "anthropic")
	ctx := &Context{Plan: plan}
	err := phase.Execute(ctx)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	agentCfg := config["agent"].(map[string]interface{})
	rev := agentCfg["reviewer"].(map[string]interface{})
	// Project agent override wins over hub agent override
	assert.Equal(t, "anthropic/claude-sonnet-4-5", rev["model"])
}

func TestDeployAgentConfig_BedrockNormalization(t *testing.T) {
	hubDir, projectDir := setupAgentConfigTest(t)

	plan := &Plan{
		ProjectPath: projectDir,
		HubDir:      hubDir,
	}

	// Deploy with bedrock provider — models should be normalized
	phase := DeployAgentConfig(hubDir, []string{"orchestrator", "reviewer"}, nil, nil, "bedrock")
	ctx := &Context{Plan: plan}
	err := phase.Execute(ctx)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	agentCfg := config["agent"].(map[string]interface{})

	// Orchestrator frontmatter: "anthropic/claude-sonnet-4-6" → bedrock format
	orch := agentCfg["orchestrator"].(map[string]interface{})
	assert.Equal(t, "amazon-bedrock/anthropic.claude-sonnet-4-6-20250715-v1:0", orch["model"])

	// Reviewer frontmatter: "anthropic/claude-opus-4" → bedrock format
	rev := agentCfg["reviewer"].(map[string]interface{})
	assert.Equal(t, "amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0", rev["model"])
}

func TestDeployAgentConfig_NoAgentsDir(t *testing.T) {
	emptyHub := t.TempDir()
	projectDir := t.TempDir()

	// Create minimal opencode.json
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "opencode.json"), []byte("{}"), 0o644))

	plan := &Plan{
		ProjectPath: projectDir,
		HubDir:      emptyHub,
	}

	phase := DeployAgentConfig(emptyHub, nil, nil, nil, "anthropic")
	ctx := &Context{Plan: plan}
	err := phase.Execute(ctx)
	require.NoError(t, err) // Should not error on missing agents dir
}
