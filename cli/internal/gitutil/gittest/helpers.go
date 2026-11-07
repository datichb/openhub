package gittest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitremote"
)

var (
	configsMu sync.Mutex
	configs   = map[testing.TB]string{}
)

// globalConfig is the global git configuration of the commands of t: the one
// Main set up when the tests run under it, else a file with Defaults() in the
// directory of the test. Either way the user's own is never read.
func globalConfig(t testing.TB) string {
	t.Helper()
	if p := os.Getenv("GIT_CONFIG_GLOBAL"); p != "" {
		return p
	}
	configsMu.Lock()
	defer configsMu.Unlock()
	if p, ok := configs[t]; ok {
		return p
	}
	p := filepath.Join(t.TempDir(), "gitconfig")
	if err := os.WriteFile(p, []byte(Defaults()), 0o600); err != nil {
		t.Fatalf("gittest: %v", err)
	}
	configs[t] = p
	t.Cleanup(func() {
		configsMu.Lock()
		delete(configs, t)
		configsMu.Unlock()
	})
	return p
}

// Run runs `git args...` in dir and returns its standard output, trimmed. It
// fails the test, with the output of the command, if git fails. The identity
// and the defaults are those of Defaults() with or without Main; an identity
// set in the environment of the test (GIT_AUTHOR_NAME…), or in the
// repository, wins over them.
func Run(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+globalConfig(t), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s (in %s): %v\n%s%s", strings.Join(args, " "), dir, err, stdout.String(), stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

// Repo creates a repository on main in a temporary directory, with one
// commit (README.md), and returns its path. It is a copy of a repository built
// once per test binary (see model.go).
func Repo(t testing.TB) string {
	t.Helper()
	m := model(t, "repo", func(dir string) {
		Run(t, dir, "init", "-q", "-b", "main")
		Commit(t, dir, "README.md", "init\n", "init")
	})
	dir := t.TempDir()
	copyTree(t, m, dir)
	return dir
}

// Commit writes file (relative to dir, its folders created) with content and
// commits it with message msg.
func Commit(t testing.TB, dir, file, content, msg string) {
	t.Helper()
	path := filepath.Join(dir, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("gittest: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("gittest: %v", err)
	}
	Run(t, dir, "add", "--", file)
	Run(t, dir, "commit", "-q", "-m", msg)
}

// EmptyBare creates a bare repository on main without any commit, as a git
// server shows a project that nobody has pushed to, and returns its path.
func EmptyBare(t testing.TB) string {
	t.Helper()
	bare := t.TempDir()
	Run(t, bare, "init", "-q", "--bare", "-b", "main")
	return bare
}

// BareClone creates a bare repository and a clone of it, both on main. The
// clone has one commit (README.md) already pushed, and main tracks
// origin/main: it is ready to commit and push. Both are copies of a pair built
// once per test binary (see model.go).
func BareClone(t testing.TB) (bare, clone string) {
	t.Helper()
	m := model(t, "bareclone", func(dir string) {
		modelBare, modelClone := filepath.Join(dir, "bare"), filepath.Join(dir, "clone")
		Run(t, dir, "init", "-q", "--bare", "-b", "main", modelBare)
		Run(t, dir, "init", "-q", "-b", "main", modelClone)
		Run(t, modelClone, "remote", "add", "origin", modelBare)
		Commit(t, modelClone, "README.md", "init\n", "init")
		Run(t, modelClone, "push", "-q", "-u", "origin", "main")
	})
	bare = t.TempDir()
	clone = filepath.Join(t.TempDir(), "clone")
	if err := os.Mkdir(clone, 0o755); err != nil {
		t.Fatalf("gittest: %v", err)
	}
	copyTree(t, filepath.Join(m, "bare"), bare)
	copyTree(t, filepath.Join(m, "clone"), clone)
	// The clone points at the bare repository of the model: at the copy. The
	// URL is read as git wrote it (gitremote decodes the escapes of a Windows
	// path, C:\\Users\\…) and rewritten by git itself.
	from := filepath.Join(m, "bare")
	if have, _ := gitremote.Origin(clone); filepath.Clean(have) != filepath.Clean(from) {
		t.Fatalf("gittest: the clone of the model points at %q, not at %q", have, from)
	}
	Run(t, clone, "remote", "set-url", "origin", bare)
	return bare, clone
}
