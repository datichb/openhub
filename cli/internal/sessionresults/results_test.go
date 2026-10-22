package sessionresults

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
)

var at = time.Date(2026, 10, 22, 14, 0, 0, 0, time.UTC)

func result() adapters.SessionResult {
	return adapters.SessionResult{
		SessionID: "ses_a", Title: "Fix auth", Agent: "coder", Branch: "feat/auth", Cost: 0.42,
		TokensIn: 1200, TokensOut: 340, TokensReasoning: 50, TokensCacheRead: 800, TokensCacheWrite: 90,
		Changes: []adapters.FileChange{
			{File: "auth.go", Status: "modified", Additions: 10, Deletions: 2, Patch: "--- a/auth.go\n+++ b/auth.go\n@@ -1 +1 @@\n-old\n+new\n"},
			{File: "README.md", Status: "added", Additions: 5, Patch: "--- /dev/null\n+++ b/README.md\n+hello"},
			{File: "bin.dat", Status: "modified", Additions: 0, Deletions: 0},
		},
	}
}

// What is saved when a session ends is what the recap shows once the tool
// server is gone: the totals, the files and the whole patch.
func TestSaveThenLoadGivesTheRecap(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, Save(dir, result(), at))

	sum, patch, err := Load(dir, "ses_a")
	require.NoError(t, err)
	assert.Equal(t, "ses_a", sum.SessionID)
	assert.Equal(t, "Fix auth", sum.Title)
	assert.Equal(t, "coder", sum.Agent)
	assert.Equal(t, "feat/auth", sum.Branch)
	assert.InDelta(t, 0.42, sum.Cost, 1e-9)
	assert.EqualValues(t, 1200, sum.TokensIn)
	assert.EqualValues(t, 340, sum.TokensOut)
	assert.EqualValues(t, 50, sum.TokensReasoning)
	assert.EqualValues(t, 800, sum.TokensCacheRead)
	assert.EqualValues(t, 90, sum.TokensCacheWrite)
	assert.True(t, sum.CapturedAt.Equal(at))
	assert.Equal(t, 15, sum.Additions, "totals add the files")
	assert.Equal(t, 2, sum.Deletions)
	assert.Equal(t, []FileStat{
		{File: "auth.go", Status: "modified", Additions: 10, Deletions: 2},
		{File: "README.md", Status: "added", Additions: 5},
		{File: "bin.dat", Status: "modified"},
	}, sum.Files, "the summary keeps the files without their patch")

	assert.Equal(t, "--- a/auth.go\n+++ b/auth.go\n@@ -1 +1 @@\n-old\n+new\n--- /dev/null\n+++ b/README.md\n+hello\n", patch,
		"patches are concatenated, one per line end; a file without patch adds nothing")
}

func TestPatchOfNothingIsEmpty(t *testing.T) {
	assert.Empty(t, Patch(nil))
	assert.Empty(t, Patch([]adapters.FileChange{{File: "a"}, {File: "b"}}))
	assert.Equal(t, "p1\np2\n", Patch([]adapters.FileChange{{Patch: "p1"}, {Patch: "p2\n"}}))
}

// A session without result, or one saved without changes, is told apart.
func TestLoadWithoutSnapshot(t *testing.T) {
	dir := t.TempDir()
	_, _, err := Load(dir, "ses_none")
	assert.ErrorIs(t, err, ErrNoSnapshot)
	_, _, err = Load("", "ses_a")
	assert.ErrorIs(t, err, ErrNoSnapshot, "no sessions folder")

	require.NoError(t, Save(dir, adapters.SessionResult{SessionID: "ses_empty"}, at))
	sum, patch, err := Load(dir, "ses_empty")
	require.NoError(t, err)
	assert.Empty(t, sum.Files)
	assert.Empty(t, patch)
	assert.Zero(t, sum.Additions)
}

func TestSaveRefusesWithoutFolderOrSession(t *testing.T) {
	assert.Error(t, Save("", result(), at))
	assert.Error(t, Save(t.TempDir(), adapters.SessionResult{}, at))
	assert.Error(t, SaveSummary("", Summary{SessionID: "x"}, ""))
	assert.Error(t, SaveSummary(t.TempDir(), Summary{}, ""))
}

// A second save replaces the first (the session went on); a damaged summary
// is reported with the session it belongs to instead of an empty recap.
func TestSaveReplacesAndALoadReportsDamage(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, Save(dir, result(), at))
	later := result()
	later.Cost = 1.5
	later.Changes = later.Changes[:1]
	require.NoError(t, Save(dir, later, at.Add(time.Hour)))

	sum, patch, err := Load(dir, "ses_a")
	require.NoError(t, err)
	assert.InDelta(t, 1.5, sum.Cost, 1e-9)
	assert.Len(t, sum.Files, 1)
	assert.NotContains(t, patch, "README.md")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "ses_a", "results", "summary.json"), []byte("{not json"), 0o600))
	_, _, err = Load(dir, "ses_a")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoSnapshot)
	assert.Contains(t, err.Error(), "ses_a")
}

// The recap holds diffs of the user's code: owner-only files.
func TestSnapshotFilesAreOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	dir := t.TempDir()
	require.NoError(t, Save(dir, result(), at))
	for _, f := range []string{"summary.json", "diff.patch"} {
		fi, err := os.Stat(filepath.Join(dir, "ses_a", "results", f))
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm(), f)
	}
	fi, err := os.Stat(filepath.Join(dir, "ses_a", "results"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), fi.Mode().Perm())
}

// An id that is not a simple name never leaves <sessions>/<id>: Load, which
// gets ids typed on the command line, and the saves refuse it and touch
// nothing outside the sessions folder.
func TestIdentifiersThatAreNotAFolderNameAreRefused(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	require.NoError(t, os.MkdirAll(sessions, 0o700))
	// Results of something else, next to the sessions folder.
	require.NoError(t, SaveSummary(root, Summary{SessionID: "other", Agent: "secret"}, "patch"))

	for _, id := range []string{"", ".", "..", "../other", "../../x", "a/b", "a/../b", "/abs", `a\b`, `..\other`, "ses_a\x00"} {
		_, _, err := Load(sessions, id)
		require.ErrorIs(t, err, ErrInvalidID, "Load %q", id)
		assert.NotErrorIs(t, err, ErrNoSnapshot, "Load %q", id)

		r := result()
		r.SessionID = id
		if id != "" {
			require.ErrorIs(t, Save(sessions, r, at), ErrInvalidID, "Save %q", id)
			require.ErrorIs(t, SaveSummary(sessions, Summary{SessionID: id}, ""), ErrInvalidID, "SaveSummary %q", id)
		}
	}
	entries, err := os.ReadDir(sessions)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing was written in the sessions folder")
	assert.NoFileExists(t, filepath.Join(root, "results", "summary.json"))
}

// Names that merely look unusual are still simple names.
func TestUnusualButSimpleIdentifiersAreAccepted(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"ses_a", "ses-b.c", "..a", "a..", "...", "é"} {
		r := result()
		r.SessionID = id
		require.NoError(t, Save(dir, r, at), id)
		sum, _, err := Load(dir, id)
		require.NoError(t, err, id)
		assert.Equal(t, id, sum.SessionID)
	}
}
