package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The life of a clone: not cloned, cloned (once), ready, and the commands that
// need a clone refuse to run before it.
func TestRepoLifecycle(t *testing.T) {
	ctx := context.Background()

	t.Run("a folder that is no clone", func(t *testing.T) {
		missing := NewRepo("", filepath.Join(t.TempDir(), "nonexistent"))
		assert.False(t, missing.IsCloned(), "no .git")
		assert.ErrorIs(t, missing.Pull(ctx), ErrNotCloned, "Pull needs a clone")
		assert.ErrorIs(t, missing.Push(ctx), ErrNotCloned, "Push needs a clone")
	})

	t.Run("a folder with a .git is a clone", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
		r := NewRepo("git@gitlab.com:team/state.git", dir)
		assert.True(t, r.IsCloned())
		assert.Equal(t, "git@gitlab.com:team/state.git", r.Remote(), "the remote")
		assert.Equal(t, dir, r.Path(), "the path")
	})

	t.Run("Clone, then Clone again, then EnsureReady", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		bare := t.TempDir()
		gittest.Run(t, bare, "init", "-q", "--bare")

		r := NewRepo(bare, filepath.Join(t.TempDir(), "clone"))
		require.NoError(t, r.Clone(ctx))
		assert.True(t, r.IsCloned(), "cloned")
		require.NoError(t, r.Clone(ctx), "a second Clone is a no-op")

		ready := NewRepo(bare, filepath.Join(t.TempDir(), "clone"))
		require.NoError(t, ready.EnsureReady(ctx), "EnsureReady clones a missing clone")
		assert.True(t, ready.IsCloned())
	})
}

func TestCommitAndPush(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	r, bare := setupGitTestRepo(t)
	head := func() string { return gittest.Run(t, bare, "rev-parse", "main") }

	require.NoError(t, os.WriteFile(filepath.Join(r.Path(), "test.txt"), []byte("hello"), 0o644))
	require.NoError(t, r.CommitAndPush(ctx, "add test file", "test.txt"))
	other := filepath.Join(t.TempDir(), "other")
	gittest.Run(t, filepath.Dir(other), "clone", "-q", bare, other)
	data, err := os.ReadFile(filepath.Join(other, "test.txt"))
	require.NoError(t, err, "the file reached the bare repository")
	assert.Equal(t, "hello", string(data))

	before := head()
	require.NoError(t, r.CommitAndPush(ctx, "no-op", "README.md"), "nothing to commit is not an error")
	assert.Equal(t, before, head(), "and nothing is pushed")
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

func TestRepoHasFiles(t *testing.T) {
	dir := t.TempDir()
	r := NewRepo("", dir)
	assert.False(t, r.HasConfig(), "no config.toml")
	assert.False(t, r.HasPolicies(), "no policies.toml")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[notification]\nenabled = false\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "policies.toml"), []byte("[policies]\n"), 0o644))
	assert.True(t, r.HasConfig(), "config.toml")
	assert.True(t, r.HasPolicies(), "policies.toml")
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
