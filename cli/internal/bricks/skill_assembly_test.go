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

func TestResolveSkillPath_Missing(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, "skills")
	_ = os.MkdirAll(skillsDir, 0o755)

	_, err := resolveSkillPath(skillsDir, "nonexistent/skill")
	if err == nil {
		t.Error("expected error when skill not found in hub")
	}
}

// ADR-051: a skill installed in ~/.oh/skills (former community registry)
// is not resolved any more.
func TestResolveSkillPath_IgnoresCommunitySkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	pkg := filepath.Join(home, "skills", "golang-idioms")
	_ = os.MkdirAll(pkg, 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "manifest.json"), []byte(`{"name":"golang-idioms"}`), 0o644)
	_ = os.WriteFile(filepath.Join(pkg, "SKILL.md"), []byte("---\nname: golang-idioms\n---\n"), 0o644)

	skillsDir := filepath.Join(t.TempDir(), "skills")
	_ = os.MkdirAll(skillsDir, 0o755)
	if _, err := resolveSkillPath(skillsDir, "golang-idioms"); err == nil {
		t.Error("community skill resolved")
	}
}
