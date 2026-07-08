package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_FullDeployPipeline tests the complete deployment pipeline
// with agent filtering, frontmatter parsing, model cascade, and permission injection.
func TestIntegration_FullDeployPipeline(t *testing.T) {
	hubDir, projectDir := setupIntegrationHub(t)

	// Simulate a project that selected only orchestrator and developer
	selectedAgents := []string{"orchestrator", "developer"}

	// Hub-level model overrides (from hub.toml [models])
	hubOverrides := &ModelOverrides{
		Default:  "claude-sonnet-4-5",
		Families: map[string]string{"planning": "claude-sonnet-4-6"},
	}

	// Project-level model overrides (from DB)
	projectOverrides := &ModelOverrides{
		Default: "claude-opus-4", // project global = opus-4
	}

	plan := &Plan{
		ProjectPath:    projectDir,
		ProjectID:      "test-integration",
		HubDir:         hubDir,
		Provider:       "anthropic",
		Model:          "claude-opus-4",
		SelectedAgents: selectedAgents,
		Phases: []Phase{
			DeployAgents(hubDir, selectedAgents),
			DeploySkills(hubDir, selectedAgents),
			DeployConfig("anthropic", "claude-opus-4"),
			DeployAgentConfig(hubDir, selectedAgents, projectOverrides, hubOverrides, "anthropic"),
		},
	}

	results, err := Execute(plan)
	require.NoError(t, err)
	assert.Len(t, results, 4)
	for _, r := range results {
		assert.True(t, r.Success, "phase %s failed: %s", r.Name, r.Message)
	}

	// === Verify agent files: only selected agents deployed ===
	assert.FileExists(t, filepath.Join(projectDir, ".opencode", "agents", "planning", "orchestrator.md"))
	assert.FileExists(t, filepath.Join(projectDir, ".opencode", "agents", "developer", "developer.md"))
	// reviewer NOT deployed (not selected)
	assert.NoFileExists(t, filepath.Join(projectDir, ".opencode", "agents", "quality", "reviewer.md"))

	// === Verify opencode.json ===
	data, err := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err)

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	// Global config — model is a string
	assert.Equal(t, "claude-opus-4", config["model"])

	// Provider is a named block
	providerCfg := config["provider"].(map[string]interface{})
	assert.Contains(t, providerCfg, "anthropic")

	// Agent section
	agentCfg := config["agent"].(map[string]interface{})

	// Native agents disabled
	for _, native := range DisabledNativeAgents {
		entry := agentCfg[native].(map[string]interface{})
		assert.Equal(t, true, entry["disable"])
	}

	// === Orchestrator: primary, has permissions, model from project global override ===
	orch := agentCfg["orchestrator"].(map[string]interface{})
	assert.NotContains(t, orch, "mode") // primary = not written
	// Model: project agent override? No. Project family "planning"? No (no project family).
	// Project global = "claude-opus-4" — this is level 3, wins.
	assert.Equal(t, "anthropic/claude-opus-4", orch["model"])

	orchPerm := orch["permission"].(map[string]interface{})
	assert.Equal(t, "allow", orchPerm["question"])
	assert.Equal(t, "deny", orchPerm["bash"])
	taskPerm := orchPerm["task"].(map[string]interface{})
	assert.Equal(t, "deny", taskPerm["*"])
	assert.Equal(t, "allow", taskPerm["planner"])

	// === Developer: subagent, has permissions, model from project global override ===
	dev := agentCfg["developer"].(map[string]interface{})
	assert.Equal(t, "subagent", dev["mode"])
	// Model: project global = "claude-opus-4" (level 3 wins — no agent/family override)
	assert.Equal(t, "anthropic/claude-opus-4", dev["model"])

	devPerm := dev["permission"].(map[string]interface{})
	assert.Equal(t, "deny", devPerm["question"])
	assert.Equal(t, "allow", devPerm["read"])
	assert.Equal(t, "allow", devPerm["edit"])
	assert.Equal(t, "allow", devPerm["write"])
	bashPerm := devPerm["bash"].(map[string]interface{})
	assert.Equal(t, "deny", bashPerm["*"])
	assert.Equal(t, "allow", bashPerm["git *"])
	assert.Equal(t, "allow", bashPerm["npm *"])

	// === Reviewer: NOT present (not selected) ===
	assert.NotContains(t, agentCfg, "reviewer")
}

// TestIntegration_DeployFilterOnly tests that unselected agents are completely excluded.
func TestIntegration_DeployFilterOnly(t *testing.T) {
	hubDir, projectDir := setupIntegrationHub(t)

	// Select only the reviewer
	selectedAgents := []string{"reviewer"}

	plan := &Plan{
		ProjectPath:    projectDir,
		ProjectID:      "filter-test",
		HubDir:         hubDir,
		SelectedAgents: selectedAgents,
		Phases: []Phase{
			DeployAgents(hubDir, selectedAgents),
			DeployConfig("", ""),
			DeployAgentConfig(hubDir, selectedAgents, nil, nil, "anthropic"),
		},
	}

	results, err := Execute(plan)
	require.NoError(t, err)

	// Verify only reviewer deployed
	assert.NoFileExists(t, filepath.Join(projectDir, ".opencode", "agents", "planning", "orchestrator.md"))
	assert.NoFileExists(t, filepath.Join(projectDir, ".opencode", "agents", "developer", "developer.md"))
	assert.FileExists(t, filepath.Join(projectDir, ".opencode", "agents", "quality", "reviewer.md"))

	// Verify only reviewer in opencode.json
	data, _ := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	agentCfg := config["agent"].(map[string]interface{})
	assert.Contains(t, agentCfg, "reviewer")
	assert.NotContains(t, agentCfg, "orchestrator")
	assert.NotContains(t, agentCfg, "developer")

	// Reviewer should have its frontmatter model
	rev := agentCfg["reviewer"].(map[string]interface{})
	assert.Equal(t, "anthropic/claude-opus-4", rev["model"])

	_ = results
}

// TestIntegration_BedrockProviderNormalization tests model normalization for bedrock.
func TestIntegration_BedrockProviderNormalization(t *testing.T) {
	hubDir, projectDir := setupIntegrationHub(t)

	selectedAgents := []string{"orchestrator", "reviewer"}

	plan := &Plan{
		ProjectPath:    projectDir,
		ProjectID:      "bedrock-test",
		HubDir:         hubDir,
		SelectedAgents: selectedAgents,
		Phases: []Phase{
			DeployAgents(hubDir, selectedAgents),
			DeployConfig("bedrock", ""),
			DeployAgentConfig(hubDir, selectedAgents, nil, nil, "bedrock"),
		},
	}

	results, err := Execute(plan)
	require.NoError(t, err)
	for _, r := range results {
		assert.True(t, r.Success, "phase %s: %s", r.Name, r.Message)
	}

	data, _ := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	agentCfg := config["agent"].(map[string]interface{})

	// Orchestrator: frontmatter "anthropic/claude-sonnet-4-6" → bedrock format
	orch := agentCfg["orchestrator"].(map[string]interface{})
	assert.Equal(t, "amazon-bedrock/anthropic.claude-sonnet-4-6-20250715-v1:0", orch["model"])

	// Reviewer: frontmatter "anthropic/claude-opus-4" → bedrock format
	rev := agentCfg["reviewer"].(map[string]interface{})
	assert.Equal(t, "amazon-bedrock/anthropic.claude-opus-4-20250514-v1:0", rev["model"])
}

// --- Test helpers ---

func setupIntegrationHub(t *testing.T) (hubDir, projectDir string) {
	t.Helper()
	hubDir = t.TempDir()
	projectDir = t.TempDir()

	// Planning family
	planningDir := filepath.Join(hubDir, "agents", "planning")
	require.NoError(t, os.MkdirAll(planningDir, 0o755))

	orchestratorMd := `---
id: orchestrator
label: Orchestrator
description: AI project manager
mode: primary
permission:
  question: allow
  skill: allow
  todowrite: allow
  bash: deny
  read: deny
  edit: deny
  glob: deny
  grep: deny
  write: deny
  task:
    "*": deny
    "planner": allow
    "debugger": allow
    "orchestrator-dev": allow
  ctx_search: allow
  ctx_batch_execute: allow
model: anthropic/claude-sonnet-4-6
skills: [orchestrator/orchestrator-protocol]
---

# Orchestrator

AI project manager agent.
`
	require.NoError(t, os.WriteFile(filepath.Join(planningDir, "orchestrator.md"), []byte(orchestratorMd), 0o644))

	// Developer family
	devDir := filepath.Join(hubDir, "agents", "developer")
	require.NoError(t, os.MkdirAll(devDir, 0o755))

	developerMd := `---
id: developer
label: Developer
description: Development assistant
mode: subagent
permission:
  question: deny
  skill: allow
  bash:
    "*": deny
    "git *": allow
    "npm *": allow
    "bd *": allow
  read: allow
  glob: allow
  grep: allow
  edit: allow
  write: allow
  task:
    "*": deny
    "documentarian": allow
  ctx_search: allow
  ctx_execute: allow
skills: [dev-standards-universal]
---

# Developer

Generic development assistant.
`
	require.NoError(t, os.WriteFile(filepath.Join(devDir, "developer.md"), []byte(developerMd), 0o644))

	// Quality family
	qualityDir := filepath.Join(hubDir, "agents", "quality")
	require.NoError(t, os.MkdirAll(qualityDir, 0o755))

	reviewerMd := `---
id: reviewer
label: CodeReviewer
description: Code reviewer
mode: primary
permission:
  read: allow
  glob: allow
  grep: allow
  edit: deny
  write: deny
  bash:
    "*": deny
    "git diff*": allow
    "git log*": allow
  ctx_search: allow
model: anthropic/claude-opus-4
skills: [review-protocol]
---

# Reviewer

Code review agent.
`
	require.NoError(t, os.WriteFile(filepath.Join(qualityDir, "reviewer.md"), []byte(reviewerMd), 0o644))

	// Skills directory
	skillsDir := filepath.Join(hubDir, "skills", "shared")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "coding.md"), []byte("# Coding Skill"), 0o644))

	return hubDir, projectDir
}
