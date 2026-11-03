package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// After team.init the clone of the team-state held a catalog/ folder that
// git did not track. catalog/ is the team brick catalogue (ADR-054), shared
// like projects/ or workflows/: EnsureStructure commits the folders it creates.
func TestEnsureStructureCommitsTheSharedLayout(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	r := NewRepo(bare, clone)

	untracked := func() string { return gittest.Run(t, clone, "status", "--porcelain", "--untracked-files=all") }
	remoteFiles := func() string { return gittest.Run(t, bare, "ls-tree", "-r", "--name-only", "main") }

	require.NoError(t, r.EnsureStructure(ctx))
	assert.Empty(t, untracked(), "nothing is left untracked in the clone")
	files := remoteFiles()
	for _, want := range []string{"catalog/agents/.gitkeep", "catalog/skills/.gitkeep", "projects/.gitkeep", "wiki/.pending/.gitkeep"} {
		assert.Contains(t, files, want, "the layout reached the remote")
	}

	head := gittest.Run(t, bare, "rev-parse", "main")
	require.NoError(t, r.EnsureStructure(ctx))
	assert.Equal(t, head, gittest.Run(t, bare, "rev-parse", "main"), "a second call commits nothing")
}

// The clone of a team created before catalog/ existed: only catalog/ is new.
func TestEnsureStructureCommitsOnlyWhatIsMissing(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	r := NewRepo(bare, clone)
	require.NoError(t, r.EnsureStructure(ctx))
	gittest.Run(t, clone, "rm", "-rq", "catalog")
	gittest.Run(t, clone, "commit", "-q", "-m", "before catalog")
	gittest.Run(t, clone, "push", "-q")

	// what an older oh left: catalog/ on disk, not tracked
	require.NoError(t, r.InitStructure(ctx))
	require.Contains(t, gittest.Run(t, clone, "status", "--porcelain", "--untracked-files=all"), "catalog/", "the bug: untracked catalog/")

	require.NoError(t, r.EnsureStructure(ctx))
	assert.Empty(t, gittest.Run(t, clone, "status", "--porcelain", "--untracked-files=all"))
	last := gittest.Run(t, bare, "show", "--stat", "--format=%s", "main")
	assert.True(t, strings.HasPrefix(last, "team: init structure"), last)
	assert.Contains(t, last, "catalog/agents/.gitkeep")
	assert.NotContains(t, last, "projects/", "the folders that were tracked are not touched")
}

// Without a clone, or without a remote, the layout is created and nothing fails.
func TestEnsureStructureWithoutRemote(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	dir := gittest.Repo(t)
	r := NewRepo("", dir)
	require.NoError(t, r.EnsureStructure(ctx), "a solo team-state keeps its commit local")
	assert.Empty(t, gittest.Run(t, dir, "status", "--porcelain", "--untracked-files=all"))

	plain := NewRepo("", t.TempDir())
	require.NoError(t, plain.EnsureStructure(ctx), "not a git repository: the folders are created, no commit")
}

// A commit of the structure that fails (remote unreachable, lock busy) is
// not lost in a log: EnsureStructure returns a *StructureWarning, which the
// callers show without stopping. The folders exist on disk and the next call
// commits them.
func TestEnsureStructureCommitFailureIsAWarning(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare, clone := gittest.BareClone(t)
	r := NewRepo(bare, clone)

	moved := bare + ".away"
	require.NoError(t, os.Rename(bare, moved))
	err := r.EnsureStructure(ctx)
	require.Error(t, err)
	var warn *StructureWarning
	require.ErrorAs(t, err, &warn)
	assert.True(t, IsStructureWarning(err))
	assert.NotNil(t, warn.Cause)
	assert.False(t, IsPullWarning(err))
	for _, d := range structureDirs {
		assert.FileExists(t, filepath.Join(clone, d, ".gitkeep"), "the layout is on disk anyway")
	}

	// Back online: the next call commits what was left.
	require.NoError(t, os.Rename(moved, bare))
	require.NoError(t, r.EnsureStructure(ctx))
	assert.Empty(t, gittest.Run(t, clone, "status", "--porcelain", "--untracked-files=all"))
	assert.Contains(t, gittest.Run(t, bare, "ls-tree", "-r", "--name-only", "main"), "catalog/agents/.gitkeep")

	// Nothing to commit: no warning, whatever the state of the remote.
	require.NoError(t, os.Rename(bare, moved))
	t.Cleanup(func() { _ = os.Rename(moved, bare) })
	assert.NoError(t, r.EnsureStructure(ctx), "the structure is already there: nothing to say")
}
