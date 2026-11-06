package gittest

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// The repositories of Repo and BareClone are copies of models built once per
// test binary: the git commands that build one (a dozen processes) cost more
// than most of the tests that use it, on a machine where starting a process is
// slow (security agents scan each one). A model lives in the folder of the
// isolated test binary (OH_TESTENV, removed with it by testenv.Isolate) or, for
// the packages that use Main only, in a folder that Main removes when the tests
// are over. A test only reads a model: it works on a copy.
var models struct {
	mu   sync.Mutex
	dirs map[string]string
	root string // the folder that holds them when there is no OH_TESTENV
}

// model returns the folder of the model key, built by build in an empty folder
// the first time.
func model(t testing.TB, key string, build func(dir string)) string {
	t.Helper()
	models.mu.Lock()
	defer models.mu.Unlock()
	if dir, ok := models.dirs[key]; ok {
		return dir
	}
	parent := os.Getenv("OH_TESTENV")
	if parent == "" {
		if models.root == "" {
			root, err := os.MkdirTemp("", "oh-gittest-models-")
			if err != nil {
				t.Fatalf("gittest: %v", err)
			}
			models.root = root
		}
		parent = models.root
	}
	dir, err := os.MkdirTemp(parent, "model-"+filepath.Base(key)+"-")
	if err != nil {
		t.Fatalf("gittest: %v", err)
	}
	build(dir)
	if models.dirs == nil {
		models.dirs = map[string]string{}
	}
	models.dirs[key] = dir
	return dir
}

// removeModels is called by Main when the tests are over.
func removeModels() {
	models.mu.Lock()
	defer models.mu.Unlock()
	for _, dir := range models.dirs {
		_ = os.RemoveAll(dir)
	}
	if models.root != "" {
		_ = os.RemoveAll(models.root)
	}
	models.dirs, models.root = nil, ""
}

// copyTree copies the tree src into the existing folder dst, modes kept.
func copyTree(t testing.TB, src, dst string) {
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
		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, info.Mode().Perm())
		}
	})
	if err != nil {
		t.Fatalf("gittest: copy of %s: %v", src, err)
	}
}
