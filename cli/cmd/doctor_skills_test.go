package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
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

// A ~/.oh/skills that cannot be read is a warning that names the folder and
// the cause, never "none"; a folder that does not exist has none.
func TestLegacySkillsCheckWarnsWhenTheFolderCannotBeRead(t *testing.T) {
	i18ntest.Set(t, "en")
	home := t.TempDir()
	t.Setenv("OH_HOME", home)

	if c := legacySkillsCheck(); !c.OK || c.Warn || !strings.Contains(c.Detail, "none") {
		t.Fatalf("no folder: a green «none», got %+v", c)
	}

	// skills is a file: ReadDir fails with something other than "absent".
	if err := os.WriteFile(filepath.Join(home, "skills"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := legacySkillsCheck()
	if !c.OK || !c.Warn {
		t.Fatalf("an unreadable folder warns without failing, got %+v", c)
	}
	if !strings.Contains(c.Detail, filepath.Join(home, "skills")) || !strings.Contains(c.Detail, "nothing was checked") {
		t.Errorf("the warning names the folder and says nothing was checked: %q", c.Detail)
	}
	if strings.Contains(c.Detail, "none in") {
		t.Errorf("never «none» after an error: %q", c.Detail)
	}
}
