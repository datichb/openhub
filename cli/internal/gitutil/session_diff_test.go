package gitutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// A23: the results of a session cover every commit and change since it
// started, untracked files included, without touching the repository.
func TestDiffSince(t *testing.T) {
	dir := t.TempDir()
	gittest.Run(t, dir, "init", "-b", "main")
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	write("a.py", "a\n")
	write("gone.py", "g\n")
	gittest.Run(t, dir, "add", ".")
	gittest.Run(t, dir, "commit", "-m", "init")
	start := gittest.Run(t, dir, "rev-parse", "HEAD")

	write("a.py", "a\nb\n")
	gittest.Run(t, dir, "commit", "-am", "first turn")
	write("tests.py", "t1\nt2\n")
	gittest.Run(t, dir, "add", "tests.py")
	gittest.Run(t, dir, "commit", "-m", "second turn")
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
	assert.Contains(t, gittest.Run(t, dir, "status", "--porcelain"), "?? new.py", "untracked files stay untracked")

	_, err = DiffSince(dir, "0123456789012345678901234567890123456789")
	assert.ErrorIs(t, err, ErrUnknownRef)
	assert.Equal(t, gittest.Run(t, dir, "rev-parse", "HEAD"), BaseRef(dir), "on the base branch: HEAD")
}
