package cmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/datichb/openhub/cli/internal/testenv"
)

// copyTree copies the tree src into the existing directory dst, modes kept
// (a git repository included: it holds no absolute path unless it has a
// remote). A copy is faster than the git commands that build the same tree.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
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
		t.Fatalf("copying %s: %v", src, err)
	}
}

// templates are directories built once by the first test that asks for one
// (by key) and copied by the tests: copying a repository is faster than the
// git commands that build it. They live as long as the test binary, in the
// folder of testenv (removed with it): the tests of ./cmd run one after the
// other, so a template removed when its last user ends would be built again
// by the next test, which is what it is there to avoid.
type templates struct {
	mu   sync.Mutex
	dirs map[string]string
}

// get returns the directory of the template key, built by build in a new
// directory the first time. A test must only read it.
func (tp *templates) get(t *testing.T, key string, build func(dir string)) string {
	t.Helper()
	tp.mu.Lock()
	defer tp.mu.Unlock()
	if dir, ok := tp.dirs[key]; ok {
		return dir
	}
	// "" when the binary is not isolated: the temporary folder of the system.
	dir, err := os.MkdirTemp(testenv.Dir(), "oh-template-")
	if err != nil {
		t.Fatal(err)
	}
	build(dir)
	if tp.dirs == nil {
		tp.dirs = map[string]string{}
	}
	tp.dirs[key] = dir
	return dir
}
