package launch

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// v5.1 (B3b): the claims taken for a remote session carry its ID, so that the
// end of the session (and the lease) apply to the claim it worked on.
func TestTeamClaimsRecordTheRemoteSession(t *testing.T) {
	if testing.Short() {
		t.Skip("git integration")
	}
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)
	tc := TeamClaims{Repo: repo}
	ctx := teamstate.WithClaimSession(context.Background(), "ses_remote")

	created, err := tc.Claim(ctx, "p1", "T-1", "alice")
	require.NoError(t, err)
	assert.True(t, created)
	c, err := repo.GetClaim("p1", "T-1")
	require.NoError(t, err)
	assert.Equal(t, "ses_remote", c.Session)

	// A claim the member already holds (taken by hand) is handed to the session.
	_, err = repo.CreateClaim(context.Background(), teamstate.Claim{TicketID: "T-2", Project: "p1", ClaimedBy: "alice", Status: teamstate.ClaimStatusInProgress})
	require.NoError(t, err)
	created, err = tc.Claim(ctx, "p1", "T-2", "alice")
	require.NoError(t, err)
	assert.False(t, created)
	c, err = repo.GetClaim("p1", "T-2")
	require.NoError(t, err)
	assert.Equal(t, "ses_remote", c.Session)

	// Somebody else's is refused and left alone.
	_, err = repo.CreateClaim(context.Background(), teamstate.Claim{TicketID: "T-3", Project: "p1", ClaimedBy: "bob", Session: "ses_bob", Status: teamstate.ClaimStatusInProgress})
	require.NoError(t, err)
	_, err = tc.Claim(ctx, "p1", "T-3", "alice")
	assert.ErrorIs(t, err, remotesvc.ErrTicketTaken)
	c, _ = repo.GetClaim("p1", "T-3")
	assert.Equal(t, "ses_bob", c.Session)
}
