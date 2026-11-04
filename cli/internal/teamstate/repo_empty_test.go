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

// A repository nobody has pushed to has no commit: the clone is possible, the
// pull has nothing to fetch (git calls it an error), and the first commit of
// the team (the structure, then the member) is pushed with `push -u`.
func TestTeamStateOnAnEmptyRemoteIsCreated(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare := gittest.EmptyBare(t)
	r := NewRepo(bare, filepath.Join(t.TempDir(), "clone"))

	require.NoError(t, r.EnsureReady(ctx), "the clone of an empty repository")
	assert.False(t, r.HasCommits(ctx), "nothing was ever committed")
	require.NoError(t, r.Pull(ctx), "an empty remote has nothing to pull; it is not an error")

	require.NoError(t, r.EnsureStructure(ctx))
	require.NoError(t, r.AddMember(ctx, Member{ID: "alice", DisplayName: "Alice", Role: "lead"}))

	assert.True(t, r.HasCommits(ctx))
	files := gittest.Run(t, bare, "ls-tree", "-r", "--name-only", "main")
	assert.Contains(t, files, "members.toml")
	assert.Contains(t, files, "catalog/agents/.gitkeep", "the first commit of the structure reached the remote")
	assert.Equal(t, "members: add alice", gittest.Run(t, bare, "log", "-1", "--format=%s", "main"))
	assert.Empty(t, gittest.Run(t, r.Path(), "status", "--porcelain", "--untracked-files=all"))

	// a colleague clones it now: the team exists
	other := NewRepo(bare, filepath.Join(t.TempDir(), "other"))
	require.NoError(t, other.EnsureReady(ctx))
	assert.True(t, other.HasCommits(ctx))
	assert.True(t, other.HasMember(ctx, "alice"))
}

// An error that is not an empty remote stays an error.
func TestPullOfAnUnreachableRemoteStillFails(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare := gittest.EmptyBare(t)
	r := NewRepo(bare, filepath.Join(t.TempDir(), "clone"))
	require.NoError(t, r.Clone(ctx))
	require.NoError(t, os.RemoveAll(bare))

	err := r.Pull(ctx)
	require.Error(t, err, "a remote that cannot be read is not an empty one")
}

// Two people create the team on the same empty repository: the second push is
// rejected, the commit that has no parent is dropped and rebuilt on top of the
// other one; neither is lost.
func TestFirstPushOfAnEmptyRemoteOvertakenIsRetried(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	bare := gittest.EmptyBare(t)
	mine := NewRepo(bare, filepath.Join(t.TempDir(), "mine"))
	theirs := NewRepo(bare, filepath.Join(t.TempDir(), "theirs"))
	require.NoError(t, mine.EnsureReady(ctx))
	require.NoError(t, theirs.EnsureReady(ctx))

	mine.beforePush = func() {
		mine.beforePush = nil
		require.NoError(t, theirs.AddMember(ctx, Member{ID: "bob", DisplayName: "Bob"}))
	}
	require.NoError(t, mine.AddMember(ctx, Member{ID: "alice", DisplayName: "Alice"}))

	check := NewRepo(bare, filepath.Join(t.TempDir(), "check"))
	require.NoError(t, check.EnsureReady(ctx))
	members, err := check.ListMembers(ctx)
	require.NoError(t, err)
	var ids []string
	for _, m := range members {
		ids = append(ids, m.ID)
	}
	assert.ElementsMatch(t, []string{"alice", "bob"}, ids)
	log := gittest.Run(t, bare, "log", "--format=%s", "main")
	assert.ElementsMatch(t, []string{"members: add alice", "members: add bob"}, strings.Split(log, "\n"), "one commit each")
}
