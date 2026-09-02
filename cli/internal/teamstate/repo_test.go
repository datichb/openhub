package teamstate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRepo(t *testing.T) {
	r := NewRepo("git@gitlab.com:team/state.git", "/home/user/.oh/team-state")
	assert.Equal(t, "git@gitlab.com:team/state.git", r.Remote())
	assert.Equal(t, "/home/user/.oh/team-state", r.Path())
}

func TestIsClonedFalse(t *testing.T) {
	r := NewRepo("", t.TempDir())
	assert.False(t, r.IsCloned())
}

func TestIsClonedTrue(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	r := NewRepo("", dir)
	assert.True(t, r.IsCloned())
}

func TestPullNotCloned(t *testing.T) {
	r := NewRepo("", filepath.Join(t.TempDir(), "nonexistent"))
	err := r.Pull(context.Background())
	assert.ErrorIs(t, err, ErrNotCloned)
}

func TestPushNotCloned(t *testing.T) {
	r := NewRepo("", filepath.Join(t.TempDir(), "nonexistent"))
	err := r.Push(context.Background())
	assert.ErrorIs(t, err, ErrNotCloned)
}

func TestCloneIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a bare repo
	bare := t.TempDir()
	gitExec(t, bare, "init", "--bare")

	// Clone it
	clone := filepath.Join(t.TempDir(), "clone")
	r := NewRepo(bare, clone)

	err := r.Clone(context.Background())
	require.NoError(t, err)
	assert.True(t, r.IsCloned())
}

func TestCloneAlreadyCloned(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bare := t.TempDir()
	gitExec(t, bare, "init", "--bare")

	clone := filepath.Join(t.TempDir(), "clone")
	r := NewRepo(bare, clone)

	require.NoError(t, r.Clone(context.Background()))
	// Second clone should be a no-op
	require.NoError(t, r.Clone(context.Background()))
}

func TestEnsureReady(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bare := t.TempDir()
	gitExec(t, bare, "init", "--bare")

	// First call clones
	clone := filepath.Join(t.TempDir(), "clone")
	r := NewRepo(bare, clone)
	require.NoError(t, r.EnsureReady(context.Background()))
	assert.True(t, r.IsCloned())
}

func TestCommitAndPushIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bare := t.TempDir()
	gitExec(t, bare, "init", "--bare")

	clone := filepath.Join(t.TempDir(), "clone")
	gitExec(t, t.TempDir(), "clone", bare, clone)

	// Initial commit (required for push to work)
	require.NoError(t, os.WriteFile(filepath.Join(clone, "README.md"), []byte("init"), 0o644))
	gitExec(t, clone, "add", ".")
	gitExec(t, clone, "commit", "-m", "init")
	gitExec(t, clone, "push")

	r := NewRepo(bare, clone)

	// Write a new file and commit
	require.NoError(t, os.WriteFile(filepath.Join(clone, "test.txt"), []byte("hello"), 0o644))
	err := r.CommitAndPush(context.Background(), "add test file", "test.txt")
	require.NoError(t, err)

	// Verify it reached the bare repo by cloning again
	clone2 := filepath.Join(t.TempDir(), "clone2")
	gitExec(t, t.TempDir(), "clone", bare, clone2)
	data, err := os.ReadFile(filepath.Join(clone2, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestCommitAndPushNothingToCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	bare := t.TempDir()
	gitExec(t, bare, "init", "--bare")

	clone := filepath.Join(t.TempDir(), "clone")
	gitExec(t, t.TempDir(), "clone", bare, clone)
	require.NoError(t, os.WriteFile(filepath.Join(clone, "README.md"), []byte("init"), 0o644))
	gitExec(t, clone, "add", ".")
	gitExec(t, clone, "commit", "-m", "init")
	gitExec(t, clone, "push")

	r := NewRepo(bare, clone)

	// Nothing to commit — should be a no-op
	err := r.CommitAndPush(context.Background(), "no-op", "README.md")
	assert.NoError(t, err)
}

func TestInitStructure(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()

	err := repo.InitStructure(ctx)
	require.NoError(t, err)

	// Verify directories were created
	expectedDirs := []string{"projects", "wiki", "wiki/.pending", "reports"}
	for _, d := range expectedDirs {
		info, err := os.Stat(filepath.Join(repo.path, d))
		require.NoError(t, err, "directory %s should exist", d)
		assert.True(t, info.IsDir())

		// .gitkeep should exist
		_, err = os.Stat(filepath.Join(repo.path, d, ".gitkeep"))
		assert.NoError(t, err, ".gitkeep should exist in %s", d)
	}
}

func gitExec(t *testing.T, dir string, args ...string) {
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
	require.NoError(t, err, "git %v failed: %s", args, string(out))
}

func TestHasConfigFalse(t *testing.T) {
	dir := t.TempDir()
	r := NewRepo("", dir)
	assert.False(t, r.HasConfig())
}

func TestHasConfigTrue(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[notification]\nenabled = false\n"), 0o644))
	r := NewRepo("", dir)
	assert.True(t, r.HasConfig())
}

func TestHasPoliciesFalse(t *testing.T) {
	dir := t.TempDir()
	r := NewRepo("", dir)
	assert.False(t, r.HasPolicies())
}

func TestHasPoliciesTrue(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "policies.toml"), []byte("[policies]\n"), 0o644))
	r := NewRepo("", dir)
	assert.True(t, r.HasPolicies())
}

func TestHasMemberFalse(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "members.toml"), []byte("[members]\n"), 0o644))
	r := NewRepo("", dir)
	assert.False(t, r.HasMember("alice"))
}

func TestHasMemberTrue(t *testing.T) {
	dir := t.TempDir()
	content := "[members.alice]\ndisplay_name = \"Alice\"\nrole = \"dev\"\ndefault_mode = \"semi-auto\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "members.toml"), []byte(content), 0o644))
	r := NewRepo("", dir)
	assert.True(t, r.HasMember("alice"))
	assert.False(t, r.HasMember("bob"))
}

func TestHasMemberNoFile(t *testing.T) {
	dir := t.TempDir()
	r := NewRepo("", dir)
	assert.False(t, r.HasMember("anyone"))
}

// TestGitCloneInvalidURL_FailsFastWithNoPrompt verifies two properties of the
// git() helper that are critical when running in a background TUI goroutine:
//
//  1. GIT_TERMINAL_PROMPT=0 prevents git from blocking indefinitely waiting for
//     interactive credential input when the remote requires authentication.
//  2. The clone fails within a reasonable time budget (well under the 5s context
//     timeout), proving that git does not hang on a prompt.
//
// The test uses a syntactically valid but unreachable HTTPS URL so that git
// attempts a real network dial (which resolves quickly via DNS failure) rather
// than raising an "invalid URL" error before any prompt logic is triggered.
func TestGitCloneInvalidURL_FailsFastWithNoPrompt(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cloneDir := filepath.Join(t.TempDir(), "clone")
	// Use a host that does not exist so the DNS lookup or connection fails fast.
	// GIT_TERMINAL_PROMPT=0 is set inside repo.git(); this test confirms it works.
	r := NewRepo("https://this-host-does-not-exist.invalid/org/repo.git", cloneDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	err := r.Clone(ctx)
	elapsed := time.Since(start)

	require.Error(t, err, "clone of an invalid URL must fail")

	// The context must NOT have expired — git should have returned an error
	// on its own (DNS failure, connection refused, or "terminal prompts disabled")
	// well before the 5-second timeout.
	assert.Nil(t, ctx.Err(),
		"context should not have timed out — got elapsed=%s, ctx.Err=%v", elapsed, ctx.Err())

	// Error must mention the clone operation (not just an opaque exit error)
	assert.Contains(t, err.Error(), "clone", "error should reference the clone operation")
}

func TestWithWriteLock_ContextCancelled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	repo, _ := setupGitTestRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately — context is already done

	called := false
	err := repo.withWriteLock(ctx, func(_ context.Context) error {
		called = true
		return nil
	})

	// pull(ctx) should fail due to cancelled context (git killed by OS signal)
	assert.Error(t, err, "withWriteLock should fail with a cancelled context")
	assert.False(t, called, "fn should not be called when pull fails due to cancelled context")
}
