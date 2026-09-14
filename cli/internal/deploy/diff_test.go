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

	// Hub agent referencing a skill
	agentsDir := filepath.Join(hubDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))
	agentContent := "---\nnative_skills:\n  - frontend/react-patterns\n---\n# Dev Agent"
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "dev.md"), []byte(agentContent), 0o644))

	// Hub skill (category/name.md structure)
	skillsDir := filepath.Join(hubDir, "skills", "frontend")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillsDir, "react-patterns.md"), []byte("# React Patterns"), 0o644))

	report, err := ComputeDiff(hubDir, projectDir, []string{"dev"})
	require.NoError(t, err)

	assert.True(t, report.HasChanges())
	// 1 agent added + 1 skill added
	added, _, _, _ := report.Summary()
	assert.Equal(t, 2, added)
	// Verify skill path uses deployed format
	var skillFound bool
	for _, f := range report.Files {
		if f.RelPath == filepath.Join("skills", "react-patterns", "SKILL.md") {
			skillFound = true
			assert.Equal(t, FileAdded, f.Status)
		}
	}
	assert.True(t, skillFound, "skill diff should use deployed path format")
}

func TestComputeDiff_SkillsUnchangedAfterDeploy(t *testing.T) {
	hubDir := t.TempDir()
	projectDir := t.TempDir()
	skillContent := []byte("# Coding Standards")

	// Hub: agent referencing a skill (minimal frontmatter)
	agentsDir := filepath.Join(hubDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))
	agentContent := []byte("---\nnative_skills:\n  - shared/coding\n---\n# Dev")
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "dev.md"), agentContent, 0o644))

	// Hub: skill source
	require.NoError(t, os.MkdirAll(filepath.Join(hubDir, "skills", "shared"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hubDir, "skills", "shared", "coding.md"), skillContent, 0o644))

	// Deployed skill: .opencode/skills/coding/SKILL.md (same content as hub source)
	deployedSkill := filepath.Join(projectDir, ".opencode", "skills", "coding")
	require.NoError(t, os.MkdirAll(deployedSkill, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deployedSkill, "SKILL.md"), skillContent, 0o644))

	report, err := ComputeDiff(hubDir, projectDir, []string{"dev"})
	require.NoError(t, err)

	// Count only skill diffs (ignore agents which may differ due to assembly)
	var skillAdded, skillRemoved, skillModified, skillUnchanged int
	for _, f := range report.Files {
		if filepath.Dir(filepath.Dir(f.RelPath)) == "skills" || filepath.Dir(f.RelPath) == "skills" {
			switch f.Status {
			case FileAdded:
				skillAdded++
			case FileRemoved:
				skillRemoved++
			case FileModified:
				skillModified++
			case FileUnchanged:
				skillUnchanged++
			}
		}
	}
	assert.Equal(t, 0, skillAdded, "no skill additions expected")
	assert.Equal(t, 0, skillRemoved, "no skill removals expected")
	assert.Equal(t, 0, skillModified, "no skill modifications expected")
	assert.Equal(t, 1, skillUnchanged, "skill should be unchanged")
}

func TestComputeDiff_SkillsModified(t *testing.T) {
	hubDir := t.TempDir()
	projectDir := t.TempDir()

	// Hub: agent + skill
	agentsDir := filepath.Join(hubDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))
	agentContent := []byte("---\nnative_skills:\n  - developer/security\n---\n# Dev")
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "dev.md"), agentContent, 0o644))

	require.NoError(t, os.MkdirAll(filepath.Join(hubDir, "skills", "developer"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hubDir, "skills", "developer", "security.md"), []byte("# Security v2"), 0o644))

	// Deployed: old version of skill
	deployedSkill := filepath.Join(projectDir, ".opencode", "skills", "security")
	require.NoError(t, os.MkdirAll(deployedSkill, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(deployedSkill, "SKILL.md"), []byte("# Security v1"), 0o644))

	report, err := ComputeDiff(hubDir, projectDir, []string{"dev"})
	require.NoError(t, err)

	// Count only skill diffs
	var skillModified int
	for _, f := range report.Files {
		if filepath.Dir(filepath.Dir(f.RelPath)) == "skills" || filepath.Dir(f.RelPath) == "skills" {
			if f.Status == FileModified {
				skillModified++
			}
		}
	}
	assert.Equal(t, 1, skillModified, "skill should be modified")
}

func TestComputeDiff_UnreferencedSkillsIgnored(t *testing.T) {
	hubDir := t.TempDir()
	projectDir := t.TempDir()

	// Hub: agent with NO native_skills
	agentsDir := filepath.Join(hubDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentsDir, "dev.md"), []byte("# Dev Agent"), 0o644))

	// Hub: 3 skills that are NOT referenced by any agent
	for _, cat := range []string{"shared", "developer", "reviewer"} {
		dir := filepath.Join(hubDir, "skills", cat)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "unused.md"), []byte("# Unused"), 0o644))
	}

	report, err := ComputeDiff(hubDir, projectDir, []string{"dev"})
	require.NoError(t, err)

	// Only the agent should appear as added, skills should be ignored
	added, _, _, _ := report.Summary()
	assert.Equal(t, 1, added, "only agent should be added, unreferenced skills ignored")
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
