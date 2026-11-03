package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The team board reads the columns and the label mapping of the team config
// with each read of the tickets: columns saved by the editor (the C key) or
// by a colleague are the board's at the next read, and the tickets are
// mapped to them.
func TestTeamBoardFollowsTheColumnsOfTheTeamConfig(t *testing.T) {
	i18ntest.Set(t, "en")
	a, repo := setupBeat(t)
	claimsCreate(t, repo, "T-1", "testuser", "")
	cfg := buildTeamBoardViewConfig(a)
	require.NotNil(t, cfg.Layout)
	require.NotNil(t, cfg.OnBoardConfig, "the C key opens the editor of the Team mode")
	assert.Empty(t, cfg.Layout().Board.Columns, "the default board")
	require.NotNil(t, cfg.MemberID)
	assert.Equal(t, "testuser", cfg.MemberID(), "the member of the active team, for « Mes tickets »")

	team, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	team.Board.Columns = []teamstate.BoardColumnConfig{
		{ID: "todo", Name: "To do", Role: teamstate.ColumnRoleInitial},
		{ID: "in_progress", Name: "Doing", Role: teamstate.ColumnRoleActive},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
	}
	team.Tracker.LabelStatusMapping = map[string]string{"wip": "in_progress"}
	require.NoError(t, repo.SaveConfig(t.Context(), team))

	tickets := cfg.RefreshFunc()
	require.Len(t, tickets, 1)
	assert.Equal(t, "in_progress", tickets[0].Status)
	l := cfg.Layout()
	require.Len(t, l.Board.Columns, 3, "the layout is the one of the read")
	assert.Equal(t, map[string]string{"wip": "in_progress"}, l.LabelStatusMapping)
}

// The « active » figure of the team mode counts the claims in the active
// columns of the board of the team, not three fixed statuses: with a board of
// its own, the claims of its columns count, and what only the default board
// calls active (a status the team has no column for) does not.
func TestTeamModeActiveCountFollowsTheColumnsOfTheBoard(t *testing.T) {
	i18ntest.Set(t, "en")
	a, repo := setupBeat(t)
	create := func(ticket, status string) {
		t.Helper()
		_, err := repo.CreateClaim(t.Context(), teamstate.Claim{TicketID: ticket, Project: claimsProject, ClaimedBy: "testuser", Status: status})
		require.NoError(t, err)
	}
	create("D-1", teamstate.ClaimStatusInProgress)
	create("D-2", teamstate.ClaimStatusReview)
	create("D-3", teamstate.ClaimStatusValidation)
	create("D-4", teamstate.ClaimStatusPlanned)
	assert.Equal(t, 3, teamModeStats(a).ActiveCount, "the default board: in progress, review and validation")

	team, err := repo.LoadConfig(t.Context())
	require.NoError(t, err)
	team.Board.Columns = []teamstate.BoardColumnConfig{
		{ID: "todo", Name: "To do", Role: teamstate.ColumnRoleInitial},
		{ID: "in_progress", Name: "Doing", Role: teamstate.ColumnRoleActive},
		{ID: "qa", Name: "QA", Role: teamstate.ColumnRoleActive},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
		{ID: "stuck", Name: "Stuck", Role: teamstate.ColumnRoleBlocked},
	}
	require.NoError(t, repo.SaveConfig(t.Context(), team))
	create("D-5", "qa")
	create("D-6", "shipped")
	create("D-7", "stuck")
	stats := teamModeStats(a)
	assert.Equal(t, 2, stats.ActiveCount, "in_progress and qa: the active columns of this board (review is not one)")
	assert.Equal(t, 1, stats.MemberCount)
}
