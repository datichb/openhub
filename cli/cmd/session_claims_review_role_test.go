package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The claim of a ticket whose session ends while it sits in a column of role
// review (whatever its ID) is kept, as one at the status review is: it waits
// for a person.
func TestEndTicketClaimsKeepsAClaimInAReviewRoleColumn(t *testing.T) {
	ctx := context.Background()
	_, repo, _ := setupClaimTestApp(t)
	board := teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: teamstate.ColumnRoleActive},
		{ID: "mr", Name: "MR open", Role: teamstate.ColumnRoleReview},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
	}}
	claimsCreate(t, repo, "pt-mr", "testuser", "ses_1")
	claimsCreate(t, repo, "pt-dev", "testuser", "ses_1")
	require.NoError(t, repo.SaveConfig(ctx, &teamstate.TeamConfig{Board: board}))
	repo.SetBoardStatuses(board.AllStatuses())
	require.NoError(t, repo.UpdateClaimStatus(ctx, claimsProject, "pt-mr", "mr"))
	require.NoError(t, repo.UpdateClaimStatus(ctx, claimsProject, "pt-dev", "dev"))

	in := sessionClaimsInput{
		Project: claimsProject, Member: "testuser", Session: "ses_1",
		Tickets: []string{"pt-mr", "pt-dev"},
		Ticket:  lookup(map[string]ticketFacts{"pt-mr": {}, "pt-dev": {}}), State: staticState(sessionEndState{}),
	}
	r := endSessionClaims(ctx, repo, in, time.Second, 30*time.Second)
	require.NoError(t, r.Flush.Err)
	assert.Equal(t, claimKept, r.Outcomes["pt-mr"].End)
	assert.Equal(t, teamstate.ClaimEndInReview, r.Outcomes["pt-mr"].Reason)
	assert.Equal(t, "mr", claimsOnDisk(t, repo, "pt-mr").Status, "left where it is")
	assert.Equal(t, claimReleased, r.Outcomes["pt-dev"].End, "an ordinary active column: released as before")
	claimsGone(t, repo, "pt-dev")
}
