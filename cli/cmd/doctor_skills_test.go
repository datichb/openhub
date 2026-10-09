package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-051: packages of the former community registry are reported, not deleted.
func TestLegacySkillsCheck(t *testing.T) {
	home := t.TempDir()
	t.Setenv("OH_HOME", home)

	if c := legacySkillsCheck(); !c.OK || c.Warn {
		t.Fatalf("no package: %+v", c)
	}

	dir := filepath.Join(home, "skills")
	for _, name := range []string{"zeta", "alpha"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "manifest.json"), []byte(`{"name":"`+name+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "not-a-package"), 0o755); err != nil {
		t.Fatal(err)
	}

	c := legacySkillsCheck()
	if !c.OK || !c.Warn {
		t.Fatalf("packages left: warning expected, got %+v", c)
	}
	if !strings.Contains(c.Detail, "alpha, zeta") || strings.Contains(c.Detail, "not-a-package") {
		t.Errorf("detail = %q", c.Detail)
	}
	if _, err := os.Stat(filepath.Join(dir, "alpha")); err != nil {
		t.Error("nothing is deleted")
	}
}
