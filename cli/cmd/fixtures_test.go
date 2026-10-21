package cmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
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
// (by key), copied by the tests, and removed when the last of them ends:
// copying a repository is faster than the git commands that build it.
type templates struct {
	mu    sync.Mutex
	dirs  map[string]string
	users map[string]int
}

// get returns the directory of the template key, built by build in a new
// directory if no test holds it.
func (tp *templates) get(t *testing.T, key string, build func(dir string)) string {
	t.Helper()
	tp.mu.Lock()
	defer tp.mu.Unlock()
	if tp.dirs == nil {
		tp.dirs, tp.users = map[string]string{}, map[string]int{}
	}
	if tp.users[key] == 0 {
		dir, err := os.MkdirTemp("", "oh-template-")
		if err != nil {
			t.Fatal(err)
		}
		build(dir)
		tp.dirs[key] = dir
	}
	tp.users[key]++
	t.Cleanup(func() {
		tp.mu.Lock()
		defer tp.mu.Unlock()
		if tp.users[key]--; tp.users[key] == 0 {
			_ = os.RemoveAll(tp.dirs[key])
		}
	})
	return tp.dirs[key]
}
