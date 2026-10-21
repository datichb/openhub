package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func TestSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"feat/bd-42", "feat-bd-42"},
		{"a/b/c", "a-b-c"},
		{"simple", "simple"},
		{"feat/my feature", "feat-my-feature"},
		{"--leading-dashes--", "leading-dashes"},
		{"feat//double", "feat-double"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, Slug(tc.input))
		})
	}
}

func TestSiblingPath(t *testing.T) {
	tests := []struct {
		projectPath string
		branch      string
		expected    string
	}{
		{"/home/user/myrepo", "feat/login", "/home/user/myrepo-feat-login"},
		{"/home/user/myrepo", "main", "/home/user/myrepo-main"},
		{"/home/user/myrepo", "fix/a/b", "/home/user/myrepo-fix-a-b"},
	}

	for _, tc := range tests {
		t.Run(tc.branch, func(t *testing.T) {
			assert.Equal(t, tc.expected, SiblingPath(tc.projectPath, tc.branch))
		})
	}
}

func TestIsGitRepo(t *testing.T) {
	// A temporary directory is NOT a git repo
	tmpDir := t.TempDir()
	assert.False(t, IsGitRepo(tmpDir))
}

func TestResolveOrCreate_BranchValidation(t *testing.T) {
	repoDir := t.TempDir()

	// Branch starting with dash (git flag injection)
	_, err := ResolveOrCreate(repoDir, "--force")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot start with '-'")

	// Branch with path traversal
	_, err = ResolveOrCreate(repoDir, "feat/../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot contain '..'")

	// Empty branch
	_, err = ResolveOrCreate(repoDir, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestResolveOrCreate_Integration tests worktree creation with a real git repo.
func TestResolveOrCreate_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary git repo
	repoDir := gittest.Repo(t)

	// Create a worktree
	wtPath, err := ResolveOrCreate(repoDir, "feat/test-branch")
	require.NoError(t, err)
	assert.DirExists(t, wtPath)
	assert.Equal(t, SiblingPath(repoDir, "feat/test-branch"), wtPath)

	// Calling again should reuse
	wtPath2, err := ResolveOrCreate(repoDir, "feat/test-branch")
	require.NoError(t, err)
	assert.Equal(t, wtPath, wtPath2)

	// Cleanup
	os.RemoveAll(wtPath)
}

func TestIsMerged_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoDir := gittest.Repo(t)

	// A branch with its own commit, merged into main
	gittest.Run(t, repoDir, "checkout", "-q", "-b", "already-merged")
	gittest.Run(t, repoDir, "commit", "--allow-empty", "-m", "work")
	gittest.Run(t, repoDir, "checkout", "-q", "main")
	gittest.Run(t, repoDir, "merge", "-q", "--no-ff", "-m", "merge", "already-merged")

	merged, err := IsMerged(repoDir, "already-merged", "main")
	require.NoError(t, err)
	assert.True(t, merged)

	// Create a branch with new commits (not merged)
	gittest.Run(t, repoDir, "checkout", "-b", "not-merged")
	gittest.Run(t, repoDir, "commit", "--allow-empty", "-m", "new work")
	gittest.Run(t, repoDir, "checkout", "main")

	merged, err = IsMerged(repoDir, "not-merged", "main")
	require.NoError(t, err)
	assert.False(t, merged)

	// A fresh branch (no commit of its own) has nothing merged
	gittest.Run(t, repoDir, "branch", "fresh")
	merged, err = IsMerged(repoDir, "fresh", "main")
	require.NoError(t, err)
	assert.False(t, merged, "a fresh branch is not merged")

	// Unknown base: an error, not "merged"
	_, err = IsMerged(repoDir, "already-merged", "nope")
	assert.Error(t, err)
}

// N16: a branch checked out in a worktree is listed by `git branch --merged`
// with a "+ " prefix, which the old parser missed; a fresh worktree branch was
// cleaned up as merged.
func TestIsMerged_WorktreeBranches(t *testing.T) {
	repoDir := gittest.Repo(t)

	done := filepath.Join(t.TempDir(), "done")
	gittest.Run(t, repoDir, "worktree", "add", "-q", "-b", "feat/done", done)
	gittest.Run(t, done, "commit", "--allow-empty", "-m", "work")
	gittest.Run(t, repoDir, "merge", "-q", "--ff-only", "feat/done")

	fresh := filepath.Join(t.TempDir(), "fresh")
	gittest.Run(t, repoDir, "worktree", "add", "-q", "-b", "feat/fresh", fresh)

	merged, err := IsMerged(repoDir, "feat/done", "main")
	require.NoError(t, err)
	assert.True(t, merged, "a worktree branch fast-forwarded into main is merged")
	merged, err = IsMerged(repoDir, "feat/fresh", "main")
	require.NoError(t, err)
	assert.False(t, merged)

	res, err := CleanupMerged(repoDir, "main", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"feat/done"}, res.Removed)
	assert.NoDirExists(t, done)
	assert.DirExists(t, fresh, "a fresh worktree is kept")
}

func TestDetectBaseBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoDir := gittest.Repo(t)

	// Should detect "main"
	base := DetectBaseBranch(repoDir)
	assert.Equal(t, "main", base)
}

func TestList_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoDir := gittest.Repo(t)

	// Initially just the main worktree
	entries, err := List(repoDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "main", entries[0].Branch)

	// Add a worktree
	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+"-test")
	gittest.Run(t, repoDir, "worktree", "add", "-b", "test", wtPath)
	defer os.RemoveAll(wtPath)

	entries, err = List(repoDir)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

// A18: Stash keeps every change (tracked and untracked) recoverable.
func TestStash(t *testing.T) {
	repoDir := t.TempDir()
	gittest.Run(t, repoDir, "init", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("v1\n"), 0o644))
	gittest.Run(t, repoDir, "add", "a.txt")
	gittest.Run(t, repoDir, "commit", "-m", "initial")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("v2\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "new.txt"), []byte("n\n"), 0o644))
	require.True(t, IsDirty(repoDir))

	ref, err := Stash(repoDir, "oh: ticket")
	require.NoError(t, err)
	assert.Regexp(t, `^[0-9a-f]{40}([0-9a-f]{24})?$`, ref, "the full commit: an abbreviated one made of digits is read as an index")
	assert.False(t, IsDirty(repoDir), "untracked files are stashed too")

	gittest.Run(t, repoDir, "stash", "apply", ref)
	got, err := os.ReadFile(filepath.Join(repoDir, "a.txt"))
	require.NoError(t, err)
	assert.Equal(t, "v2\n", string(got))
	assert.FileExists(t, filepath.Join(repoDir, "new.txt"))
}

// A23: a session starts from HEAD, or from a commit of the uncommitted
// changes found there (nothing stored, the tree untouched).
func TestStartRef(t *testing.T) {
	repoDir := t.TempDir()
	assert.Empty(t, StartRef(repoDir), "outside git")
	gittest.Run(t, repoDir, "init", "-b", "main")
	assert.Empty(t, StartRef(repoDir), "no commit")
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("v1\n"), 0o644))
	gittest.Run(t, repoDir, "add", "a.txt")
	gittest.Run(t, repoDir, "commit", "-m", "initial")
	head := gittest.Run(t, repoDir, "rev-parse", "HEAD")
	assert.Equal(t, head, StartRef(repoDir))

	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "a.txt"), []byte("v2\n"), 0o644))
	ref := StartRef(repoDir)
	assert.NotEqual(t, head, ref)
	assert.Equal(t, "v2", gittest.Run(t, repoDir, "show", ref+":a.txt"))
	assert.True(t, IsDirty(repoDir), "the change stays in the tree")
	assert.Empty(t, gittest.Run(t, repoDir, "stash", "list"), "nothing stashed")
}
