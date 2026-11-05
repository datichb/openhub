package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The board says who holds a ticket instead of announcing a claim that was not
// made, and takes over a claim that has expired, with a brief and an event.
func TestBoardTakesOverAnExpiredClaimAndNamesTheOwnerOfALiveOne(t *testing.T) {
	a, repo := setupBeat(t)
	i18ntest.Set(t, "en")
	addTeamMembers(t, repo, "alice")
	actions := buildTeamBoardViewConfig(a).Actions
	require.NotNil(t, actions)
	expiredClaimOf(t, repo, "EXP-1", "alice")
	claimsCreate(t, repo, "LIVE-1", "alice", "")

	var held *views.ClaimHeldError
	require.ErrorAs(t, actions.OnClaim(claimsProject, "EXP-1"), &held)
	assert.Equal(t, "alice", held.Owner)
	assert.True(t, held.Expired, "the board offers to take it over")
	require.ErrorAs(t, actions.OnClaim(claimsProject, "LIVE-1"), &held)
	assert.False(t, held.Expired)

	held = nil
	require.ErrorAs(t, actions.OnTakeOver(claimsProject, "LIVE-1"), &held, "a live claim is not taken over")
	assert.Equal(t, "alice", claimsOnDisk(t, repo, "LIVE-1").ClaimedBy)

	require.NoError(t, actions.OnTakeOver(claimsProject, "EXP-1"))
	assert.Equal(t, "testuser", claimsOnDisk(t, repo, "EXP-1").ClaimedBy)
	var transferred []teamstate.Event
	for _, e := range eventsOn(t, repo, claimsProject) {
		if e.Type == teamstate.EventClaimTransferred {
			transferred = append(transferred, e)
		}
	}
	require.Len(t, transferred, 1)
	assert.Equal(t, "testuser", transferred[0].Actor)
	assert.Equal(t, "expired", transferred[0].Data["reason"])
	brief, err := repo.ReadBrief(context.Background(), claimsProject, "EXP-1")
	require.NoError(t, err)
	assert.Contains(t, brief, "alice")
}

// Release, transfer and take from the board leave their events, signed by the
// member of the board.
func TestBoardActionsLeaveEventsSignedByTheMember(t *testing.T) {
	a, repo := setupBeat(t)
	i18ntest.Set(t, "en")
	addTeamMembers(t, repo, "alice", "bob")
	actions := buildTeamBoardViewConfig(a).Actions
	claimsCreate(t, repo, "A-1", "alice", "")
	claimsCreate(t, repo, "A-2", "alice", "")

	require.NoError(t, actions.OnRelease(claimsProject, "A-1"))
	require.NoError(t, actions.OnTransfer(claimsProject, "A-2", "bob"))
	require.NoError(t, actions.OnClaim(claimsProject, "NEW-1"))

	byType := map[string]teamstate.Event{}
	for _, e := range eventsOn(t, repo, claimsProject) {
		byType[e.Type] = e
	}
	require.Contains(t, byType, teamstate.EventClaimReleased)
	assert.Equal(t, "testuser", byType[teamstate.EventClaimReleased].Actor)
	assert.Equal(t, "alice", byType[teamstate.EventClaimReleased].Data["owner"])
	require.Contains(t, byType, teamstate.EventClaimTransferred)
	assert.Equal(t, "testuser", byType[teamstate.EventClaimTransferred].Actor)
	require.Contains(t, byType, teamstate.EventClaimTaken)
	assert.Equal(t, "testuser", byType[teamstate.EventClaimTaken].Actor)

	err := actions.OnTransfer(claimsProject, "A-2", "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ghost is not a card of the team")
}
