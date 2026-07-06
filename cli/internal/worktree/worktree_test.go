package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

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

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")

	// Create a branch that is merged (no new commits)
	runGit(t, repoDir, "branch", "already-merged")

	merged, err := IsMerged(repoDir, "already-merged", "main")
	require.NoError(t, err)
	assert.True(t, merged)

	// Create a branch with new commits (not merged)
	runGit(t, repoDir, "checkout", "-b", "not-merged")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "new work")
	runGit(t, repoDir, "checkout", "main")

	merged, err = IsMerged(repoDir, "not-merged", "main")
	require.NoError(t, err)
	assert.False(t, merged)
}

func TestDetectBaseBranch_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")

	// Should detect "main"
	base := DetectBaseBranch(repoDir)
	assert.Equal(t, "main", base)
}

func TestList_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial")

	// Initially just the main worktree
	entries, err := List(repoDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "main", entries[0].Branch)

	// Add a worktree
	wtPath := filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+"-test")
	runGit(t, repoDir, "worktree", "add", "-b", "test", wtPath)
	defer os.RemoveAll(wtPath)

	entries, err = List(repoDir)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))
}
