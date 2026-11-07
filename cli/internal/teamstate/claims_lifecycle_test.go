package teamstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// lifecycleTeam is a team of a lead and two developers with claims of ana and
// bob.
func lifecycleTeam(t *testing.T) (*Repo, string) {
	t.Helper()
	repo, bare := setupGitTestRepo(t)
	ctx := context.Background()
	require.NoError(t, repo.AddMember(ctx, Member{ID: "lea", DisplayName: "Léa", Role: RoleLead}))
	require.NoError(t, repo.AddMember(ctx, Member{ID: "ana", DisplayName: "Ana", Role: "dev"}))
	require.NoError(t, repo.AddMember(ctx, Member{ID: "bob", DisplayName: "Bob", Role: "dev"}))
	for _, c := range []Claim{
		{TicketID: "1", Project: "api", ClaimedBy: "ana", Status: ClaimStatusInProgress},
		{TicketID: "2", Project: "api", ClaimedBy: "ana", Status: ClaimStatusReview},
		{TicketID: "3", Project: "web", ClaimedBy: "ana", Status: ClaimStatusDone},
		{TicketID: "4", Project: "api", ClaimedBy: "bob", Status: ClaimStatusInProgress},
	} {
		_, err := repo.CreateClaim(ctx, c)
		require.NoError(t, err)
	}
	return repo, bare
}

func eventsOf(t *testing.T, repo *Repo, project string) []Event {
	t.Helper()
	events, err := repo.ListEvents(context.Background(), project, time.Time{})
	require.NoError(t, err)
	return events
}

func TestReleaseClaimAsNamesTheOwnerAndTheActor(t *testing.T) {
	t.Parallel()
	repo, bare := lifecycleTeam(t)
	ctx := WithActor(context.Background(), "lea")

	_, err := repo.ReleaseClaimAs(ctx, "api", "1", ReleaseOpts{Owner: "bob"})
	var notOwned *ClaimNotOwnedError
	require.ErrorAs(t, err, &notOwned)
	assert.ErrorIs(t, err, ErrClaimNotOwned)
	assert.Equal(t, "ana", notOwned.Owner, "the error names who holds the claim")
	_, err = repo.GetClaim(ctx, "api", "1")
	require.NoError(t, err, "a refused release removes nothing")

	before := commitCount(t, bare)
	got, err := repo.ReleaseClaimAs(ctx, "api", "1", ReleaseOpts{Owner: "ana"})
	require.NoError(t, err)
	assert.Equal(t, "ana", got.ClaimedBy)
	assert.Equal(t, before+1, commitCount(t, bare), "the claim and its event are one commit")
	assert.Equal(t, "release: api/1", gittest.Run(t, bare, "log", "-1", "--format=%s", "main"))
	assert.True(t, repo.ClaimRemovedOnPurpose(ctx, "api", "1"))

	events := eventsOf(t, repo, "api")
	require.Len(t, events, 1)
	assert.Equal(t, EventClaimReleased, events[0].Type)
	assert.Equal(t, "lea", events[0].Actor, "the actor is the member who releases, not the owner")
	assert.Equal(t, "ana", events[0].Data["owner"])
	assert.Equal(t, ReleaseManual, events[0].Data["reason"])

	_, err = repo.ReleaseClaimAs(ctx, "api", "1", ReleaseOpts{})
	assert.ErrorIs(t, err, ErrClaimNotFound)
}

func TestTransferClaimChecksTheTargetAndTheStatus(t *testing.T) {
	t.Parallel()
	repo, _ := lifecycleTeam(t)
	ctx := WithActor(context.Background(), "lea")

	err := repo.TransferClaim(ctx, "api", "1", "ghost")
	assert.ErrorIs(t, err, ErrMemberNotFound, "the target is a card of the team")
	err = repo.TransferClaim(ctx, "api", "1", "")
	assert.Error(t, err)
	err = repo.TransferClaim(ctx, "web", "3", "bob")
	assert.ErrorIs(t, err, ErrClaimClosed, "a finished claim is not handed over")
	c, err := repo.GetClaim(ctx, "api", "1")
	require.NoError(t, err)
	assert.Equal(t, "ana", c.ClaimedBy)
	assert.Empty(t, eventsOf(t, repo, "api"))

	require.NoError(t, repo.TransferClaim(ctx, "api", "1", "bob"))
	c, err = repo.GetClaim(ctx, "api", "1")
	require.NoError(t, err)
	assert.Equal(t, "bob", c.ClaimedBy)
	events := eventsOf(t, repo, "api")
	require.Len(t, events, 1)
	assert.Equal(t, EventClaimTransferred, events[0].Type)
	assert.Equal(t, "lea", events[0].Actor, "the actor is the member who hands the claim over, not its previous owner")
	assert.Equal(t, "ana", events[0].Data["from"])
	assert.Equal(t, "bob", events[0].Data["to"])
}

func TestReassignClaimsHandsOverTheOpenClaimsInOneCommit(t *testing.T) {
	t.Parallel()
	repo, bare := lifecycleTeam(t)
	ctx := WithActor(context.Background(), "lea")
	before := commitCount(t, bare)

	res, err := repo.ReassignClaims(ctx, ReassignOpts{From: "ana", To: "bob"})
	require.NoError(t, err)
	assert.Len(t, res.Transferred, 2)
	assert.Empty(t, res.Released)
	assert.Equal(t, []ClaimRef{{Project: "web", TicketID: "3"}}, res.Kept, "the finished claim stays where it is")
	assert.Equal(t, before+1, commitCount(t, bare))
	assert.Equal(t, "transfer: 2 claim(s) of ana to bob", gittest.Run(t, bare, "log", "-1", "--format=%s", "main"))

	for _, ref := range res.Transferred {
		c, err := repo.GetClaim(ctx, ref.Project, ref.TicketID)
		require.NoError(t, err)
		assert.Equal(t, "bob", c.ClaimedBy)
	}
	done, err := repo.GetClaim(ctx, "web", "3")
	require.NoError(t, err)
	assert.Equal(t, "ana", done.ClaimedBy)
	for _, e := range eventsOf(t, repo, "api") {
		assert.Equal(t, EventClaimTransferred, e.Type)
		assert.Equal(t, "lea", e.Actor)
	}

	again, err := repo.ReassignClaims(ctx, ReassignOpts{From: "ana", To: "bob"})
	require.NoError(t, err)
	assert.Empty(t, again.Transferred)
	assert.Equal(t, before+1, commitCount(t, bare), "nothing left to move: no second commit")
}

func TestReassignClaimsRefusesWithoutAnAnswerOrAWrongTarget(t *testing.T) {
	t.Parallel()
	repo, bare := lifecycleTeam(t)
	ctx := WithActor(context.Background(), "lea")
	before := commitCount(t, bare)

	_, err := repo.ReassignClaims(ctx, ReassignOpts{From: "ana"})
	var has *MemberHasClaimsError
	require.ErrorAs(t, err, &has)
	assert.Equal(t, 2, has.Count)

	_, err = repo.ReassignClaims(ctx, ReassignOpts{From: "ana", To: "ghost"})
	assert.ErrorIs(t, err, ErrMemberNotFound)
	_, err = repo.ReassignClaims(ctx, ReassignOpts{From: "ana", To: "ana"})
	assert.Error(t, err)
	_, err = repo.ReassignClaims(ctx, ReassignOpts{From: "ana", To: "bob", Release: true})
	assert.Error(t, err)
	assert.Equal(t, before, commitCount(t, bare), "nothing was committed")
	c, err := repo.GetClaim(ctx, "api", "1")
	require.NoError(t, err)
	assert.Equal(t, "ana", c.ClaimedBy)
}

func TestReassignClaimsReleasesAndRemovesTheCardInTheSameCommit(t *testing.T) {
	t.Parallel()
	repo, bare := lifecycleTeam(t)
	ctx := WithActor(context.Background(), "lea")
	before := commitCount(t, bare)

	res, err := repo.ReassignClaims(ctx, ReassignOpts{From: "ana", Release: true, ReleaseReason: ReleaseMemberRemoved, RemoveMember: true})
	require.NoError(t, err)
	assert.Len(t, res.Released, 2)
	assert.True(t, res.MemberRemoved)
	assert.Equal(t, before+1, commitCount(t, bare))
	assert.Equal(t, "release: 2 claim(s) of ana, member removed", gittest.Run(t, bare, "log", "-1", "--format=%s", "main"))
	assert.True(t, repo.ClaimRemovedOnPurpose(ctx, "api", "1"), "known to be released on purpose")

	_, err = repo.GetMember(ctx, "ana")
	assert.ErrorIs(t, err, ErrMemberNotFound)
	team := eventsOf(t, repo, TeamEventsProject)
	require.Len(t, team, 1)
	assert.Equal(t, EventMemberRemoved, team[0].Type)
	assert.Equal(t, "lea", team[0].Actor)
	assert.Equal(t, "ana", team[0].Data["member"])
	assert.EqualValues(t, 2, team[0].Data["claims_released"])
	for _, e := range eventsOf(t, repo, "api") {
		assert.Equal(t, EventClaimReleased, e.Type)
		assert.Equal(t, ReleaseMemberRemoved, e.Data["reason"])
	}
}

func TestRemovingAnotherMemberNeedsALead(t *testing.T) {
	t.Parallel()
	repo, _ := lifecycleTeam(t)

	err := repo.RemoveMember(WithActor(context.Background(), "bob"), "ana")
	assert.ErrorIs(t, err, ErrRemoveNeedsLead)
	_, err = repo.ReassignClaims(WithActor(context.Background(), "bob"), ReassignOpts{From: "ana", Release: true, RemoveMember: true})
	assert.ErrorIs(t, err, ErrRemoveNeedsLead)
	_, err = repo.GetMember(context.Background(), "ana")
	require.NoError(t, err, "a refused removal removes nothing")
	c, err := repo.GetClaim(context.Background(), "api", "1")
	require.NoError(t, err)
	assert.Equal(t, "ana", c.ClaimedBy, "nor does it release the claims")

	// a member removes their own card
	_, err = repo.ReassignClaims(WithActor(context.Background(), "bob"), ReassignOpts{From: "bob", Release: true, RemoveMember: true, ReleaseReason: ReleaseLeftTeam})
	require.NoError(t, err)
	events := eventsOf(t, repo, TeamEventsProject)
	require.Len(t, events, 1)
	assert.Equal(t, true, events[0].Data["self"])

	// a lead removes another
	require.NoError(t, repo.RemoveMember(WithActor(context.Background(), "lea"), "ana"))
}

func TestATeamWithoutALeadIsNotLocked(t *testing.T) {
	t.Parallel()
	repo, _ := setupGitTestRepo(t)
	ctx := context.Background()
	require.NoError(t, repo.AddMember(ctx, Member{ID: "ana", Role: "dev"}))
	require.NoError(t, repo.AddMember(ctx, Member{ID: "bob", Role: "dev"}))
	require.NoError(t, repo.RemoveMember(WithActor(ctx, "bob"), "ana"), "no lead: any member may remove a card")
}

func TestQueuedReleaseLeavesAnEvent(t *testing.T) {
	t.Parallel()
	repo, _ := lifecycleTeam(t)
	ctx := context.Background()
	require.NoError(t, repo.EnqueueClaimOp(t.Context(), ClaimOp{Kind: ClaimOpRelease, Project: "api", TicketID: "4", ClaimedBy: "bob"}))
	applied, _, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	events := eventsOf(t, repo, "api")
	require.Len(t, events, 1)
	assert.Equal(t, EventClaimReleased, events[0].Type)
	assert.Equal(t, "bob", events[0].Actor)
	assert.Equal(t, ReleaseSessionEnd, events[0].Data["reason"])
}

func TestPendingWorkSeesWhatTheRemoteLacks(t *testing.T) {
	t.Parallel()
	repo, _ := lifecycleTeam(t)
	ctx := context.Background()
	p, err := repo.PendingWork(ctx)
	require.NoError(t, err)
	assert.True(t, p.None(), "%+v", p)

	require.NoError(t, os.WriteFile(filepath.Join(repo.Path(), "scratch.txt"), []byte("x"), 0o644))
	p, err = repo.PendingWork(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"scratch.txt"}, p.Uncommitted)

	gittest.Run(t, repo.Path(), "add", "scratch.txt")
	gittest.Run(t, repo.Path(), "commit", "-m", "local only")
	p, err = repo.PendingWork(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, p.Unpushed)
	assert.Empty(t, p.Uncommitted)
	assert.False(t, p.None())
}

func TestQueuedCreateTakesAFreeTicketOnly(t *testing.T) {
	t.Parallel()
	repo, _ := lifecycleTeam(t)
	ctx := context.Background()

	queued, err := repo.QueueCreateClaim(t.Context(), "api", "NEW-1", "ana", "ses_1", "")
	require.NoError(t, err)
	assert.True(t, queued)
	queued, err = repo.QueueCreateClaim(t.Context(), "api", "NEW-1", "ana", "ses_1", "")
	require.NoError(t, err)
	assert.True(t, queued, "queuing it twice queues it once")
	queued, err = repo.QueueCreateClaim(t.Context(), "api", "1", "ana", "ses_1", "")
	require.NoError(t, err)
	assert.False(t, queued, "a ticket with a claim in the clone is not queued")
	queued, err = repo.QueueCreateClaim(t.Context(), "api", "NEW-2", "bob", "ses_2", "")
	require.NoError(t, err)
	assert.True(t, queued)
	// bob's NEW-2 is taken by lea before the flush
	other := NewRepo(repo.Remote(), repo.Path()+"-other")
	require.NoError(t, other.Clone(ctx))
	_, err = other.CreateClaim(ctx, Claim{TicketID: "NEW-2", Project: "api", ClaimedBy: "lea", Status: ClaimStatusInProgress})
	require.NoError(t, err)

	applied, dropped, err := repo.FlushClaimQueue(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)
	assert.Equal(t, 1, dropped)
	c, err := repo.GetClaim(ctx, "api", "NEW-1")
	require.NoError(t, err)
	assert.Equal(t, "ana", c.ClaimedBy)
	assert.Equal(t, "ses_1", c.Session)
	assert.Equal(t, ClaimStatusInProgress, c.Status)
	c, err = repo.GetClaim(ctx, "api", "NEW-2")
	require.NoError(t, err)
	assert.Equal(t, "lea", c.ClaimedBy)
}
