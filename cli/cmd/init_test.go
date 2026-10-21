package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// detectBranchPatternHeuristic tests
// ─────────────────────────────────────────────────────────────────────────────

// setupGitRepo creates a temp git repo with an initial commit and the given branches.
//
// The repository is a copy of one built once, and a branch is the file git
// writes for it (refs/heads/<name> holding the commit): no git command.
func setupGitRepo(t *testing.T, branches ...string) string {
	t.Helper()
	dir := t.TempDir()
	copyTree(t, gitRepoTemplate(t), dir)
	head, err := os.ReadFile(filepath.Join(dir, ".git", "refs", "heads", "main"))
	require.NoError(t, err)
	for _, b := range branches {
		ref := filepath.Join(dir, ".git", "refs", "heads", filepath.FromSlash(b))
		require.NoError(t, os.MkdirAll(filepath.Dir(ref), 0o755))
		require.NoError(t, os.WriteFile(ref, head, 0o644))
	}
	return dir
}

var gitRepoTemplates templates

// gitRepoTemplate is the repository setupGitRepo copies. It lives as long as
// the test t that asked for it: a test with subtests asks for it first, so
// that the subtests share it.
func gitRepoTemplate(t *testing.T) string {
	t.Helper()
	return gitRepoTemplates.get(t, "repo", func(dir string) {
		gittest.Run(t, dir, "init", "-q", "-b", "main")
		gittest.Commit(t, dir, "README.md", "init\n", "init")
	})
}

func TestDetectBranchPatternHeuristic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping git-dependent test in short mode")
	}
	gitRepoTemplate(t)

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
