package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// A session ends offline (its `release` waits in
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
	ops, err := repo.ClaimQueue(t.Context())
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

// A session launched offline on a ticket nobody holds says it, and takes the
// ticket when the network is back — if it is still free then.
func TestOfflineLaunchOnANewTicketQueuesTheClaim(t *testing.T) {
	i18ntest.Set(t, "en")
	ctx := context.Background()
	_, repo, _ := setupClaimTestApp(t)
	addTeamMembers(t, repo, "bob")

	online := goOffline(t, repo)
	note := startTicketClaim(runsvc.WithStartedSession(ctx, "ses_1"), repo, claimsProject, "testuser", "NEW-1")
	assert.Contains(t, note, "will be claimed in your name when the network is back")
	note = startTicketClaim(runsvc.WithStartedSession(ctx, "ses_1"), repo, claimsProject, "testuser", "NEW-2")
	assert.Contains(t, note, "will be claimed")
	_, err := repo.GetClaim(ctx, claimsProject, "NEW-1")
	require.ErrorIs(t, err, teamstate.ErrClaimNotFound, "nothing yet")

	online()
	// meanwhile a colleague took NEW-2
	other := freshClone(t, repo.Remote())
	_, err = other.CreateClaim(ctx, teamstate.Claim{TicketID: "NEW-2", Project: claimsProject, ClaimedBy: "bob", Status: teamstate.ClaimStatusInProgress})
	require.NoError(t, err)

	f := flushClaimQueue(ctx, repo, time.Second, 30*time.Second)
	require.NoError(t, f.Err)
	c := claimsOnDisk(t, repo, "NEW-1")
	assert.Equal(t, "testuser", c.ClaimedBy)
	assert.Equal(t, "ses_1", c.Session)
	assert.Equal(t, "bob", claimsOnDisk(t, repo, "NEW-2").ClaimedBy, "a claim taken meanwhile is left to its owner")
	events := eventsOn(t, repo, claimsProject)
	taken := 0
	for _, e := range events {
		if e.Type == teamstate.EventClaimTaken && e.Ticket == "NEW-1" {
			taken++
			assert.Equal(t, "testuser", e.Actor)
		}
	}
	assert.Equal(t, 1, taken)
	ops, err := repo.ClaimQueue(t.Context())
	require.NoError(t, err)
	assert.Empty(t, ops)
}
