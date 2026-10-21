package fanout

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// The project of a fixture is a copy of one repository prepared once for the
// whole run: copying a directory is faster than the five git commands that
// build it, and every test of the package needs one.
var projectTemplate struct {
	once sync.Once
	dir  string
}

func TestMain(m *testing.M) {
	code := gittest.Main(m)
	if projectTemplate.dir != "" {
		_ = os.RemoveAll(projectTemplate.dir)
	}
	os.Exit(code)
}

// newProject returns a repository on main with README.md and calc.go
// committed, in a directory of its own.
func newProject(t *testing.T) string {
	t.Helper()
	projectTemplate.once.Do(func() {
		dir, err := os.MkdirTemp("", "oh-fanout-project-")
		if err != nil {
			t.Errorf("project template: %v", err)
			return
		}
		projectTemplate.dir = dir
		gittest.Run(t, dir, "init", "-q", "-b", "main")
		gittest.Commit(t, dir, "README.md", "init\n", "init")
		gittest.Commit(t, dir, "calc.go", "package calc\n", "calc")
	})
	if projectTemplate.dir == "" {
		t.Fatal("the project template could not be built")
	}
	dst := t.TempDir()
	err := filepath.WalkDir(projectTemplate.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(projectTemplate.dir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatalf("project copy: %v", err)
	}
	return dst
}
