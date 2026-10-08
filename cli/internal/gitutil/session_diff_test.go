package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

// A23: the results of a session cover every commit and change since it
// started, untracked files included, without touching the repository.
func TestDiffSince(t *testing.T) {
	dir := t.TempDir()
	gitT(t, dir, "init", "-b", "main")
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	write("a.py", "a\n")
	write("gone.py", "g\n")
	gitT(t, dir, "add", ".")
	gitT(t, dir, "commit", "-m", "init")
	start := gitT(t, dir, "rev-parse", "HEAD")

	write("a.py", "a\nb\n")
	gitT(t, dir, "commit", "-am", "first turn")
	write("tests.py", "t1\nt2\n")
	gitT(t, dir, "add", "tests.py")
	gitT(t, dir, "commit", "-m", "second turn")
	require.NoError(t, os.Remove(filepath.Join(dir, "gone.py")))
	write("new.py", "n\n")

	d, err := DiffSince(dir, start)
	require.NoError(t, err)
	got := map[string]FileChange{}
	for _, f := range d.Files {
		got[f.File] = f
	}
	assert.Equal(t, FileChange{File: "a.py", Status: "modified", Additions: 1}, got["a.py"])
	assert.Equal(t, FileChange{File: "tests.py", Status: "added", Additions: 2}, got["tests.py"])
	assert.Equal(t, FileChange{File: "gone.py", Status: "deleted", Deletions: 1}, got["gone.py"])
	assert.Equal(t, FileChange{File: "new.py", Status: "added", Additions: 1}, got["new.py"])
	assert.Contains(t, d.Patch, "+t2")
	assert.Contains(t, gitT(t, dir, "status", "--porcelain"), "?? new.py", "untracked files stay untracked")

	_, err = DiffSince(dir, "0123456789012345678901234567890123456789")
	assert.ErrorIs(t, err, ErrUnknownRef)
	assert.Equal(t, gitT(t, dir, "rev-parse", "HEAD"), BaseRef(dir), "on the base branch: HEAD")
}
