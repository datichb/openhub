package deploy

import (
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

	// Create hub agents
	agentsDir := filepath.Join(hubDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "coder.md"), []byte("# Coder Agent"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "reviewer.md"), []byte("# Reviewer Agent"), 0o644))

	// Create hub skills
	skillsDir := filepath.Join(hubDir, "skills", "shared")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "coding.md"), []byte("# Coding Skill"), 0o644))

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
			DeployAgents(hubDir),
			DeploySkills(hubDir),
			DeployConfig("bedrock", "claude-opus-4"),
		},
	}

	results, err := Execute(plan)
	require.NoError(t, err)
	assert.Len(t, results, 3)
	for _, r := range results {
		assert.True(t, r.Success, "phase %s failed: %s", r.Name, r.Message)
	}

	// Verify agents deployed
	assert.FileExists(t, filepath.Join(projectDir, ".opencode", "agents", "coder.md"))
	assert.FileExists(t, filepath.Join(projectDir, ".opencode", "agents", "reviewer.md"))

	// Verify skills deployed
	assert.FileExists(t, filepath.Join(projectDir, ".opencode", "skills", "shared", "coding.md"))

	// Verify config
	assert.FileExists(t, filepath.Join(projectDir, "opencode.json"))
	data, _ := os.ReadFile(filepath.Join(projectDir, "opencode.json"))
	assert.Contains(t, string(data), "bedrock")
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
			DeployAgents(hubDir), // This succeeds
			failingPhase,         // This fails → rollback
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
			DeployAgents(emptyHub),
			DeploySkills(emptyHub),
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
	content := string(data)
	assert.Contains(t, content, `"custom"`)    // preserved
	assert.Contains(t, content, `"bedrock"`)   // added
}
