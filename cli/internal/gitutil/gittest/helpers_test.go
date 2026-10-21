package gittest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// The helpers depend on neither the environment Main sets up nor the git
// configuration of the machine: here the global configuration is hostile.
func hostileGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git")
	}
	home := t.TempDir()
	hostile := "[user]\n\tname = someone else\n\temail = else@example.com\n[init]\n\tdefaultBranch = trunk\n[commit]\n\tgpgsign = true\n"
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(hostile), 0o600))
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	for _, k := range []string{"GIT_CONFIG_GLOBAL", "GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "")
		require.NoError(t, os.Unsetenv(k))
	}
}

func TestRepoAndCommit(t *testing.T) {
	hostileGit(t)

	repo := gittest.Repo(t)
	assert.Equal(t, "main", gittest.Run(t, repo, "symbolic-ref", "--short", "HEAD"), "the repository starts on main")
	assert.Equal(t, "1", gittest.Run(t, repo, "rev-list", "--count", "HEAD"), "one initial commit")
	assert.Equal(t, "init", gittest.Run(t, repo, "log", "-1", "--format=%s"), "initial commit message")

	gittest.Commit(t, repo, "docs/guide.md", "hello\n", "add guide")
	assert.Equal(t, "add guide", gittest.Run(t, repo, "log", "-1", "--format=%s"), "commit message")
	assert.Equal(t, "oh tests <tests@oh.invalid>", gittest.Run(t, repo, "log", "-1", "--format=%an <%ae>"), "identity of the tests")
	assert.Equal(t, "docs/guide.md", gittest.Run(t, repo, "show", "--name-only", "--format=", "HEAD"), "file in a new folder committed")
	assert.Empty(t, gittest.Run(t, repo, "status", "--porcelain"), "working tree clean")
	assert.Equal(t, "2", gittest.Run(t, repo, "rev-list", "--count", "HEAD"), "two commits")
}

func TestBareClone(t *testing.T) {
	hostileGit(t)

	bare, clone := gittest.BareClone(t)
	assert.Equal(t, "main", gittest.Run(t, bare, "symbolic-ref", "--short", "HEAD"), "bare on main")
	assert.Equal(t, "main", gittest.Run(t, clone, "symbolic-ref", "--short", "HEAD"), "clone on main")
	assert.Equal(t, gittest.Run(t, clone, "rev-parse", "HEAD"), gittest.Run(t, bare, "rev-parse", "main"), "the initial commit is pushed")
	assert.Equal(t, "origin/main", gittest.Run(t, clone, "rev-parse", "--abbrev-ref", "main@{upstream}"), "main tracks origin/main")

	gittest.Commit(t, clone, "a.txt", "a\n", "a")
	gittest.Run(t, clone, "push", "-q")
	assert.Equal(t, gittest.Run(t, clone, "rev-parse", "HEAD"), gittest.Run(t, bare, "rev-parse", "main"), "a plain push reaches the bare repository")

	_, err := os.Stat(filepath.Join(clone, "README.md"))
	assert.NoError(t, err, "the initial file is in the clone")
}
