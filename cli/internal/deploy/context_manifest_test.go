package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupManifestHub creates a minimal hub with agents and skills for manifest tests.
func setupManifestHub(t *testing.T) (hubDir, projectDir string) {
	t.Helper()

	hubDir = t.TempDir()
	projectDir = t.TempDir()

	// Create hub structure
	require.NoError(t, os.MkdirAll(filepath.Join(hubDir, "agents"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(hubDir, "skills", "coding"), 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(hubDir, "agents", "coder.md"), []byte("# Coder Agent"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(hubDir, "agents", "reviewer.md"), []byte("# Reviewer Agent"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(hubDir, "skills", "coding", "SKILL.md"), []byte("# Coding Skill"), 0o644))

	// Create .opencode dir in project
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".opencode"), 0o755))

	return hubDir, projectDir
}

func TestWriteContextManifest_Basic(t *testing.T) {
	hubDir, projectDir := setupManifestHub(t)

	err := WriteContextManifest(hubDir, projectDir)
	require.NoError(t, err)

	// Read and parse the manifest
	manifestPath := filepath.Join(projectDir, ".opencode", "context-manifest.json")
	assert.FileExists(t, manifestPath)

	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)

	var manifest ContextManifest
	require.NoError(t, json.Unmarshal(data, &manifest))

	// Should have 3 files
	assert.Len(t, manifest.Files, 3)
	assert.Contains(t, manifest.Files, filepath.Join("agents", "coder.md"))
	assert.Contains(t, manifest.Files, filepath.Join("agents", "reviewer.md"))
	assert.Contains(t, manifest.Files, filepath.Join("skills", "coding", "SKILL.md"))

	// Each entry should have a valid SHA-256 (64 hex chars) and a deployed_at
	for relPath, entry := range manifest.Files {
		assert.Len(t, entry.SHA256, 64, "SHA-256 should be 64 hex chars for %s", relPath)
		assert.NotEmpty(t, entry.DeployedAt, "deployed_at should be set for %s", relPath)
	}

	// HubDir recorded
	assert.Equal(t, hubDir, manifest.HubDir)
	assert.NotEmpty(t, manifest.GeneratedAt)
}

func TestWriteContextManifest_EmptyHub(t *testing.T) {
	hubDir := t.TempDir()
	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".opencode"), 0o755))

	// Empty hub — no agents/ or skills/ dirs
	err := WriteContextManifest(hubDir, projectDir)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(projectDir, ".opencode", "context-manifest.json"))
	require.NoError(t, err)

	var manifest ContextManifest
	require.NoError(t, json.Unmarshal(data, &manifest))
	assert.Empty(t, manifest.Files)
}

func TestWriteContextManifest_NoOpenCodeDir(t *testing.T) {
	hubDir, _ := setupManifestHub(t)
	projectDir := t.TempDir() // no .opencode/ created

	err := WriteContextManifest(hubDir, projectDir)
	assert.Error(t, err, "should fail when .opencode/ does not exist")
}

func TestCheckContextFreshness_AllFresh(t *testing.T) {
	hubDir, projectDir := setupManifestHub(t)
	require.NoError(t, WriteContextManifest(hubDir, projectDir))

	report, err := CheckContextFreshness(hubDir, projectDir)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.True(t, report.Fresh)
	assert.Empty(t, report.Stale)
	assert.Empty(t, report.Missing)
	assert.Empty(t, report.New)
}

func TestCheckContextFreshness_StaleFile(t *testing.T) {
	hubDir, projectDir := setupManifestHub(t)
	require.NoError(t, WriteContextManifest(hubDir, projectDir))

	// Modify an agent source file
	require.NoError(t, os.WriteFile(filepath.Join(hubDir, "agents", "coder.md"), []byte("# Coder Agent v2 — MODIFIED"), 0o644))

	report, err := CheckContextFreshness(hubDir, projectDir)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.False(t, report.Fresh)
	assert.Contains(t, report.Stale, filepath.Join("agents", "coder.md"))
	assert.Empty(t, report.Missing)
	assert.Empty(t, report.New)
}

func TestCheckContextFreshness_NewFile(t *testing.T) {
	hubDir, projectDir := setupManifestHub(t)
	require.NoError(t, WriteContextManifest(hubDir, projectDir))

	// Add a new agent
	require.NoError(t, os.WriteFile(filepath.Join(hubDir, "agents", "new-agent.md"), []byte("# New Agent"), 0o644))

	report, err := CheckContextFreshness(hubDir, projectDir)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.False(t, report.Fresh)
	assert.Contains(t, report.New, filepath.Join("agents", "new-agent.md"))
	assert.Empty(t, report.Stale)
	assert.Empty(t, report.Missing)
}

func TestCheckContextFreshness_MissingFile(t *testing.T) {
	hubDir, projectDir := setupManifestHub(t)
	require.NoError(t, WriteContextManifest(hubDir, projectDir))

	// Remove an agent from hub source
	require.NoError(t, os.Remove(filepath.Join(hubDir, "agents", "coder.md")))

	report, err := CheckContextFreshness(hubDir, projectDir)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.False(t, report.Fresh)
	assert.Contains(t, report.Missing, filepath.Join("agents", "coder.md"))
	assert.Empty(t, report.Stale)
}

func TestCheckContextFreshness_NoManifest(t *testing.T) {
	hubDir := t.TempDir()
	projectDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".opencode"), 0o755))

	report, err := CheckContextFreshness(hubDir, projectDir)
	assert.NoError(t, err)
	assert.Nil(t, report, "no manifest should return nil report")
}
