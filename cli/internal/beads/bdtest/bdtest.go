// Package bdtest helps the tests that run the real bd.
//
// bd init starts the embedded Dolt, which takes seconds; the tests that
// need the real binary are therefore left out of go test -short, and run in
// the full suite when bd is installed (RequireRealBd). The ones that only
// need a repository already initialized share a single bd init per test
// binary (InitedRepo).
package bdtest

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// RequireRealBd skips the test under go test -short, and when bd is not
// installed; else it returns the path of bd.
func RequireRealBd(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("real bd: skipped by -short (bd init starts Dolt)")
	}
	path, err := exec.LookPath("bd")
	if err != nil {
		t.Skip("real bd: bd is not installed")
	}
	return path
}

// prefix is the ticket prefix of the repositories InitedRepo returns.
const prefix = "bt"

var model struct {
	once sync.Once
	dir  string
	err  error
}

// InitedRepo returns a new git repository, without commit, where
// bd init --prefix bt --skip-hooks --skip-agents has run: a copy of a model
// initialized once per test binary, so that a test that is not about bd init
// does not pay for it. The test must call RequireRealBd first. Call Cleanup
// at the end of the TestMain of the package to remove the model.
func InitedRepo(t testing.TB) string {
	t.Helper()
	model.once.Do(func() { model.dir, model.err = buildModel() })
	if model.err != nil {
		t.Fatalf("bdtest: model repository: %v", model.err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := copyTree(model.dir, dir); err != nil {
		t.Fatalf("bdtest: copy of the model repository: %v", err)
	}
	return dir
}

// Cleanup removes the model of InitedRepo, if one was made.
func Cleanup() {
	if model.dir != "" {
		_ = os.RemoveAll(model.dir)
	}
}

func buildModel() (string, error) {
	dir, err := os.MkdirTemp("", "oh-bdtest-")
	if err != nil {
		return "", err
	}
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		return "", err
	}
	for _, args := range [][]string{{"git", "init", "-q"}, {"bd", "init", "--quiet", "--prefix", prefix, "--skip-hooks", "--skip-agents"}} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "BEADS_DIR=")
		if out, err := cmd.CombinedOutput(); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("%v: %w\n%s", args, err, out)
		}
	}
	return dir, nil
}

// copyTree copies the folder src into the existing folder dst, modes and
// symbolic links included.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		to := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			if rel == "." {
				return nil
			}
			return os.Mkdir(to, info.Mode().Perm()|0o700)
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(target, to)
		case info.Mode().IsRegular():
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, in); err != nil {
				out.Close()
				return err
			}
			return out.Close()
		}
		return nil
	})
}
