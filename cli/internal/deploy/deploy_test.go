package deploy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestHub(t *testing.T) (hubDir, projectDir string) {
	t.Helper()
	hubDir = t.TempDir()
	projectDir = t.TempDir()

	// Create hub agents with frontmatter
	agentsDir := filepath.Join(hubDir, "agents", "dev")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))

	coderAgent := `---
id: coder
label: Coder
description: A coding agent
mode: primary
permission:
  read: allow
skills: [shared/coding-inline]
native_skills: [shared/coding]
---

# Coder Agent
`
	reviewerAgent := `---
id: reviewer
label: Reviewer
description: A reviewing agent
mode: primary
permission:
  read: allow
native_skills: [shared/coding]
---

# Reviewer Agent
`
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "coder.md"), []byte(coderAgent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "reviewer.md"), []byte(reviewerAgent), 0o644))

	// Create hub skills
	skillsDir := filepath.Join(hubDir, "skills", "shared")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))

	codingSkill := `---
name: coding
description: Coding best practices
---

# Coding Skill

Follow these coding practices.
`
	codingInlineSkill := `---
name: coding-inline
description: Inline coding skill
---

# Inline Coding Skill

This should be inlined into the agent.
`
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "coding.md"), []byte(codingSkill), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "coding-inline.md"), []byte(codingInlineSkill), 0o644))

	return hubDir, projectDir
}

func TestExecute_FullDeploy(t *testing.T) {
	hubDir, projectDir := setupTestHub(t)

	plan := &Plan{
		ProjectPath: projectDir,
		ProjectID:   "test-project",
		HubDir:      hubDir,
		Provider:    "bedrock",
		Model:       "claude-opus-4",
		Phases: []Phase{
			DeployAgents(hubDir, nil),
			DeploySkills(hubDir, nil),
			DeployConfig("bedrock", "claude-opus-4"),
		},
	}

	results, err := Execute(plan)
	require.NoError(t, err)
	assert.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.Success, "phase %s failed: %s", r.Name, r.Message)
	}

	// Verify agents deployed (in subdirectory matching hub structure)
	assert.FileExists(t, filepath.Join(projectDir, ".opencode", "agents", "dev", "coder.md"))
	assert.FileExists(t, filepath.Join(projectDir, ".opencode", "agents", "dev", "reviewer.md"))

	// Verify Bucket A skill was inlined into the coder agent
	coderData, _ := os.ReadFile(filepath.Join(projectDir, ".opencode", "agents", "dev", "coder.md"))
	assert.Contains(t, string(coderData), "Inline Coding Skill") // inlined content
	assert.NotContains(t, string(coderData), "skills:")          // hub field stripped

	// Verify Bucket B skills deployed in opencode format: <name>/SKILL.md
	assert.FileExists(t, filepath.Join(projectDir, ".opencode", "skills", "coding", "SKILL.md"))

	// Verify config
	assert.FileExists(t, filepath.Join(projectDir, "opencode.json"))
	data, _ := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	assert.Contains(t, string(data), "amazon-bedrock")
}

func TestExecute_Rollback(t *testing.T) {
	hubDir, projectDir := setupTestHub(t)

	// Pre-existing opencode.json to verify rollback restores it
	originalConfig := `{"version": "original"}`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "opencode.json"), []byte(originalConfig), 0o644))

	// Phase that will fail
	failingPhase := Phase{
		Name: "Failing",
		Execute: func(ctx *Context) error {
			return fmt.Errorf("intentional failure")
		},
	}

	plan := &Plan{
		ProjectPath: projectDir,
		ProjectID:   "test-project",
		HubDir:      hubDir,
		Phases: []Phase{
			DeployAgents(hubDir, nil), // This succeeds
			failingPhase,              // This fails → rollback
		},
	}

	results, err := Execute(plan)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rolled back")
	assert.Len(t, results, 2)
	assert.True(t, results[0].Success)
	assert.False(t, results[1].Success)

	// Verify rollback: opencode.json should be restored to original content
	data, err2 := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	require.NoError(t, err2)
	assert.Equal(t, originalConfig, string(data))
}

func TestExecute_NoAgents(t *testing.T) {
	emptyHub := t.TempDir()
	projectDir := t.TempDir()

	plan := &Plan{
		ProjectPath: projectDir,
		HubDir:      emptyHub,
		Phases: []Phase{
			DeployAgents(emptyHub, nil),
			DeploySkills(emptyHub, nil),
		},
	}

	results, err := Execute(plan)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success)
	}
}

func TestDeployConfig_MergesExisting(t *testing.T) {
	projectDir := t.TempDir()
	existing := `{"custom": "value", "provider": {"other": "keep"}}`
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "opencode.json"), []byte(existing), 0o644))

	plan := &Plan{
		ProjectPath: projectDir,
		Phases: []Phase{
			DeployConfig("bedrock", "claude-opus-4"),
		},
	}

	results, err := Execute(plan)
	require.NoError(t, err)
	assert.True(t, results[0].Success)

	data, _ := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	// Preserved existing entries
	assert.Contains(t, config, "custom")

	// Model is a string (not an object)
	assert.Equal(t, "claude-opus-4", config["model"])

	// Provider is a named block (amazon-bedrock for "bedrock")
	providerCfg := config["provider"].(map[string]interface{})
	assert.Contains(t, providerCfg, "amazon-bedrock")
	assert.Contains(t, providerCfg, "other") // preserved from existing

	// $schema present
	assert.Equal(t, "https://opencode.ai/config.json", config["$schema"])

	// enabled_providers set
	enabledProviders := config["enabled_providers"].([]interface{})
	assert.Contains(t, enabledProviders, "amazon-bedrock")
}

func TestDeployConfig_PluginAndCompaction(t *testing.T) {
	projectDir := t.TempDir()

	plan := &Plan{
		ProjectPath: projectDir,
		Phases: []Phase{
			DeployConfig("anthropic", "claude-sonnet-4-5"),
		},
	}

	results, err := Execute(plan)
	require.NoError(t, err)
	assert.True(t, results[0].Success)

	data, _ := os.ReadFile(filepath.Join(projectDir, "opencode.json"))

	var config map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &config))

	// Plugin: context-mode always deployed
	plugins, ok := config["plugin"].([]interface{})
	require.True(t, ok, "plugin should be an array")
	assert.Contains(t, plugins, "context-mode")

	// Compaction: standard settings
	compaction, ok := config["compaction"].(map[string]interface{})
	require.True(t, ok, "compaction should be an object")
	assert.Equal(t, true, compaction["auto"])
	assert.Equal(t, true, compaction["prune"])
	assert.Equal(t, float64(10000), compaction["reserved"]) // JSON numbers are float64
}
