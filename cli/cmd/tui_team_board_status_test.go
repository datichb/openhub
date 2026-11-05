package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The card dropped on the TODO column of the default board is moved to the
// status planned (the team-state knows statuses, not columns); a transition
// the team-state refuses is said as such, not as an unknown status "todo".
func TestTeamBoardStatusResolvesTheColumn(t *testing.T) {
	a, repo := wipTeam(t, 0, 0)
	actions := buildTeamBoardViewConfig(a).Actions
	require.NotNil(t, actions)
	claimsCreate(t, repo, "S-1", "testuser", "")

	err := actions.OnStatus(claimsProject, "S-1", "todo")
	require.ErrorIs(t, err, teamstate.ErrInvalidTransition, "in progress does not go back to planned")
	assert.NotErrorIs(t, err, teamstate.ErrInvalidStatus)

	err = actions.OnStatus(claimsProject, "S-1", "no-such-column")
	require.ErrorIs(t, err, teamstate.ErrInvalidStatus)

	require.NoError(t, actions.OnStatus(claimsProject, "S-1", teamstate.ClaimStatusReview))
	assert.Equal(t, teamstate.ClaimStatusReview, claimsOnDisk(t, repo, "S-1").Status)
}

// On a board of its own, a card is moved to the column the team named, and the
// ticket taken from the board goes to the first column of work of that board,
// not to the status in_progress, which it may not have.
func TestTeamBoardUsesTheColumnsOfTheBoard(t *testing.T) {
	a, repo := wipTeam(t, 0, 0)
	team, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	team.Board.Columns = []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: teamstate.ColumnRoleActive},
		{ID: "qa", Name: "QA", Role: teamstate.ColumnRoleActive},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
	}
	require.NoError(t, repo.SaveConfig(t.Context(), team))
	actions := buildTeamBoardViewConfig(a).Actions
	require.NotNil(t, actions)

	require.NoError(t, actions.OnClaim(claimsProject, "C-1"))
	assert.Equal(t, "dev", claimsOnDisk(t, repo, "C-1").Status, "the first column of work of the board")

	require.NoError(t, actions.OnStatus(claimsProject, "C-1", "qa"))
	assert.Equal(t, "qa", claimsOnDisk(t, repo, "C-1").Status)
	require.NoError(t, actions.OnStatus(claimsProject, "C-1", "backlog"))
	assert.Equal(t, "backlog", claimsOnDisk(t, repo, "C-1").Status, "a custom board may go back")
}
