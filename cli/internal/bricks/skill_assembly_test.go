package bricks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSkillPath_HubFound(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills", "shared")
	_ = os.MkdirAll(skillsDir, 0o755)

	skillContent := "---\nname: test\n---\n\n# Test\n"
	_ = os.WriteFile(filepath.Join(skillsDir, "test-skill.md"), []byte(skillContent), 0o644)

	path, err := resolveSkillPath(filepath.Join(tmp, "skills"), "shared/test-skill")
	if err != nil {
		t.Fatalf("expected hub path to resolve, got error: %v", err)
	}
	if path != filepath.Join(skillsDir, "test-skill.md") {
		t.Errorf("expected %s, got %s", filepath.Join(skillsDir, "test-skill.md"), path)
	}
}

func TestResolveSkillPath_BothMiss(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills")
	_ = os.MkdirAll(skillsDir, 0o755)

	_, err := resolveSkillPath(skillsDir, "nonexistent/skill")
	if err == nil {
		t.Error("expected error when skill not found in hub or community")
	}
}
