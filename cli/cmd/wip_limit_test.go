package cmd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// wipTeam is a team of one member (testuser) whose limit of tickets in
// progress is limit (team config, new key) with n tickets in progress.
func wipTeam(t *testing.T, limit, inProgress int) (*app.App, *teamstate.Repo) {
	t.Helper()
	i18ntest.Set(t, "en")
	a, repo := setupBeat(t)
	setTeamLimit(t, repo, &limit)
	for i := 1; i <= inProgress; i++ {
		claimsCreate(t, repo, fmt.Sprintf("W-%d", i), "testuser", "")
	}
	return a, repo
}

func setTeamLimit(t *testing.T, repo *teamstate.Repo, limit *int) {
	t.Helper()
	cfg, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	cfg.Tracker.MaxInProgressPerMember = limit
	cfg.Tracker.MaxAutoPlanPerMember = 0
	require.NoError(t, repo.SaveConfig(t.Context(), cfg))
}

func intp(n int) *int { return &n }

func requireLimitError(t *testing.T, err error, count, limit int) {
	t.Helper()
	var lim *teamstate.InProgressLimitError
	require.ErrorAs(t, err, &lim)
	assert.Equal(t, count, lim.Count)
	assert.Equal(t, limit, lim.Limit)
	assert.Contains(t, err.Error(), "max_in_progress_per_member")
	assert.Contains(t, err.Error(), fmt.Sprint(count))
}

// oh team claim: one more ticket in progress is refused at the limit, with
// the count and the key; nothing is created. A planned claim, a ticket that
// already has a claim and a limit of 0 are not.
func TestTeamClaimIsRefusedAtTheInProgressLimit(t *testing.T) {
	a, repo := wipTeam(t, 2, 2)

	err := runClaim(newClaimCmd(t, claimsProject), []string{"NEW-1"})
	requireLimitError(t, err, 2, 2)
	_, gerr := repo.GetClaim(t.Context(), claimsProject, "NEW-1")
	assert.ErrorIs(t, gerr, teamstate.ErrClaimNotFound, "nothing is created")

	// Planned is not in progress.
	plan := newClaimCmd(t, claimsProject)
	require.NoError(t, plan.Flags().Set("planned", "true"))
	require.NoError(t, runClaim(plan, []string{"PLAN-1"}))

	// The member's own claim in progress is renewed, not counted twice.
	require.NoError(t, runClaim(newClaimCmd(t, claimsProject), []string{"W-1"}))

	// No limit.
	setTeamLimit(t, repo, intp(0))
	require.NoError(t, runClaim(newClaimCmd(t, claimsProject), []string{"NEW-1"}))
	assert.Equal(t, "testuser", claimsOnDisk(t, repo, "NEW-1").ClaimedBy)

	// The limit of hub.toml wins over the team's.
	setTeamLimit(t, repo, intp(10))
	a.Config.Tracker.MaxInProgressPerMember = intp(3)
	err = runClaim(newClaimCmd(t, claimsProject), []string{"NEW-2"})
	requireLimitError(t, err, 3, 3)
	a.Config.Tracker.MaxInProgressPerMember = intp(0)
	require.NoError(t, runClaim(newClaimCmd(t, claimsProject), []string{"NEW-2"}))
}

// The tickets in review (status or label) wait for a person: they are not
// counted.
func TestTeamClaimDoesNotCountTheTicketsInReview(t *testing.T) {
	_, repo := wipTeam(t, 2, 2)
	ctx := t.Context()
	claimsCreate(t, repo, "R-1", "testuser", "")
	require.NoError(t, repo.UpdateClaimStatus(ctx, claimsProject, "R-1", teamstate.ClaimStatusReview))
	claimsCreate(t, repo, "R-2", "testuser", "")
	require.NoError(t, repo.AddClaimLabel(ctx, claimsProject, "R-2", teamstate.LabelReadyForReview))
	// Another member's claims are not the member's.
	claimsCreate(t, repo, "B-1", "bob", "")
	claimsCreate(t, repo, "B-2", "bob", "")

	err := runClaim(newClaimCmd(t, claimsProject), []string{"NEW-1"})
	requireLimitError(t, err, 2, 2) // W-1 and W-2 only

	// One of the two in progress goes to review: there is room again.
	require.NoError(t, repo.UpdateClaimStatus(ctx, claimsProject, "W-2", teamstate.ClaimStatusReview))
	require.NoError(t, runClaim(newClaimCmd(t, claimsProject), []string{"NEW-1"}))
}

// The former max_auto_plan_per_member of the team is the limit when the new
// key is absent.
func TestTeamClaimReadsTheFormerKeyOfTheTeam(t *testing.T) {
	_, repo := wipTeam(t, 5, 0)
	// What an older oh left in config.toml, not what SaveConfig writes.
	require.NoError(t, writeFormerKey(repo, 1))
	claimsCreate(t, repo, "W-1", "testuser", "")

	err := runClaim(newClaimCmd(t, claimsProject), []string{"NEW-1"})
	requireLimitError(t, err, 1, 1)
}

func writeFormerKey(repo *teamstate.Repo, n int) error {
	return os.WriteFile(repo.Path()+"/config.toml", []byte(fmt.Sprintf("[tracker]\nmax_auto_plan_per_member = %d\n", n)), 0o644)
}

// The takeover of an expired claim in progress, and a claim handed to
// oneself, are one more ticket in progress; handed to somebody else they are
// not.
func TestTakeoverAndTransferToSelfAreRefusedAtTheLimit(t *testing.T) {
	a, repo := wipTeam(t, 1, 1)
	expiredClaimOf(t, repo, "EXP-1", "alice")
	terminalIs(t, false)

	err := runClaim(newClaimYesCmd(t, claimsProject, true), []string{"EXP-1"})
	requireLimitError(t, err, 1, 1)
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "EXP-1").ClaimedBy, "the claim stays with its owner")

	claimsCreate(t, repo, "ALI-1", "alice", "")
	err = runClaimTransfer(newTransferCmd(t, claimsProject, "testuser"), []string{"ALI-1"})
	requireLimitError(t, err, 1, 1)
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "ALI-1").ClaimedBy)

	// To somebody else, or when the claim is not work in progress: allowed.
	require.NoError(t, runClaimTransfer(newTransferCmd(t, claimsProject, "bob"), []string{"ALI-1"}))
	_, err = repo.CreateClaim(t.Context(), teamstate.Claim{TicketID: "PLN-1", Project: claimsProject, ClaimedBy: "alice", Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	require.NoError(t, runClaimTransfer(newTransferCmd(t, claimsProject, "testuser"), []string{"PLN-1"}))
	_ = a
}

// The board: the take key, a card moved to in progress and a claim handed to
// oneself are refused at the limit; a move to a column that is not work, a
// card of somebody else and the take of a ticket someone holds are not.
func TestTeamBoardActionsAreRefusedAtTheInProgressLimit(t *testing.T) {
	a, repo := wipTeam(t, 2, 2)
	ctx := t.Context()
	actions := buildTeamBoardViewConfig(a).Actions
	require.NotNil(t, actions)

	// Take key.
	err := actions.OnClaim("NEW-1")
	requireLimitError(t, err, 2, 2)
	_, gerr := repo.GetClaim(ctx, claimsProject, "NEW-1")
	assert.ErrorIs(t, gerr, teamstate.ErrClaimNotFound)

	// A ticket someone holds: left as it is, no refusal.
	claimsCreate(t, repo, "B-1", "bob", "")
	require.NoError(t, actions.OnClaim("B-1"))
	assert.Equal(t, "bob", claimsOnDisk(t, repo, "B-1").ClaimedBy)

	// A pool ticket is a take too.
	_, err = repo.CreateClaim(ctx, teamstate.Claim{TicketID: "POOL-1", Project: claimsProject, Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	requireLimitError(t, actions.OnClaim("POOL-1"), 2, 2)
	assert.Empty(t, claimsOnDisk(t, repo, "POOL-1").ClaimedBy)

	// A planned card of the member dragged to in progress.
	_, err = repo.CreateClaim(ctx, teamstate.Claim{TicketID: "PLN-1", Project: claimsProject, ClaimedBy: "testuser", Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	requireLimitError(t, actions.OnStatus("PLN-1", teamstate.ClaimStatusInProgress), 2, 2)
	assert.Equal(t, teamstate.ClaimStatusPlanned, claimsOnDisk(t, repo, "PLN-1").Status)
	// …but not to a column that is not work, nor between two columns of work.
	require.NoError(t, actions.OnStatus("W-1", teamstate.ClaimStatusReview))
	require.NoError(t, actions.OnStatus("W-1", teamstate.ClaimStatusInProgress), "W-1 was left in review, there is room: 1 in progress")
	// A card of somebody else is not the member's to count.
	_, err = repo.CreateClaim(ctx, teamstate.Claim{TicketID: "BPL-1", Project: claimsProject, ClaimedBy: "bob", Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	require.NoError(t, actions.OnStatus("BPL-1", teamstate.ClaimStatusInProgress))

	// Handed to oneself.
	claimsCreate(t, repo, "ALI-1", "alice", "")
	requireLimitError(t, actions.OnTransfer("ALI-1", "testuser"), 2, 2)
	require.NoError(t, actions.OnTransfer("ALI-1", "bob"))
	assert.Equal(t, "bob", claimsOnDisk(t, repo, "ALI-1").ClaimedBy)

	// Without limit everything goes through.
	setTeamLimit(t, repo, intp(0))
	require.NoError(t, actions.OnClaim("NEW-1"))
	require.NoError(t, actions.OnStatus("PLN-1", teamstate.ClaimStatusInProgress))
}

// A session launched on tickets: the tickets it would start are one more
// ticket in progress; the ones the member holds in progress or in review
// start nothing, a planned one does.
func TestTicketsRoomOfASessionLaunch(t *testing.T) {
	a, repo := wipTeam(t, 2, 2)
	ctx := t.Context()
	claimsCreate(t, repo, "R-1", "testuser", "")
	require.NoError(t, repo.UpdateClaimStatus(ctx, claimsProject, "R-1", teamstate.ClaimStatusReview))
	_, err := repo.CreateClaim(ctx, teamstate.Claim{TicketID: "PLN-1", Project: claimsProject, ClaimedBy: "testuser", Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)
	claimsCreate(t, repo, "B-1", "bob", "")
	local := a.Config.Tracker

	check := func(tickets ...string) error {
		return checkTicketsRoom(ctx, repo, local, "testuser", claimsProject, tickets)
	}
	requireLimitError(t, check("NEW-1"), 2, 2)
	requireLimitError(t, check("PLN-1"), 2, 2)
	assert.NoError(t, check("W-1", "W-2"), "already in progress: nothing new")
	assert.NoError(t, check("R-1"), "in review: the session starts nothing")
	assert.NoError(t, check("B-1"), "somebody else's: the session starts nothing")
	assert.NoError(t, check(), "no ticket")

	// Two tickets at once past the limit: refused with the number it would add.
	setTeamLimit(t, repo, intp(3))
	assert.NoError(t, check("NEW-1"))
	err = check("NEW-1", "NEW-2")
	requireLimitError(t, err, 2, 3)
	var lim *teamstate.InProgressLimitError
	require.ErrorAs(t, err, &lim)
	assert.Equal(t, 2, lim.Adding)

	// ticketRoom wires it to the team of the project, for the active member.
	room := ticketRoom(a, &a.Projects.(*mockProjectStore).projects[0])
	require.NotNil(t, room)
	requireLimitError(t, room(ctx, claimsProject, []string{"NEW-1", "NEW-2"}), 2, 3)
	assert.NoError(t, room(ctx, claimsProject, []string{"NEW-1"}))
	setTeamLimit(t, repo, intp(0))
	assert.NoError(t, room(ctx, claimsProject, []string{"NEW-1", "NEW-2", "NEW-3"}), "0 is no limit")
	assert.Nil(t, ticketRoom(a, nil), "no project, no team")
}

// The check never blocks on what it cannot read.
func TestInProgressRoomIgnoresAnUnreadableTeamState(t *testing.T) {
	a, repo := wipTeam(t, 1, 1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assert.NoError(t, checkInProgressRoom(ctx, repo, a.Config.Tracker, "testuser", teamstate.ClaimRef{Project: claimsProject, TicketID: "X"}), "a cancelled read is not a refusal")
	assert.NoError(t, checkInProgressRoom(t.Context(), repo, a.Config.Tracker, "", teamstate.ClaimRef{Project: claimsProject, TicketID: "X"}), "no member")
}
