package bdtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	code := m.Run()
	Cleanup()
	os.Exit(code)
}

func TestCopyTree(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "b", "f"), []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("b/f", filepath.Join(src, "a", "link")); err != nil {
		t.Skipf("no symbolic links: %v", err)
	}
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "a", "b", "f"))
	if err != nil || string(got) != "x" {
		t.Fatalf("file: %q, %v", got, err)
	}
	if st, _ := os.Stat(filepath.Join(dst, "a", "b", "f")); st.Mode().Perm() != 0o640 {
		t.Errorf("mode = %v", st.Mode())
	}
	if target, err := os.Readlink(filepath.Join(dst, "a", "link")); err != nil || target != "b/f" {
		t.Fatalf("link: %q, %v", target, err)
	}
}

// Two repositories from the model are independent, and each has the .git
// and .beads of a bd init.
func TestInitedRepo(t *testing.T) {
	RequireRealBd(t)
	a, b := InitedRepo(t), InitedRepo(t)
	if a == b {
		t.Fatal("same directory")
	}
	for _, d := range []string{a, b} {
		for _, rel := range []string{".git", filepath.Join(".beads", "config.yaml")} {
			if _, err := os.Stat(filepath.Join(d, rel)); err != nil {
				t.Errorf("%s: %v", d, err)
			}
		}
	}
	bd := func(dir string, args ...string) string {
		c := exec.Command("bd", args...)
		c.Dir, c.Env = dir, append(os.Environ(), "BEADS_DIR=")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("bd %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	bd(a, "q", "only in a")
	if !strings.Contains(bd(a, "list", "--json"), "only in a") {
		t.Error("the ticket is not in the first repository")
	}
	if strings.Contains(bd(b, "list", "--json"), "only in a") {
		t.Error("the second repository holds the ticket of the first")
	}
}
