package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeDiff_AllNew(t *testing.T) {
	// Setup: hub has files, project has nothing
	hubDir := t.TempDir()
	projectDir := t.TempDir()

	// Create hub agents
	agentsDir := filepath.Join(hubDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "dev.md"), []byte("# Dev Agent"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "review.md"), []byte("# Review Agent"), 0o644))

	report, err := ComputeDiff(hubDir, projectDir, nil)
	require.NoError(t, err)

	assert.True(t, report.HasChanges())
	added, modified, removed, unchanged := report.Summary()
	assert.Equal(t, 2, added)
	assert.Equal(t, 0, modified)
	assert.Equal(t, 0, removed)
	assert.Equal(t, 0, unchanged)
}

func TestComputeDiff_Unchanged(t *testing.T) {
	hubDir := t.TempDir()
	projectDir := t.TempDir()

	content := []byte("# Dev Agent\nSame content")

	// Hub
	agentsDir := filepath.Join(hubDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "dev.md"), content, 0o644))

	// Project (deployed)
	deployedDir := filepath.Join(projectDir, ".opencode", "agents")
	require.NoError(t, os.MkdirAll(deployedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deployedDir, "dev.md"), content, 0o644))

	report, err := ComputeDiff(hubDir, projectDir, nil)
	require.NoError(t, err)

	assert.False(t, report.HasChanges())
	_, _, _, unchanged := report.Summary()
	assert.Equal(t, 1, unchanged)
}

func TestComputeDiff_Modified(t *testing.T) {
	hubDir := t.TempDir()
	projectDir := t.TempDir()

	// Hub has updated content
	agentsDir := filepath.Join(hubDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "dev.md"), []byte("# Dev Agent v2"), 0o644))

	// Project has old content
	deployedDir := filepath.Join(projectDir, ".opencode", "agents")
	require.NoError(t, os.MkdirAll(deployedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deployedDir, "dev.md"), []byte("# Dev Agent v1"), 0o644))

	report, err := ComputeDiff(hubDir, projectDir, nil)
	require.NoError(t, err)

	assert.True(t, report.HasChanges())
	_, modified, _, _ := report.Summary()
	assert.Equal(t, 1, modified)
}

func TestComputeDiff_Removed(t *testing.T) {
	hubDir := t.TempDir()
	projectDir := t.TempDir()

	// Hub has no agents
	require.NoError(t, os.MkdirAll(filepath.Join(hubDir, "agents"), 0o755))

	// Project has a deployed agent that no longer exists in hub
	deployedDir := filepath.Join(projectDir, ".opencode", "agents")
	require.NoError(t, os.MkdirAll(deployedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deployedDir, "old.md"), []byte("# Old Agent"), 0o644))

	report, err := ComputeDiff(hubDir, projectDir, nil)
	require.NoError(t, err)

	assert.True(t, report.HasChanges())
	_, _, removed, _ := report.Summary()
	assert.Equal(t, 1, removed)
}

func TestComputeDiff_WithSkills(t *testing.T) {
	hubDir := t.TempDir()
	projectDir := t.TempDir()

	// Hub skills with subdirectory
	skillsDir := filepath.Join(hubDir, "skills", "frontend")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# Frontend Skill"), 0o644))

	report, err := ComputeDiff(hubDir, projectDir, nil)
	require.NoError(t, err)

	assert.True(t, report.HasChanges())
	added, _, _, _ := report.Summary()
	assert.Equal(t, 1, added)
	// Verify path includes skills prefix
	assert.Contains(t, report.Files[0].RelPath, "skills/")
}

func TestComputeDiff_MixedScenario(t *testing.T) {
	hubDir := t.TempDir()
	projectDir := t.TempDir()

	// Hub agents
	agentsDir := filepath.Join(hubDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "dev.md"), []byte("# Dev v2"), 0o644))      // modified
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "new.md"), []byte("# New Agent"), 0o644))   // added
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "same.md"), []byte("# Same Agent"), 0o644)) // unchanged

	// Project deployed agents
	deployedDir := filepath.Join(projectDir, ".opencode", "agents")
	require.NoError(t, os.MkdirAll(deployedDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deployedDir, "dev.md"), []byte("# Dev v1"), 0o644))      // will be modified
	require.NoError(t, os.WriteFile(filepath.Join(deployedDir, "old.md"), []byte("# Old Agent"), 0o644))   // will be "removed"
	require.NoError(t, os.WriteFile(filepath.Join(deployedDir, "same.md"), []byte("# Same Agent"), 0o644)) // unchanged

	report, err := ComputeDiff(hubDir, projectDir, nil)
	require.NoError(t, err)

	assert.True(t, report.HasChanges())
	added, modified, removed, unchanged := report.Summary()
	assert.Equal(t, 1, added)
	assert.Equal(t, 1, modified)
	assert.Equal(t, 1, removed)
	assert.Equal(t, 1, unchanged)
}

func TestFormatDiffReport(t *testing.T) {
	report := &DiffReport{
		Files: []FileDiff{
			{RelPath: "agents/new.md", Status: FileAdded},
			{RelPath: "agents/dev.md", Status: FileModified},
			{RelPath: "agents/old.md", Status: FileRemoved},
			{RelPath: "agents/same.md", Status: FileUnchanged},
		},
	}

	output := FormatDiffReport(report, false)
	assert.Contains(t, output, "+ agents/new.md")
	assert.Contains(t, output, "~ agents/dev.md")
	assert.Contains(t, output, "- agents/old.md")
	assert.NotContains(t, output, "= agents/same.md")
	assert.Contains(t, output, "1 ajouté(s)")
	assert.Contains(t, output, "1 modifié(s)")
	assert.Contains(t, output, "1 supprimé(s)")

	// Verbose mode
	verboseOutput := FormatDiffReport(report, true)
	assert.Contains(t, verboseOutput, "= agents/same.md")
}

func TestFileStatus_String(t *testing.T) {
	assert.Equal(t, "unchanged", FileUnchanged.String())
	assert.Equal(t, "modified", FileModified.String())
	assert.Equal(t, "added", FileAdded.String())
	assert.Equal(t, "removed", FileRemoved.String())
}
