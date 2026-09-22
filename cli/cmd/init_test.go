package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// detectBranchPatternHeuristic tests
// ─────────────────────────────────────────────────────────────────────────────

// setupGitRepo creates a temp git repo with an initial commit and the given branches.
func setupGitRepo(t *testing.T, branches ...string) string {
	t.Helper()
	dir := t.TempDir()
	testGitCmd(t, dir, "init")
	testGitCmd(t, dir, "config", "user.email", "test@test.com")
	testGitCmd(t, dir, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644))
	testGitCmd(t, dir, "add", ".")
	testGitCmd(t, dir, "commit", "-m", "init")
	for _, b := range branches {
		testGitCmd(t, dir, "branch", b)
	}
	return dir
}

func TestDetectBranchPatternHeuristic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-dependent test in short mode")
	}

	t.Run("no_branches_returns_empty", func(t *testing.T) {
		dir := setupGitRepo(t) // only main/master
		assert.Empty(t, detectBranchPatternHeuristic(dir))
	})

	t.Run("dominant_feat_prefix", func(t *testing.T) {
		dir := setupGitRepo(t,
			"feat/login", "feat/signup", "feat/dashboard",
			"fix/typo",
		)
		// feat=3 out of 4 (75%) > 50% -> "feat/%s"
		assert.Equal(t, "feat/%s", detectBranchPatternHeuristic(dir))
	})

	t.Run("no_dominant_prefix", func(t *testing.T) {
		dir := setupGitRepo(t,
			"feat/a", "fix/b", "chore/c", "docs/d",
		)
		// Each has 25% -> none > 50% -> ""
		assert.Empty(t, detectBranchPatternHeuristic(dir))
	})

	t.Run("exactly_50_percent_not_sufficient", func(t *testing.T) {
		dir := setupGitRepo(t,
			"feat/a", "fix/b",
		)
		// feat=1 out of 2 (50%), condition is count*2 > total (strict)
		// 1*2 = 2, total = 2 -> 2 > 2 is false -> ""
		assert.Empty(t, detectBranchPatternHeuristic(dir))
	})

	t.Run("just_over_50_percent", func(t *testing.T) {
		dir := setupGitRepo(t,
			"feat/a", "feat/b", "fix/c",
		)
		// feat=2 out of 3 (66.7%), 2*2=4 > 3 -> "feat/%s"
		assert.Equal(t, "feat/%s", detectBranchPatternHeuristic(dir))
	})

	t.Run("feature_prefix", func(t *testing.T) {
		dir := setupGitRepo(t,
			"feature/login", "feature/signup", "feature/api",
		)
		assert.Equal(t, "feature/%s", detectBranchPatternHeuristic(dir))
	})

	t.Run("non_git_directory_returns_empty", func(t *testing.T) {
		dir := t.TempDir()
		assert.Empty(t, detectBranchPatternHeuristic(dir))
	})

	t.Run("unknown_prefixes_ignored", func(t *testing.T) {
		dir := setupGitRepo(t,
			"my-branch", "other-work", "experiment",
		)
		// total=3, no known prefix counts -> ""
		assert.Empty(t, detectBranchPatternHeuristic(dir))
	})

	t.Run("main_master_develop_skipped", func(t *testing.T) {
		// These are in the skip map and should not count.
		// Only feat/a counts (total=1), feat=1, 1*2=2 > 1 -> "feat/%s"
		dir := setupGitRepo(t, "feat/a")
		assert.Equal(t, "feat/%s", detectBranchPatternHeuristic(dir))
	})

	t.Run("nonexistent_directory", func(t *testing.T) {
		assert.Empty(t, detectBranchPatternHeuristic("/nonexistent/path"))
	})
}
