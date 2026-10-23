package teamstate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func adoptOp(session string) ClaimOp {
	return ClaimOp{Kind: ClaimOpAdopt, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Session: session}
}

func TestAdoptOperationNeedsASession(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	op := adoptOp("")
	assert.ErrorIs(t, repo.EnqueueClaimOp(op), ErrInvalidClaimOp)
}

// Queuing the adoption cancels what the other sessions had queued on the
// claim; the operations on other claims and of the new session stay.
func TestQueuedAdoptionCancelsTheOperationsOfOtherSessions(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpLabel, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Label: "session-ended"})) // no session
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-2")))
	require.NoError(t, repo.EnqueueClaimOp(adoptOp("ses_2")))

	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	require.Len(t, ops, 2)
	assert.Equal(t, "SRU-2", ops[0].TicketID, "another claim: untouched")
	assert.Equal(t, ClaimOpAdopt, ops[1].Kind)

	// An operation of the previous session queued after the adoption is not added.
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpTouch, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1"}))
	// Those of the new session are.
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpTouch, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_2"}))
	ops, err = repo.ClaimQueue()
	require.NoError(t, err)
	require.Len(t, ops, 3)
	assert.Equal(t, "ses_2", ops[2].Session)

	session, ok := repo.QueuedAdoption("T-SRU", "SRU-1", "benjamin")
	assert.True(t, ok)
	assert.Equal(t, "ses_2", session)
	_, ok = repo.QueuedAdoption("T-SRU", "SRU-2", "benjamin")
	assert.False(t, ok)

	// A later adoption by another session replaces the first.
	require.NoError(t, repo.EnqueueClaimOp(adoptOp("ses_3")))
	session, _ = repo.QueuedAdoption("T-SRU", "SRU-1", "benjamin")
	assert.Equal(t, "ses_3", session)
}

func TestFlushClaimQueueAppliesAnAdoption(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1"})
	require.NoError(t, repo.AddClaimLabel(ctx, "T-SRU", "SRU-1", LabelSessionEnded))
	require.NoError(t, repo.EnqueueClaimOp(adoptOp("ses_2")))
	before := commitCount(t, bare)

	applied, dropped, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	assert.Zero(t, dropped)
	c, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, "ses_2", c.Session)
	assert.Empty(t, c.Labels, "the label of the previous session goes")
	assert.Equal(t, before+1, commitCount(t, bare))

	// Replayed: nothing changes.
	require.NoError(t, repo.EnqueueClaimOp(adoptOp("ses_2")))
	applied, _, err = repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	assert.Equal(t, before+1, commitCount(t, bare), "no commit")

	// Somebody else's claim, or a claim that is gone: dropped.
	mustCreate(t, repo, Claim{TicketID: "SRU-3", ClaimedBy: "alice", Session: "ses_a"})
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpAdopt, Project: "T-SRU", TicketID: "SRU-3", ClaimedBy: "benjamin", Session: "ses_2"}))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpAdopt, Project: "T-SRU", TicketID: "GONE-1", ClaimedBy: "benjamin", Session: "ses_2"}))
	applied, dropped, err = repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Zero(t, applied)
	assert.Equal(t, 2, dropped)
	c, _ = repo.GetClaim("T-SRU", "SRU-3")
	assert.Equal(t, "ses_a", c.Session)
}

func TestAdoptClaimDurableOffline(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1"})
	mustCreate(t, repo, Claim{TicketID: "BOB-1", ClaimedBy: "bob", Session: "ses_b"})
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	online := goOffline(t, repo)

	adopted, queued, err := repo.AdoptClaimDurable(ctx, "T-SRU", "SRU-1", "benjamin", "ses_2")
	require.NoError(t, err)
	assert.False(t, adopted)
	assert.True(t, queued)
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	require.Len(t, ops, 1, "the release of the previous session is cancelled")
	assert.Equal(t, ClaimOpAdopt, ops[0].Kind)

	_, queued, err = repo.AdoptClaimDurable(ctx, "T-SRU", "BOB-1", "benjamin", "ses_2")
	require.NoError(t, err)
	assert.False(t, queued, "somebody else's claim is not adopted")
	_, _, err = repo.AdoptClaimDurable(ctx, "T-SRU", "NOPE-1", "benjamin", "ses_2")
	assert.ErrorIs(t, err, ErrClaimNotFound)

	online()
	applied, _, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	c, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, "ses_2", c.Session)
}

// Online, the adoption is immediate and the operations of the other sessions
// that were waiting in the queue (here without session, so they would apply
// to any claim) are cancelled.
func TestAdoptClaimDurableOnlineCancelsTheQueueOfTheOthers(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "benjamin"})
	mustCreate(t, repo, Claim{TicketID: "SRU-2", ClaimedBy: "benjamin"})
	for _, id := range []string{"SRU-1", "SRU-2"} {
		require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpRelease, Project: "T-SRU", TicketID: id, ClaimedBy: "benjamin"}))
	}

	adopted, queued, err := repo.AdoptClaimDurable(ctx, "T-SRU", "SRU-1", "benjamin", "ses_2")
	require.NoError(t, err)
	assert.True(t, adopted)
	assert.False(t, queued)
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.Equal(t, "SRU-2", ops[0].TicketID)

	_, _, err = repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	c, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, "ses_2", c.Session, "still held")
	_, err = repo.GetClaim("T-SRU", "SRU-2")
	assert.ErrorIs(t, err, ErrClaimNotFound)
}
