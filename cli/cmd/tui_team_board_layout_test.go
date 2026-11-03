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
