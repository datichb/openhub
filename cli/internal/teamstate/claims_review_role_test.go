package teamstate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reviewRoleBoard() BoardConfig {
	return BoardConfig{Columns: []BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: ColumnRoleActive},
		{ID: "mr", Name: "MR open", Role: ColumnRoleReview},
		{ID: "shipped", Name: "Shipped", Role: ColumnRoleTerminal},
	}}
}

// A claim in a column of role review is in review whatever the ID of the
// column: it waits for a person, so it never expires and carries no lease that
// runs out, as a claim at the status review.
func TestClaimInAReviewRoleColumnNeverExpires(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	cfg := &TeamConfig{Takeover: TakeoverConfig{StaleDays: 2}, Board: reviewRoleBoard()}
	past := InstantOf(now.Add(-48 * time.Hour))
	c := Claim{ClaimedBy: "alice", Status: "mr", LeaseUntil: past, LastActivity: InstantOf(now.Add(-30 * 24 * time.Hour))}

	assert.False(t, c.InReview(), "without the board only the status and the labels are known")
	assert.True(t, c.InReviewOn(cfg.Board))
	assert.False(t, c.IsExpired(now, cfg))
	assert.False(t, c.HasLease(cfg))
	assert.True(t, c.ReviewStale(now, cfg), "the doctor tells it has waited long")

	// The same claim on a board that has no such column: an ordinary status.
	other := &TeamConfig{Takeover: TakeoverConfig{StaleDays: 2}}
	assert.False(t, c.InReviewOn(other.Board))
	assert.True(t, c.IsExpired(now, other))

	// The development column is not a review one.
	dev := Claim{ClaimedBy: "alice", Status: "dev", LeaseUntil: past}
	assert.False(t, dev.InReviewOn(cfg.Board))
	assert.True(t, dev.IsExpired(now, cfg))
	assert.True(t, dev.HasLease(cfg))

	// The default board: the review status, as before.
	assert.True(t, (&Claim{Status: ClaimStatusReview}).InReviewOn(DefaultBoardConfig()))
	assert.True(t, (&Claim{Status: ClaimStatusReview}).InReviewOn(BoardConfig{}))
}

// A claim back from a review column gets a lease that runs from now, as one
// back from the status review does.
func TestClaimLeavingAReviewRoleColumnGetsAFreshLease(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	board := reviewRoleBoard()
	require.NoError(t, repo.SaveConfig(ctx, &TeamConfig{Board: board}))
	repo.SetBoardStatuses(board.AllStatuses())

	past := InstantOf(time.Now().Add(-72 * time.Hour))
	createUnder(t, repo, "p", Claim{TicketID: "T-1", ClaimedBy: "alice", Session: "ses_1", Status: "dev", LeaseUntil: past})
	require.NoError(t, repo.UpdateClaimStatus(ctx, "p", "T-1", "mr"))
	require.NoError(t, repo.UpdateClaimStatus(ctx, "p", "T-1", "dev"))
	got, err := repo.GetClaim(ctx, "p", "T-1")
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), got.LeaseUntil.Time(), time.Minute, "a lease that runs from the end of the review")
	assert.False(t, got.IsExpired(time.Now(), &TeamConfig{Board: board}))
}

// The end of a session keeps a claim in a review column.
func TestSessionEndKeepsAClaimInAReviewRoleColumn(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ClaimEndKeep, DecideClaimEnd(ClaimEndFacts{ClaimStatus: "mr", ClaimInReview: true}).Action)
	assert.Equal(t, ClaimEndInReview, DecideClaimEnd(ClaimEndFacts{ClaimStatus: "mr", ClaimInReview: true}).Reason)
	assert.Equal(t, ClaimEndRelease, DecideClaimEnd(ClaimEndFacts{ClaimStatus: "mr"}).Action, "the fact is what says it")
	assert.Equal(t, ClaimEndComplete, DecideClaimEnd(ClaimEndFacts{ClaimStatus: "mr", ClaimInReview: true, Closed: true}).Action, "a closed ticket still completes")
}
