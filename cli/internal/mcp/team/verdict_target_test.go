package team

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The verdict leads to the statuses of the board of the team.
func TestVerdictTargetFollowsTheBoard(t *testing.T) {
	assert.Equal(t, "done", verdictTarget(teamstate.BoardConfig{}, true))
	assert.Equal(t, "in_progress", verdictTarget(teamstate.BoardConfig{}, false))
	board := teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: teamstate.ColumnRoleActive},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
	}}
	assert.Equal(t, "shipped", verdictTarget(board, true))
	assert.Equal(t, "dev", verdictTarget(board, false))
}
