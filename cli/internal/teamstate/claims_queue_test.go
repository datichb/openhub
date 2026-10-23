package teamstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func goOffline(t *testing.T, repo *Repo) func() {
	t.Helper()
	remote := gittest.Run(t, repo.Path(), "remote", "get-url", "origin")
	gittest.Run(t, repo.Path(), "remote", "set-url", "origin", "http://127.0.0.1:1/x.git")
	return func() { gittest.Run(t, repo.Path(), "remote", "set-url", "origin", remote) }
}

func relOp(ticket string) ClaimOp {
	return ClaimOp{Kind: ClaimOpRelease, Project: "T-SRU", TicketID: ticket, ClaimedBy: "benjamin", Session: "ses_1"}
}

func mustClaim(t *testing.T, repo *Repo, id string) {
	t.Helper()
	mustCreate(t, repo, Claim{TicketID: id, ClaimedBy: "benjamin", Session: "ses_1"})
}

func TestClaimQueueFileFormat(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	at := time.Date(2026, 7, 8, 14, 30, 0, 0, time.UTC)
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{ID: "a", Kind: ClaimOpStatus, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1", Status: "review", QueuedAt: at}))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{ID: "b", Kind: ClaimOpLabel, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Label: "session-ended", QueuedAt: at}))

	data, err := os.ReadFile(filepath.Join(repo.Path(), ".git", "oh-claims-queue.json"))
	require.NoError(t, err)
	want := `{
  "version": 1,
  "ops": [
    {
      "id": "a",
      "kind": "status",
      "project": "T-SRU",
      "ticket": "SRU-1",
      "claimed_by": "benjamin",
      "session": "ses_1",
      "status": "review",
      "queued_at": "2026-07-08T14:30:00Z"
    },
    {
      "id": "b",
      "kind": "label",
      "project": "T-SRU",
      "ticket": "SRU-1",
      "claimed_by": "benjamin",
      "label": "session-ended",
      "queued_at": "2026-07-08T14:30:00Z"
    }
  ]
}
`
	assert.Equal(t, want, string(data))
	assert.Empty(t, gittest.Run(t, repo.Path(), "status", "--porcelain"), "the queue is not versioned")

	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	assert.Len(t, ops, 2)
}

func TestEnqueueClaimOpValidation(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	for name, op := range map[string]ClaimOp{
		"unknown kind":       {Kind: "boom", Project: "P", TicketID: "T-1", ClaimedBy: "a"},
		"no owner":           {Kind: ClaimOpRelease, Project: "P", TicketID: "T-1"},
		"status without one": {Kind: ClaimOpStatus, Project: "P", TicketID: "T-1", ClaimedBy: "a"},
		"label without one":  {Kind: ClaimOpLabel, Project: "P", TicketID: "T-1", ClaimedBy: "a"},
		"bad project":        {Kind: ClaimOpRelease, Project: "../x", TicketID: "T-1", ClaimedBy: "a"},
		"bad ticket":         {Kind: ClaimOpRelease, Project: "P", TicketID: "", ClaimedBy: "a"},
	} {
		assert.ErrorIs(t, repo.EnqueueClaimOp(op), ErrInvalidClaimOp, name)
	}
	assert.NoFileExists(t, filepath.Join(repo.Path(), ".git", "oh-claims-queue.json"))
}

func TestEnqueueClaimOpNotCloned(t *testing.T) {
	t.Parallel()
	repo := NewRepo("", filepath.Join(t.TempDir(), "nope"))
	assert.ErrorIs(t, repo.EnqueueClaimOp(relOp("SRU-1")), ErrNotCloned)
}

func TestEnqueueClaimOpDeduplicates(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	early, late := time.Now().Add(-time.Hour).UTC(), time.Now().UTC()
	touch := func(at time.Time) ClaimOp {
		return ClaimOp{Kind: ClaimOpTouch, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1", QueuedAt: at}
	}
	require.NoError(t, repo.EnqueueClaimOp(touch(early)))
	require.NoError(t, repo.EnqueueClaimOp(touch(late)))
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpLabel, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Label: "a"}))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpLabel, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Label: "b"}))

	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	require.Len(t, ops, 4, "one touch (the latest), one release, two labels")
	kinds := []string{ops[0].Kind, ops[1].Kind, ops[2].Kind, ops[3].Kind}
	assert.Equal(t, []string{ClaimOpTouch, ClaimOpRelease, ClaimOpLabel, ClaimOpLabel}, kinds)
	assert.WithinDuration(t, late, ops[0].QueuedAt, time.Second)
	assert.NotEmpty(t, ops[0].ID)
}

func TestEnqueueClaimOpConcurrent(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	other := NewRepo(repo.Remote(), repo.Path()) // another Repo value on the same clone
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := repo
			if i%2 == 1 {
				r = other
			}
			assert.NoError(t, r.EnqueueClaimOp(relOp(fmt.Sprintf("SRU-%d", i))))
		}()
	}
	wg.Wait()
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	assert.Len(t, ops, 20)
}

func TestFlushClaimQueueEmpty(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	before := commitCount(t, repo.path)
	applied, dropped, err := repo.FlushClaimQueue(context.Background())
	require.NoError(t, err)
	assert.Zero(t, applied+dropped)
	assert.Equal(t, before, commitCount(t, repo.path))
}

func TestFlushClaimQueueOneCommitForSeveralOperations(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	for _, id := range []string{"SRU-1", "SRU-2", "SRU-3", "SRU-4"} {
		mustClaim(t, repo, id)
	}
	before, beforeBare := commitCount(t, repo.path), commitCount(t, bare)

	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpStatus, Project: "T-SRU", TicketID: "SRU-2", ClaimedBy: "benjamin", Session: "ses_1", Status: ClaimStatusReview}))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpLabel, Project: "T-SRU", TicketID: "SRU-2", ClaimedBy: "benjamin", Session: "ses_1", Label: "session-ended"}))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpLabel, Project: "T-SRU", TicketID: "SRU-3", ClaimedBy: "benjamin", Session: "ses_1", Label: "review"}))

	applied, dropped, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, applied)
	assert.Zero(t, dropped)
	assert.Equal(t, before+1, commitCount(t, repo.path), "one commit for the four operations")
	assert.Equal(t, beforeBare+1, commitCount(t, bare), "pushed")
	assert.Equal(t, "claims-queue: benjamin applies 4 operation(s)", gittest.Run(t, repo.path, "log", "-1", "--format=%s"))

	_, err = repo.GetClaim("T-SRU", "SRU-1")
	assert.ErrorIs(t, err, ErrClaimNotFound)
	c2, err := repo.GetClaim("T-SRU", "SRU-2")
	require.NoError(t, err)
	assert.Equal(t, ClaimStatusReview, c2.Status)
	assert.Equal(t, []string{"session-ended"}, c2.Labels)
	assert.False(t, c2.LastActivity.IsZero())
	c3, err := repo.GetClaim("T-SRU", "SRU-3")
	require.NoError(t, err)
	assert.Equal(t, []string{"review"}, c3.Labels)
	c4, err := repo.GetClaim("T-SRU", "SRU-4")
	require.NoError(t, err)
	assert.Empty(t, c4.Labels, "untouched")

	assert.NoFileExists(t, repo.claimQueuePath(), "the queue is empty")
	assert.Empty(t, gittest.Run(t, repo.path, "status", "--porcelain"))
}

func TestFlushClaimQueueStaysQueuedWhileOffline(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	mustClaim(t, repo, "SRU-1")
	mustClaim(t, repo, "SRU-2")
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpLabel, Project: "T-SRU", TicketID: "SRU-2", ClaimedBy: "benjamin", Session: "ses_1", Label: "session-ended"}))
	before, beforeBare := commitCount(t, repo.path), commitCount(t, bare)
	online := goOffline(t, repo)

	applied, dropped, err := repo.FlushClaimQueue(ctx)
	require.ErrorIs(t, err, ErrOffline)
	assert.Zero(t, applied+dropped)
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	assert.Len(t, ops, 2, "the operations stay in the queue")
	assert.Equal(t, before, commitCount(t, repo.path))
	_, err = repo.GetClaim("T-SRU", "SRU-1")
	assert.NoError(t, err, "nothing applied")

	online()
	applied, dropped, err = repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, applied)
	assert.Zero(t, dropped)
	assert.Equal(t, beforeBare+1, commitCount(t, bare))
	assert.NoFileExists(t, repo.claimQueuePath())
	_, err = repo.GetClaim("T-SRU", "SRU-1")
	assert.ErrorIs(t, err, ErrClaimNotFound)
}

func TestFlushClaimQueueStaysQueuedWhenThePushFails(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	mustClaim(t, repo, "SRU-1")
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	before := commitCount(t, repo.path)
	// The pull works, the push does not: a pre-receive hook refuses.
	hook := filepath.Join(bare, "hooks", "pre-receive")
	require.NoError(t, os.WriteFile(hook, []byte("#!/bin/sh\necho refused >&2\nexit 1\n"), 0o755))

	_, _, err := repo.FlushClaimQueue(context.Background())
	require.Error(t, err)
	ops, qerr := repo.ClaimQueue()
	require.NoError(t, qerr)
	assert.Len(t, ops, 1)
	assert.Equal(t, before, commitCount(t, repo.path), "the local commit is dropped")
	_, err = repo.GetClaim("T-SRU", "SRU-1")
	assert.NoError(t, err, "the claim is back")
	assert.Empty(t, gittest.Run(t, repo.path, "status", "--porcelain"))

	require.NoError(t, os.Remove(hook))
	applied, _, err := repo.FlushClaimQueue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
}

func TestFlushClaimQueueDropsWhatNoLongerApplies(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	mustCreate(t, repo, Claim{TicketID: "OWNED", ClaimedBy: "alice", Session: "ses_alice"})   // another member
	mustCreate(t, repo, Claim{TicketID: "OTHERSES", ClaimedBy: "benjamin", Session: "ses_2"}) // same member, another session
	mustCreate(t, repo, Claim{TicketID: "NOSES", ClaimedBy: "benjamin"})                      // claim without session
	mustCreate(t, repo, Claim{TicketID: "TRANS", ClaimedBy: "benjamin", Session: "ses_1"})    // in_progress → done is not allowed
	mustCreate(t, repo, Claim{TicketID: "OK", ClaimedBy: "benjamin", Session: "ses_1"})       // applies
	for _, id := range []string{"OWNED", "OTHERSES", "NOSES", "GONE"} {
		require.NoError(t, repo.EnqueueClaimOp(relOp(id)))
	}
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpStatus, Project: "T-SRU", TicketID: "TRANS", ClaimedBy: "benjamin", Session: "ses_1", Status: ClaimStatusDone}))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpStatus, Project: "T-SRU", TicketID: "TRANS", ClaimedBy: "benjamin", Session: "ses_1", Status: "nonsense"}))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpLabel, Project: "T-SRU", TicketID: "OK", ClaimedBy: "benjamin", Session: "ses_1", Label: "x"}))
	before := commitCount(t, repo.path)

	applied, dropped, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	assert.Equal(t, 6, dropped)
	assert.Equal(t, before+1, commitCount(t, repo.path))
	for _, id := range []string{"OWNED", "OTHERSES", "NOSES", "TRANS"} {
		c, err := repo.GetClaim("T-SRU", id)
		require.NoError(t, err, id)
		assert.Empty(t, c.Labels, id)
	}
	trans, _ := repo.GetClaim("T-SRU", "TRANS")
	assert.Equal(t, ClaimStatusInProgress, trans.Status)
	assert.NoFileExists(t, repo.claimQueuePath(), "dropped operations leave the queue too")
}

func TestFlushClaimQueueNoCommitWhenEverythingIsDropped(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	mustCreate(t, repo, Claim{TicketID: "SRU-1", ClaimedBy: "alice", Session: "ses_alice"})
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	before, beforeBare := commitCount(t, repo.path), commitCount(t, bare)

	applied, dropped, err := repo.FlushClaimQueue(context.Background())
	require.NoError(t, err)
	assert.Zero(t, applied)
	assert.Equal(t, 1, dropped)
	assert.Equal(t, before, commitCount(t, repo.path))
	assert.Equal(t, beforeBare, commitCount(t, bare))
	assert.NoFileExists(t, repo.claimQueuePath())
}

func TestFlushClaimQueueAnyTransition(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	mustClaim(t, repo, "SRU-1")
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpStatus, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1", Status: ClaimStatusDone, AnyTransition: true}))
	applied, dropped, err := repo.FlushClaimQueue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	assert.Zero(t, dropped)
	c, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, ClaimStatusDone, c.Status)
}

func TestFlushClaimQueueIsIdempotent(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	mustClaim(t, repo, "SRU-1")
	mustClaim(t, repo, "SRU-2")
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpStatus, Project: "T-SRU", TicketID: "SRU-2", ClaimedBy: "benjamin", Session: "ses_1", Status: ClaimStatusReview}))
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpLabel, Project: "T-SRU", TicketID: "SRU-2", ClaimedBy: "benjamin", Session: "ses_1", Label: "x"}))
	saved, err := os.ReadFile(repo.claimQueuePath())
	require.NoError(t, err)

	applied, _, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, applied)
	after, afterBare := commitCount(t, repo.path), commitCount(t, bare)
	claim, err := os.ReadFile(repo.claimFilePath("T-SRU", "SRU-2"))
	require.NoError(t, err)

	// Twice in a row: nothing left to do.
	applied, dropped, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Zero(t, applied+dropped)

	// Replay of the same file, as after a crash between the push and the
	// emptying of the queue: no change, no commit.
	require.NoError(t, os.WriteFile(repo.claimQueuePath(), saved, 0o600))
	applied, dropped, err = repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, applied, "status and label are already in place")
	assert.Equal(t, 1, dropped, "the released claim is gone")
	assert.Equal(t, after, commitCount(t, repo.path))
	assert.Equal(t, afterBare, commitCount(t, bare))
	again, err := os.ReadFile(repo.claimFilePath("T-SRU", "SRU-2"))
	require.NoError(t, err)
	assert.Equal(t, string(claim), string(again))
}

func TestFlushClaimQueueKeepsWhatIsQueuedMeanwhile(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	mustClaim(t, repo, "SRU-1")
	mustClaim(t, repo, "SRU-2")
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	repo.beforePush = func() { assert.NoError(t, repo.EnqueueClaimOp(relOp("SRU-2"))) }

	applied, _, err := repo.FlushClaimQueue(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.Equal(t, "SRU-2", ops[0].TicketID)
}

func TestFlushClaimQueueSetsAsideACorruptFile(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	bad := `{"version": 1, "ops": [ {"id": "a", "kin`
	require.NoError(t, os.WriteFile(repo.claimQueuePath(), []byte(bad), 0o600))

	_, _, err := repo.FlushClaimQueue(ctx)
	var corrupt *ClaimQueueCorruptError
	require.ErrorAs(t, err, &corrupt)
	assert.Contains(t, err.Error(), "unreadable")
	assert.Contains(t, err.Error(), "set aside")
	require.NotEmpty(t, corrupt.SetAside)
	saved, rerr := os.ReadFile(corrupt.SetAside)
	require.NoError(t, rerr)
	assert.Equal(t, bad, string(saved), "nothing is lost")
	assert.NoFileExists(t, repo.claimQueuePath())

	// The next flush starts clean.
	applied, dropped, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Zero(t, applied+dropped)
}

func TestEnqueueClaimOpOverACorruptFile(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	require.NoError(t, os.WriteFile(repo.claimQueuePath(), []byte("not json"), 0o600))

	_, err := repo.ClaimQueue()
	var corrupt *ClaimQueueCorruptError
	require.ErrorAs(t, err, &corrupt)
	assert.Empty(t, corrupt.SetAside, "reading does not move the file")
	assert.FileExists(t, repo.claimQueuePath())

	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")), "the new operation is not refused")
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	assert.Len(t, ops, 1)
	matches, err := filepath.Glob(repo.claimQueuePath() + ".corrupt-*")
	require.NoError(t, err)
	require.Len(t, matches, 1, "the old file is kept")
	saved, _ := os.ReadFile(matches[0])
	assert.Equal(t, "not json", string(saved))
}

func TestClaimQueueFromANewerOh(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	future := `{"version": 9, "ops": []}`
	require.NoError(t, os.WriteFile(repo.claimQueuePath(), []byte(future), 0o600))

	_, _, err := repo.FlushClaimQueue(context.Background())
	assert.ErrorIs(t, err, ErrClaimQueueNewer)
	assert.ErrorIs(t, repo.EnqueueClaimOp(relOp("SRU-1")), ErrClaimQueueNewer)
	data, _ := os.ReadFile(repo.claimQueuePath())
	assert.Equal(t, future, string(data), "left as it is")
}

func TestFlushClaimQueueRefusesUnsafeNamesFromTheFile(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	bad, err := json.Marshal(claimQueueFile{Version: 1, Ops: []ClaimOp{
		{ID: "a", Kind: ClaimOpRelease, Project: "..", TicketID: "x", ClaimedBy: "benjamin"},
		{ID: "b", Kind: ClaimOpRelease, Project: "T-SRU", TicketID: "../../config", ClaimedBy: "benjamin"},
	}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(repo.claimQueuePath(), bad, 0o600))
	applied, dropped, err := repo.FlushClaimQueue(context.Background())
	require.NoError(t, err)
	assert.Zero(t, applied)
	assert.Equal(t, 2, dropped)
}

func TestFlushClaimQueueTouch(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	mustClaim(t, repo, "SRU-1")
	created, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)

	// Queued 13 h after the creation (11 h left: less than half), flushed at once.
	at := time.Now().UTC().Add(13 * time.Hour)
	touch := ClaimOp{Kind: ClaimOpTouch, Project: "T-SRU", TicketID: "SRU-1", ClaimedBy: "benjamin", Session: "ses_1", QueuedAt: at}
	require.NoError(t, repo.EnqueueClaimOp(touch))
	applied, _, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	got, err := repo.GetClaim("T-SRU", "SRU-1")
	require.NoError(t, err)
	assert.Equal(t, InstantOf(at.Add(24*time.Hour)), got.LeaseUntil, "renewed from the time it was queued")
	assert.Equal(t, created.LastActivity, got.LastActivity, "a renewal is not an activity")

	// An older touch does not shorten the lease and commits nothing.
	before := commitCount(t, repo.path)
	touch.QueuedAt = time.Now().UTC()
	require.NoError(t, repo.EnqueueClaimOp(touch))
	applied, dropped, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	assert.Zero(t, dropped)
	assert.Equal(t, before, commitCount(t, repo.path))
	got, _ = repo.GetClaim("T-SRU", "SRU-1")
	assert.Equal(t, InstantOf(at.Add(24*time.Hour)), got.LeaseUntil)
}

func TestHeartbeatTouchesThenFlushes(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	mustClaim(t, repo, "SRU-1")
	mustClaim(t, repo, "SRU-2")
	require.NoError(t, repo.EnqueueClaimOp(ClaimOp{Kind: ClaimOpLabel, Project: "T-SRU", TicketID: "SRU-2", ClaimedBy: "benjamin", Session: "ses_1", Label: "x"}))
	before := commitCount(t, bare)

	now := time.Now().UTC().Add(13 * time.Hour)
	res, err := repo.Heartbeat(ctx, []ClaimRef{ref("SRU-1")}, "benjamin", "ses_1", now)
	require.NoError(t, err)
	assert.Equal(t, []ClaimRef{ref("SRU-1")}, res.Touched)
	assert.Equal(t, 1, res.Applied)
	assert.Zero(t, res.Dropped+res.Queued)
	assert.Equal(t, before+2, commitCount(t, bare), "the renewal and the queue: two commits")

	// Nothing to do: no commit.
	res, err = repo.Heartbeat(ctx, []ClaimRef{ref("SRU-1")}, "benjamin", "ses_1", now.Add(time.Minute))
	require.NoError(t, err)
	assert.Empty(t, res.Touched)
	assert.Equal(t, before+2, commitCount(t, bare))
}

func TestHeartbeatOfflineQueuesTheRenewalThenTheNextBeatAppliesIt(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	mustClaim(t, repo, "SRU-1")
	mustClaim(t, repo, "SRU-2")
	online := goOffline(t, repo)
	beforeBare := commitCount(t, bare)

	now := time.Now().UTC().Add(13 * time.Hour)
	refs := []ClaimRef{ref("SRU-1"), ref("SRU-2")}
	for range 2 { // two beats offline: the touches are replaced, not piled up
		res, err := repo.Heartbeat(ctx, refs, "benjamin", "ses_1", now)
		require.ErrorIs(t, err, ErrOffline)
		assert.Empty(t, res.Touched)
		assert.Equal(t, 2, res.Queued)
	}
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	assert.Len(t, ops, 2)

	online()
	res, err := repo.Heartbeat(ctx, nil, "benjamin", "ses_1", now.Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, 2, res.Applied)
	assert.Equal(t, beforeBare+1, commitCount(t, bare))
	for _, id := range []string{"SRU-1", "SRU-2"} {
		c, err := repo.GetClaim("T-SRU", id)
		require.NoError(t, err)
		assert.Equal(t, InstantOf(now.Add(24*time.Hour)), c.LeaseUntil, id)
	}
	assert.NoFileExists(t, repo.claimQueuePath())
}

func TestHeartbeatSkipsTheQueueWhenTheRenewalCouldNotReachTheRemote(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	mustClaim(t, repo, "SRU-1")
	require.NoError(t, repo.EnqueueClaimOp(relOp("SRU-1")))
	goOffline(t, repo)

	_, err := repo.Heartbeat(context.Background(), []ClaimRef{ref("SRU-1")}, "benjamin", "ses_1", time.Now().Add(20*time.Hour))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOffline))
	ops, qerr := repo.ClaimQueue()
	require.NoError(t, qerr)
	assert.Len(t, ops, 2, "the release and the touch")
}
