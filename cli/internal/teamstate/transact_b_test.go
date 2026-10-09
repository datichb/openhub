package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// N15 step B: "." is refused; the caller lists its files.
func TestCommitEverythingIsRefused(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	gitIdentity(t)
	ctx := context.Background()
	require.ErrorIs(t, repo.CommitAndPush(ctx, "all", "."), ErrStageAll)
	require.ErrorIs(t, repo.CommitAndPush(ctx, "all", "./"), ErrStageAll)
	err := repo.WithWriteLock(ctx, func(ctx context.Context) error {
		require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "x.txt"), []byte("x"), 0o644))
		return repo.commitAndPush(ctx, "all", ".")
	})
	require.ErrorIs(t, err, ErrStageAll)
}

// N15 step B: a write commits its own files only; another writer's files
// (untracked or modified) stay out of the commit and in place.
func TestWritesCommitTheirFilesOnly(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	gitIdentity(t)
	ctx := context.Background()
	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "other.txt"), []byte("not mine"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "README.md"), []byte("edited by someone"), 0o644))

	require.NoError(t, repo.AddMember(ctx, Member{ID: "alice", DisplayName: "Alice"}))
	assert.Equal(t, "members.toml", gitOut(t, repo.Path(), "show", "--name-only", "--format=", "HEAD"))
	status := gitOut(t, repo.Path(), "status", "--porcelain")
	assert.Contains(t, status, "?? other.txt")
	assert.Contains(t, status, "M README.md")
}

// N15 step B: CommitAndPush is a transaction too: a push overtaken by
// another member is retried on top of it, the caller's file kept.
func TestCommitAndPushRetriesOnTop(t *testing.T) {
	repo, bare := setupGitTestRepo(t)
	gitIdentity(t)
	ctx := context.Background()
	other := NewRepo(bare, filepath.Join(t.TempDir(), "other"))
	require.NoError(t, other.Clone(ctx))
	var calls int
	repo.beforePush = func() {
		repo.beforePush = nil
		require.NoError(t, other.Transact(ctx, counterBuild(&calls, "bob")))
	}
	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "notes.md"), []byte("mine"), 0o644))
	require.NoError(t, repo.CommitAndPush(ctx, "notes", "notes.md"))
	assert.Equal(t, "notes", gitOut(t, repo.Path(), "log", "-1", "--format=%s"))
	assert.Equal(t, "counter 1 by bob", gitOut(t, repo.Path(), "log", "-1", "--skip=1", "--format=%s"))
	assert.Equal(t, gitOut(t, repo.Path(), "rev-parse", "HEAD"), gitOut(t, bare, "rev-parse", "HEAD"))
	got, _ := os.ReadFile(filepath.Join(repo.Path(), "notes.md"))
	assert.Equal(t, "mine", string(got))
}

// N15 step B: inside a transaction, the public methods join it — one
// commit, no second lock nor pull (no deadlock).
func TestPublicMethodsJoinTheTransaction(t *testing.T) {
	repo, bare := setupGitTestRepo(t)
	gitIdentity(t)
	ctx := context.Background()
	base, _ := strconv.Atoi(gitOut(t, repo.Path(), "rev-list", "--count", "HEAD"))
	err := repo.Transact(ctx, func(ctx context.Context, tx *Tx) (TxResult, error) {
		require.NoError(t, repo.Pull(ctx))
		if _, err := repo.CreateClaim(ctx, Claim{TicketID: "T-1", Project: "p", ClaimedBy: "alice"}); err != nil {
			return TxResult{}, err
		}
		if err := repo.UpdateClaimStatus(ctx, "p", "T-1", ClaimStatusReview); err != nil {
			return TxResult{}, err
		}
		if _, err := repo.ListClaims("p"); err != nil { // a read during the build
			return TxResult{}, err
		}
		return TxResult{Message: "batch"}, repo.AppendEvent(ctx, Event{Type: "test", Actor: "alice", Project: "p"})
	})
	require.NoError(t, err)
	n, _ := strconv.Atoi(gitOut(t, repo.Path(), "rev-list", "--count", "HEAD"))
	assert.Equal(t, base+1, n, "one commit")
	assert.Equal(t, "batch", gitOut(t, repo.Path(), "log", "-1", "--format=%s"))
	assert.Equal(t, gitOut(t, repo.Path(), "rev-parse", "HEAD"), gitOut(t, bare, "rev-parse", "HEAD"))
	c, err := repo.GetClaim("p", "T-1")
	require.NoError(t, err)
	assert.Equal(t, ClaimStatusReview, c.Status)
}

// A failed write puts its files back (no half-written state left).
func TestWriteErrorRestoresFiles(t *testing.T) {
	repo, _ := setupGitTestRepo(t)
	gitIdentity(t)
	ctx := context.Background()
	err := repo.WithWriteLock(ctx, func(ctx context.Context) error {
		require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "README.md"), []byte("half"), 0o644))
		require.NoError(t, repo.commitAndPush(ctx, "x", "README.md"))
		return assert.AnError
	})
	require.ErrorIs(t, err, assert.AnError)
	got, _ := os.ReadFile(filepath.Join(repo.Path(), "README.md"))
	assert.Equal(t, "init", string(got))
}
