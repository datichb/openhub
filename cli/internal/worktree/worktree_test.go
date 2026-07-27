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
	runGit(t, repoDir, "init", "-b", "main")
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

// ─────────────────────────────────────────────────────────────────────────────
// EnsureWorktreeConfig tests
// ─────────────────────────────────────────────────────────────────────────────

// setupMainProject creates a temporary project directory with a minimal
// .opencode/ layout, an opencode.json root config, and optionally .beads/.
func setupMainProject(t *testing.T, withBeads bool) string {
	t.Helper()
	projectPath := t.TempDir()

	// Create .opencode/ with the standard sub-directories and files
	opencodePath := filepath.Join(projectPath, ".opencode")
	for _, sub := range []string{"agents", "skills", "servers"} {
		require.NoError(t, os.MkdirAll(filepath.Join(opencodePath, sub), 0o755))
		// Put a sentinel file so we can verify it's reachable via symlink
		require.NoError(t, os.WriteFile(
			filepath.Join(opencodePath, sub, "sentinel.txt"),
			[]byte(sub), 0o644))
	}
	require.NoError(t, os.WriteFile(
		filepath.Join(opencodePath, "opencode.json"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(opencodePath, "package.json"), []byte(`{}`), 0o644))

	// Root-level opencode.json
	require.NoError(t, os.WriteFile(
		filepath.Join(projectPath, "opencode.json"), []byte(`{}`), 0o644))

	if withBeads {
		require.NoError(t, os.MkdirAll(filepath.Join(projectPath, ".beads"), 0o755))
	}

	return projectPath
}

func TestEnsureWorktreeConfig_NotDeployed(t *testing.T) {
	projectPath := t.TempDir() // no .opencode/ — not deployed
	wtPath := t.TempDir()

	err := EnsureWorktreeConfig(wtPath, projectPath)
	assert.ErrorIs(t, err, ErrProjectNotDeployed)
}

func TestEnsureWorktreeConfig_CreatesRealDirectory(t *testing.T) {
	projectPath := setupMainProject(t, false)
	wtPath := t.TempDir()

	require.NoError(t, EnsureWorktreeConfig(wtPath, projectPath))

	wtOpencode := filepath.Join(wtPath, ".opencode")

	// .opencode must be a real directory, NOT a symlink
	fi, err := os.Lstat(wtOpencode)
	require.NoError(t, err)
	assert.False(t, fi.Mode()&os.ModeSymlink != 0,
		".opencode/ must be a real directory, not a symlink")
	assert.True(t, fi.IsDir())
}

func TestEnsureWorktreeConfig_SubdirSymlinks(t *testing.T) {
	projectPath := setupMainProject(t, false)
	wtPath := t.TempDir()

	require.NoError(t, EnsureWorktreeConfig(wtPath, projectPath))

	wtOpencode := filepath.Join(wtPath, ".opencode")

	// agents/, skills/, servers/ must be symlinks pointing into the main project
	for _, sub := range []string{"agents", "skills", "servers"} {
		linkPath := filepath.Join(wtOpencode, sub)

		fi, err := os.Lstat(linkPath)
		require.NoError(t, err, "expected %s to exist", linkPath)
		assert.True(t, fi.Mode()&os.ModeSymlink != 0,
			".opencode/%s must be a symlink", sub)

		// The sentinel file must be reachable through the symlink
		sentinel := filepath.Join(linkPath, "sentinel.txt")
		data, err := os.ReadFile(sentinel)
		require.NoError(t, err, "sentinel file must be reachable via symlink for %s", sub)
		assert.Equal(t, sub, string(data))
	}
}

func TestEnsureWorktreeConfig_ConfigFilesCopied(t *testing.T) {
	projectPath := setupMainProject(t, false)
	wtPath := t.TempDir()

	require.NoError(t, EnsureWorktreeConfig(wtPath, projectPath))

	wtOpencode := filepath.Join(wtPath, ".opencode")

	// opencode.json and package.json must be regular files (copies), not symlinks
	for _, fname := range []string{"opencode.json", "package.json"} {
		dst := filepath.Join(wtOpencode, fname)
		fi, err := os.Lstat(dst)
		require.NoError(t, err, "expected %s to exist", dst)
		assert.False(t, fi.Mode()&os.ModeSymlink != 0,
			".opencode/%s must be a copied file, not a symlink", fname)
	}
}

func TestEnsureWorktreeConfig_RootOpencodeJsonSymlinked(t *testing.T) {
	projectPath := setupMainProject(t, false)
	wtPath := t.TempDir()

	require.NoError(t, EnsureWorktreeConfig(wtPath, projectPath))

	link := filepath.Join(wtPath, "opencode.json")
	fi, err := os.Lstat(link)
	require.NoError(t, err)
	assert.True(t, fi.Mode()&os.ModeSymlink != 0,
		"root opencode.json must be a symlink")
}

func TestEnsureWorktreeConfig_BeadsSymlinkedWhenPresent(t *testing.T) {
	projectPath := setupMainProject(t, true /* withBeads */)
	wtPath := t.TempDir()

	require.NoError(t, EnsureWorktreeConfig(wtPath, projectPath))

	link := filepath.Join(wtPath, ".beads")
	fi, err := os.Lstat(link)
	require.NoError(t, err)
	assert.True(t, fi.Mode()&os.ModeSymlink != 0, ".beads must be a symlink")
}

func TestEnsureWorktreeConfig_BeadsSkippedWhenAbsent(t *testing.T) {
	projectPath := setupMainProject(t, false /* no .beads */)
	wtPath := t.TempDir()

	require.NoError(t, EnsureWorktreeConfig(wtPath, projectPath))

	link := filepath.Join(wtPath, ".beads")
	_, err := os.Lstat(link)
	assert.True(t, os.IsNotExist(err), ".beads must not be created when absent in main project")
}

func TestEnsureWorktreeConfig_Idempotent(t *testing.T) {
	projectPath := setupMainProject(t, true)
	wtPath := t.TempDir()

	// Calling twice must not return an error
	require.NoError(t, EnsureWorktreeConfig(wtPath, projectPath))
	require.NoError(t, EnsureWorktreeConfig(wtPath, projectPath))
}

func TestEnsureWorktreeConfig_MigratesLegacySymlink(t *testing.T) {
	projectPath := setupMainProject(t, false)
	wtPath := t.TempDir()

	// Simulate legacy layout: .opencode is a symlink to the whole directory
	legacyLink := filepath.Join(wtPath, ".opencode")
	relProject, _ := filepath.Rel(wtPath, projectPath)
	require.NoError(t, os.Symlink(
		filepath.Join(relProject, ".opencode"), legacyLink))

	// Verify it's currently a symlink
	fi, _ := os.Lstat(legacyLink)
	require.True(t, fi.Mode()&os.ModeSymlink != 0, "precondition: must start as symlink")

	// EnsureWorktreeConfig must migrate it to a real directory
	require.NoError(t, EnsureWorktreeConfig(wtPath, projectPath))

	fi, err := os.Lstat(legacyLink)
	require.NoError(t, err)
	assert.False(t, fi.Mode()&os.ModeSymlink != 0,
		"after migration .opencode must be a real directory, not a symlink")
	assert.True(t, fi.IsDir())
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

