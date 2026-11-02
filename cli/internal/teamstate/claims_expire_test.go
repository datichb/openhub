package teamstate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func TestReleaseExpiredClaims(t *testing.T) {
	t.Parallel()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	now := time.Now().UTC()
	past, future := InstantOf(now.Add(-time.Hour)), InstantOf(now.Add(time.Hour))
	for _, c := range []Claim{
		{TicketID: "expired", ClaimedBy: "alice", Status: ClaimStatusInProgress, LeaseUntil: past},
		{TicketID: "expired-blocked", ClaimedBy: "alice", Status: ClaimStatusBlocked, LeaseUntil: past},
		{TicketID: "alive", ClaimedBy: "alice", Status: ClaimStatusInProgress, LeaseUntil: future},
		{TicketID: "in-review", ClaimedBy: "alice", Status: ClaimStatusReview, LeaseUntil: past},
		{TicketID: "review-label", ClaimedBy: "alice", Status: ClaimStatusInProgress, LeaseUntil: past, Labels: []string{LabelReadyForReview}},
		{TicketID: "pool", Status: ClaimStatusInProgress, LeaseUntil: past},
		{TicketID: "planned", ClaimedBy: "alice", Status: ClaimStatusPlanned, LeaseUntil: past},
		{TicketID: "done", ClaimedBy: "alice", Status: ClaimStatusDone, LeaseUntil: past},
		{TicketID: "stale-no-lease", ClaimedBy: "alice", Status: ClaimStatusInProgress, ClaimedAt: now.Add(-30 * 24 * time.Hour)},
		{TicketID: "other-project", ClaimedBy: "alice", Status: ClaimStatusInProgress, LeaseUntil: past},
	} {
		project := "p"
		if c.TicketID == "other-project" {
			project = "q"
		}
		createUnder(t, repo, project, c)
	}
	// A claim created without lease takes one: write the old shape by hand.
	stale, err := repo.GetClaim("p", "stale-no-lease")
	require.NoError(t, err)
	stale.LeaseUntil = ""
	data, err := marshalClaim(stale)
	require.NoError(t, err)
	require.NoError(t, writeRel(repo.Path(), "projects/p/claims/stale-no-lease.toml", data))
	gittest.Run(t, repo.Path(), "commit", "-qam", "seed")
	gittest.Run(t, repo.Path(), "push", "-q")
	before := commitCount(t, bare)

	released, err := repo.ReleaseExpiredClaims(ctx, now, "p")
	require.NoError(t, err)
	var ids []string
	for _, c := range released {
		ids = append(ids, c.TicketID)
	}
	assert.ElementsMatch(t, []string{"expired", "expired-blocked", "stale-no-lease"}, ids)
	assert.Equal(t, before+1, commitCount(t, bare), "one commit")
	assert.Contains(t, gittest.Run(t, repo.Path(), "log", "-1", "--format=%s"), "release: 3 expired claim(s)")
	for _, id := range []string{"alive", "in-review", "review-label", "pool", "planned", "done"} {
		_, err := repo.GetClaim("p", id)
		assert.NoError(t, err, id+" is kept")
	}
	_, err = repo.GetClaim("q", "other-project")
	assert.NoError(t, err, "the other project is not asked")
	for _, id := range ids {
		assert.True(t, repo.ClaimRemovedOnPurpose(ctx, "p", id), id)
	}

	released, err = repo.ReleaseExpiredClaims(ctx, now, "p")
	require.NoError(t, err)
	assert.Empty(t, released)
	assert.Equal(t, before+1, commitCount(t, bare), "nothing to release: no commit")

	released, err = repo.ReleaseExpiredClaims(ctx, now)
	require.NoError(t, err)
	require.Len(t, released, 1, "no project: all of them")
	assert.Equal(t, "other-project", released[0].TicketID)
}
