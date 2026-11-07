package teamstate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// The history of the clone says how a claim file left: on purpose (release,
// queue, retention, re-homing) or by accident (by hand, never there).
func TestClaimRemovedOnPurpose(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	for _, id := range []string{"released", "queued", "cleaned", "rehomed", "by-hand", "kept"} {
		createUnder(t, repo, "p", Claim{TicketID: id, ClaimedBy: "alice", Status: ClaimStatusInProgress})
	}
	require.NoError(t, repo.ReleaseClaim(ctx, "p", "released"))
	require.NoError(t, repo.EnqueueClaimOp(t.Context(), ClaimOp{Kind: ClaimOpRelease, Project: "p", TicketID: "queued", ClaimedBy: "alice"}))
	_, _, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	require.NoError(t, repo.UpdateClaimStatusFromTracker(ctx, "p", "cleaned", ClaimStatusDone))
	_, err = repo.CleanupDoneClaims(ctx, -1)
	require.NoError(t, err)
	_, err = repo.RehomeClaims(ctx, "alice", []ClaimMove{{From: "p", To: "q"}})
	require.NoError(t, err) // moves "rehomed", "by-hand" and "kept" too
	createUnder(t, repo, "p", Claim{TicketID: "by-hand", ClaimedBy: "alice"})
	gittest.Run(t, repo.Path(), "rm", "-q", "projects/p/claims/by-hand.toml")
	gittest.Run(t, repo.Path(), "commit", "-qm", "oops")

	for ticket, want := range map[string]bool{
		"released": true, "queued": true, "cleaned": true, "rehomed": true,
		"by-hand": false, "never-existed": false,
	} {
		assert.Equal(t, want, repo.ClaimRemovedOnPurpose(ctx, "p", ticket), ticket)
	}
	assert.False(t, repo.ClaimRemovedOnPurpose(ctx, "p", "../etc"), "an unsafe name is refused")
}

// A commit that does other things as well names the claims it removed on
// purpose in its body.
func TestClaimRemovedOnPurposeReadsTheTrailersOfTheBody(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	for _, id := range []string{"named", "other", "unnamed"} {
		createUnder(t, repo, "p", Claim{TicketID: id, ClaimedBy: "alice", Status: ClaimStatusInProgress})
	}
	gittest.Run(t, repo.Path(), "rm", "-q", "projects/p/claims/named.toml", "projects/p/claims/other.toml")
	msg := WithReleasedTrailers("sync: 0 created", []Claim{{Project: "p", TicketID: "named"}})
	gittest.Run(t, repo.Path(), "commit", "-qm", msg)
	gittest.Run(t, repo.Path(), "rm", "-q", "projects/p/claims/unnamed.toml")
	gittest.Run(t, repo.Path(), "commit", "-qm", "sync: 0 created")

	assert.True(t, repo.ClaimRemovedOnPurpose(ctx, "p", "named"))
	assert.False(t, repo.ClaimRemovedOnPurpose(ctx, "p", "other"), "removed by the same commit but not named")
	assert.False(t, repo.ClaimRemovedOnPurpose(ctx, "p", "unnamed"))
	assert.Equal(t, "sync: 0 created", WithReleasedTrailers("sync: 0 created", nil))
}
