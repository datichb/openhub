package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// readRepo is a repository on main with a branch feat/x (one more file).
func readRepo(t *testing.T) string {
	t.Helper()
	repo := gittest.Repo(t)
	gittest.Run(t, repo, "switch", "-q", "-c", "feat/x")
	gittest.Commit(t, repo, "only-on-x.txt", "x\n", "x")
	gittest.Run(t, repo, "switch", "-q", "main")
	return repo
}

func TestResolveReadTarget(t *testing.T) {
	repo := readRepo(t)

	got, err := ResolveReadTarget(repo, "feat/x")
	require.NoError(t, err)
	assert.Equal(t, ReadTarget{Ref: "refs/heads/feat/x"}, got, "a local branch that is not the current one")
	assert.Equal(t, "feat/x", got.Name())

	got, err = ResolveReadTarget(repo, "main")
	require.NoError(t, err)
	assert.True(t, got.Current, "the branch checked out in the project")

	_, err = ResolveReadTarget(repo, "nope")
	assert.ErrorIs(t, err, ErrBranchNotFound)
	for _, bad := range []string{"", "-x", "a..b"} {
		_, err = ResolveReadTarget(repo, bad)
		assert.ErrorIs(t, err, ErrBranchNotFound, bad)
	}

	// Only a remote has it: the remote-tracking branch is read, origin first.
	_, clone := gittest.BareClone(t)
	gittest.Run(t, clone, "switch", "-q", "-c", "feat/y")
	gittest.Commit(t, clone, "y.txt", "y\n", "y")
	gittest.Run(t, clone, "push", "-q", "origin", "feat/y")
	gittest.Run(t, clone, "switch", "-q", "main")
	gittest.Run(t, clone, "branch", "-q", "-D", "feat/y")
	got, err = ResolveReadTarget(clone, "feat/y")
	require.NoError(t, err)
	assert.Equal(t, ReadTarget{Ref: "refs/remotes/origin/feat/y"}, got)
	assert.Equal(t, "origin/feat/y", got.Name())
}

func TestResolveReadTargetDoesNotConfuseABranchWithTheEndOfAnother(t *testing.T) {
	_, clone := gittest.BareClone(t)
	gittest.Run(t, clone, "switch", "-q", "-c", "feat/y")
	gittest.Run(t, clone, "push", "-q", "origin", "feat/y")
	gittest.Run(t, clone, "switch", "-q", "main")
	_, err := ResolveReadTarget(clone, "y")
	assert.ErrorIs(t, err, ErrBranchNotFound, "origin/feat/y is not origin/y")
}

func TestAddDetachedAndRemove(t *testing.T) {
	repo := readRepo(t)
	root := filepath.Join(t.TempDir(), "worktrees")
	path := NewTempPath(root, "proj-1", "feat/x")
	assert.True(t, IsTemp(root, path))
	assert.Equal(t, filepath.Join(root, "proj-1"), filepath.Dir(path))
	assert.NotEqual(t, path, NewTempPath(root, "proj-1", "feat/x"), "one folder per launch")

	require.NoError(t, AddDetached(repo, path, "refs/heads/feat/x"))
	got, err := os.ReadFile(filepath.Join(path, "only-on-x.txt"))
	require.NoError(t, err)
	assert.Equal(t, "x\n", string(got), "the files of the reviewed branch")
	_, err = os.Stat(filepath.Join(repo, "only-on-x.txt"))
	assert.True(t, os.IsNotExist(err), "the project keeps its own branch")
	cur, _ := CurrentBranch(repo)
	assert.Equal(t, "main", cur)
	assert.Empty(t, gittest.Run(t, path, "branch", "--show-current"), "detached")
	want, _ := filepath.EvalSymlinks(repo)
	got2, _ := filepath.EvalSymlinks(RepoOf(path))
	assert.Equal(t, want, got2)

	require.NoError(t, RemoveDetached(path))
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
	assert.NotContains(t, gittest.Run(t, repo, "worktree", "list", "--porcelain"), "only-on-x")
	assert.True(t, BranchExists(repo, "feat/x"), "the reviewed branch stays")
	require.NoError(t, RemoveDetached(path), "already gone")
}

func TestRemoveDetachedWithoutItsRepository(t *testing.T) {
	repo := readRepo(t)
	path := filepath.Join(t.TempDir(), "w", "p")
	require.NoError(t, AddDetached(repo, path, "refs/heads/feat/x"))
	require.NoError(t, os.RemoveAll(repo))
	require.NoError(t, RemoveDetached(path), "the folder goes even if the repository is gone")
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestIsTemp(t *testing.T) {
	root := "/hub/worktrees"
	assert.True(t, IsTemp(root, "/hub/worktrees/p1/feat-x-ab12"))
	for _, p := range []string{"", "/hub/worktrees", "/hub/worktrees/p1", "/hub/worktrees/p1/a/b", "/hub/other/p1/a", "/hub/worktrees/../x/a", "/hub/worktrees-x/p1/a", "/repo"} {
		assert.False(t, IsTemp(root, p), p)
	}
	assert.False(t, IsTemp("", "/hub/worktrees/p1/a"))
}
