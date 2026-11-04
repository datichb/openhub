package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/beads/bdtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// cardFacts makes the project tell what is known of the cards (by ID).
func cardFacts(t *testing.T, facts map[string]ticketFacts) {
	t.Helper()
	prev := cardFactsOf
	cardFactsOf = func(string) ticketLookup { return lookup(facts) }
	t.Cleanup(func() { cardFactsOf = prev })
}

// A card the launch started the work on carries the session: when the session
// ends, its claim is decided like the claims of the tickets of the session,
// though the session has no ticket (quick on an issue). Closed: terminal;
// in review: kept; nothing left: released; the claims of another session or
// another member are left alone.
func TestSessionEndSettlesTheClaimOfACard(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name  string
		facts ticketFacts
		check func(t *testing.T, repo *teamstate.Repo)
	}{
		{"closed", ticketFacts{Closed: true}, func(t *testing.T, repo *teamstate.Repo) {
			assert.Equal(t, teamstate.ClaimStatusDone, claimsOnDisk(t, repo, "623").Status)
		}},
		{"in review", ticketFacts{Labels: []string{"ready-for-review"}}, func(t *testing.T, repo *teamstate.Repo) {
			assert.Equal(t, teamstate.ClaimStatusInProgress, claimsOnDisk(t, repo, "623").Status, "kept as it is")
		}},
		{"nothing left", ticketFacts{}, func(t *testing.T, repo *teamstate.Repo) { claimsGone(t, repo, "623") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, repo, _ := setupClaimTestApp(t)
			proj := teamProject(claimsProject)
			a.Projects = &mockProjectStore{projects: []domain.Project{proj}}
			cardFacts(t, map[string]ticketFacts{"623": c.facts, "624": {Closed: true}, "625": {Closed: true}})

			// The launch put the card in progress with its session.
			note := claimCardAtLaunch(ctx, a, &proj, &tuiCardClaim{Project: claimsProject, Ticket: "623"}, "ses_card")
			require.NotEmpty(t, note)
			require.Equal(t, "ses_card", claimsOnDisk(t, repo, "623").Session, "the claim is tied to the session")
			claimsCreate(t, repo, "624", "testuser", "ses_other") // the card of another session
			claimsCreate(t, repo, "625", "bob", "ses_card")       // somebody else's

			sessionClaimsEnd(ctx, a, &proj, domain.Session{ID: "ses_card", ProjectID: claimsProject}, false)

			c.check(t, repo)
			assert.Equal(t, teamstate.ClaimStatusInProgress, claimsOnDisk(t, repo, "624").Status, "another session's claim")
			assert.Equal(t, teamstate.ClaimStatusInProgress, claimsOnDisk(t, repo, "625").Status, "another member's claim")
			ops, err := repo.ClaimQueue()
			require.NoError(t, err)
			assert.Empty(t, ops, "the durable queue was flushed")
		})
	}
}

// The card of the session and its tickets are settled together, each with its
// own facts; in the daemon the decisions are queued before the flush, as for
// the tickets.
func TestSessionEndSettlesTheTicketsAndTheCardTogether(t *testing.T) {
	ctx := context.Background()
	a, repo, proj, s := endedSessionApp(t) // pt-open, held by ses_1
	cardFacts(t, map[string]ticketFacts{"623": {Closed: true}})
	_, err := repo.CreateClaim(ctx, teamstate.Claim{TicketID: "623", Project: claimsProject, ClaimedBy: "testuser", Status: teamstate.ClaimStatusInProgress, Session: "ses_1"})
	require.NoError(t, err)
	swapClaimFlushes(t, &taskTracker{spawn: func(func()) {}}) // the flush never starts

	sessionClaimsEnd(ctx, a, proj, s, true)

	ops, err := repo.ClaimQueue()
	require.NoError(t, err)
	require.Len(t, ops, 2, "queued although the background task never ran")
	kinds := map[string]string{}
	for _, op := range ops {
		kinds[op.TicketID] = op.Kind
	}
	assert.Equal(t, map[string]string{"pt-open": teamstate.ClaimOpRelease, "623": teamstate.ClaimOpStatus}, kinds)

	// The next beat of the daemon applies them.
	require.NoError(t, beaterOf(a, time.Now()).beat(ctx))
	claimsGone(t, repo, "pt-open")
	assert.Equal(t, teamstate.ClaimStatusDone, claimsOnDisk(t, repo, "623").Status)
}

// A session that holds no claim ends as before: nothing to settle.
func TestSessionEndWithoutTicketsNorCardDoesNothing(t *testing.T) {
	ctx := context.Background()
	a, repo, _ := setupClaimTestApp(t)
	proj := teamProject(claimsProject)
	a.Projects = &mockProjectStore{projects: []domain.Project{proj}}
	cardFacts(t, nil)
	claimsCreate(t, repo, "625", "testuser", "ses_other")
	commits := commitsOf(t, repo.Path())
	sessionClaimsEnd(ctx, a, &proj, domain.Session{ID: "ses_free", ProjectID: claimsProject}, false)
	assert.Equal(t, commits, commitsOf(t, repo.Path()))
	assert.Equal(t, "testuser", claimsOnDisk(t, repo, "625").ClaimedBy)
}

// The heartbeat of the daemon renews the lease of the card of an open session
// too, though the session has no ticket.
func TestClaimsBeatRenewsTheLeaseOfACardOfAnOpenSession(t *testing.T) {
	open := openSession("ses_card") // no ticket
	a, repo := setupBeat(t, open)
	claimsCreate(t, repo, "623", "testuser", "ses_card")
	claimsCreate(t, repo, "624", "testuser", "ses_gone")
	claimsCreate(t, repo, "625", "bob", "ses_card")
	before624 := claimsOnDisk(t, repo, "624").LeaseUntil
	before625 := claimsOnDisk(t, repo, "625").LeaseUntil

	later := time.Now().Add(13 * time.Hour)
	require.NoError(t, beaterOf(a, later).beat(context.Background()))

	assert.WithinDuration(t, later.Add(24*time.Hour), claimsOnDisk(t, repo, "623").LeaseUntil.Time(), 2*time.Second, "the card of the open session")
	assert.Equal(t, before624, claimsOnDisk(t, repo, "624").LeaseUntil, "a session that is not open renews nothing")
	assert.Equal(t, before625, claimsOnDisk(t, repo, "625").LeaseUntil, "somebody else's claim")
}

// With the real bd: the Beads ticket the issue was imported as tells the end
// of the session whether the card's work is closed; a card with no such
// ticket, or a key of another tracker, has no facts and bd is not asked.
func TestCardFactsFromTheImportedTicket(t *testing.T) {
	bdtest.RequireRealBd(t)
	dir := bdtest.InitedRepo(t)
	t.Setenv("BEADS_DIR", "")
	facts := cardFactsFromBeads(dir)

	got, err := facts("623")
	require.NoError(t, err)
	assert.Equal(t, ticketFacts{}, got, "no ticket imported: nothing known")

	id, _, err := beads.ImportFromTracker(dir, beads.ProviderGitLab, "623", "Export CSV", "")
	require.NoError(t, err)
	got, err = facts("623")
	require.NoError(t, err)
	assert.False(t, got.Closed)

	closeCmd := beads.BdCommand(t.Context(), "-C", dir, "close", id)
	out, err := closeCmd.CombinedOutput()
	require.NoError(t, err, string(out))
	got, err = facts("623")
	require.NoError(t, err)
	assert.True(t, got.Closed, "the imported ticket is closed: so is the card's work")

	got, err = facts("SRU-12")
	require.NoError(t, err)
	assert.Equal(t, ticketFacts{}, got, "a key of another tracker")
	got, err = cardFactsFromBeads(t.TempDir())("623")
	require.NoError(t, err)
	assert.Equal(t, ticketFacts{}, got, "Beads is not initialised in the project")
}
