package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// v5.1 (B3b), risk (b) of B3a: a session ends offline (its `release` waits in
// the queue), then a new session starts on the same ticket, still offline, and
// cannot take the claim over. When the network is back the old operation must
// not release a claim the new session works on.
func TestOfflineAdoptionSurvivesTheReleaseOfThePreviousSession(t *testing.T) {
	ctx := context.Background()
	_, repo, _ := setupClaimTestApp(t)
	claimsCreate(t, repo, "pt-open", "testuser", "ses_1")
	in := sessionClaimsInput{
		Project: claimsProject, Member: "testuser", Session: "ses_1", Tickets: []string{"pt-open"},
		Ticket: lookup(map[string]ticketFacts{"pt-open": {}}), State: staticState(sessionEndState{}),
	}

	online := goOffline(t, repo)
	r := endSessionClaims(ctx, repo, in, time.Second, 20*time.Second)
	require.ErrorIs(t, r.Flush.Err, teamstate.ErrOffline)
	require.Equal(t, claimReleased, r.Outcomes["pt-open"].End)

	note := startTicketClaim(runsvc.WithStartedSession(ctx, "ses_2"), repo, claimsProject, "testuser", "pt-open")
	assert.NotEmpty(t, note, "the person is told the claim is taken over when the network is back")

	online()
	f := flushClaimQueue(ctx, repo, time.Second, 30*time.Second)
	require.NoError(t, f.Err)
	c := claimsOnDisk(t, repo, "pt-open")
	assert.Equal(t, "testuser", c.ClaimedBy)
	assert.Equal(t, "ses_2", c.Session, "the claim is held by the new session")
	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	assert.Empty(t, ops)

	// The new session ends in turn: its own operations apply.
	in.Session = "ses_2"
	r = endSessionClaims(ctx, repo, in, time.Second, 30*time.Second)
	require.NoError(t, r.Flush.Err)
	claimsGone(t, repo, "pt-open")
}

// The previous session may end after the new one has started (it was stopped
// late): its operations must not release the claim the new session queued the
// takeover of.
func TestPreviousSessionEndingAfterTheOfflineAdoptionLeavesTheClaim(t *testing.T) {
	ctx := context.Background()
	_, repo, _ := setupClaimTestApp(t)
	claimsCreate(t, repo, "pt-open", "testuser", "") // a claim taken by hand
	in := sessionClaimsInput{
		Project: claimsProject, Member: "testuser", Session: "ses_1", Tickets: []string{"pt-open"},
		Ticket: lookup(map[string]ticketFacts{"pt-open": {}}), State: staticState(sessionEndState{}),
	}

	online := goOffline(t, repo)
	startTicketClaim(runsvc.WithStartedSession(ctx, "ses_2"), repo, claimsProject, "testuser", "pt-open")
	r := endSessionClaims(ctx, repo, in, time.Second, 20*time.Second)
	assert.Empty(t, r.Outcomes, "the claim is on its way to ses_2: ses_1 leaves it alone")

	online()
	f := flushClaimQueue(ctx, repo, time.Second, 30*time.Second)
	require.NoError(t, f.Err)
	assert.Equal(t, "ses_2", claimsOnDisk(t, repo, "pt-open").Session)
}
