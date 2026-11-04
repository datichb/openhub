package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The editor lists a label under the column whose ID it is mapped to: 'planned'
// (what the config holds for the initial column of a board without columns)
// is listed under the initial column of the board it edits.
func TestColumnEditorListsPlannedUnderTheInitialColumn(t *testing.T) {
	labels := map[string]string{"Ready": "planned", "Doing": "in_progress"}
	statuses := map[string]string{"Open": "planned"}
	listUnderTheirColumns(teamstate.DefaultBoardConfig().Columns, labels, statuses)
	assert.Equal(t, "todo", labels["Ready"])
	assert.Equal(t, "in_progress", labels["Doing"])
	assert.Equal(t, "todo", statuses["Open"])

	labels = map[string]string{"Ready": "planned"}
	listUnderTheirColumns([]teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "B", Role: teamstate.ColumnRoleInitial}, {ID: "end", Name: "E", Role: teamstate.ColumnRoleTerminal},
	}, labels)
	assert.Equal(t, "backlog", labels["Ready"])
}

// A column the team renamed keeps its labels: a mapping to the former 'todo' is
// the status of the initial column, and the board is valid for the config.
func TestApplyBoardColumnsReadsTodoAsTheInitialColumn(t *testing.T) {
	cfg := &teamstate.TeamConfig{}
	columns := []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "BACKLOG", Role: teamstate.ColumnRoleInitial},
		{ID: "doing", Name: "DOING", Role: teamstate.ColumnRoleActive},
		{ID: "done", Name: "DONE", Role: teamstate.ColumnRoleTerminal},
	}
	applyBoardColumns(cfg, columns, map[string]string{"Ready": "todo", "Doing": "doing"}, map[string]string{})
	assert.Equal(t, "backlog", cfg.Tracker.LabelStatusMapping["Ready"])
	assert.Equal(t, "doing", cfg.Tracker.LabelStatusMapping["Doing"])
	assert.NoError(t, cfg.Tracker.ValidateMappings(cfg.Board))
}

// A mapping to a column that is no longer there stays, and the save names it.
func TestApplyBoardColumnsLeavesWhatStandsForNothingToBeRefused(t *testing.T) {
	cfg := &teamstate.TeamConfig{}
	applyBoardColumns(cfg, []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "B", Role: teamstate.ColumnRoleInitial}, {ID: "done", Name: "D", Role: teamstate.ColumnRoleTerminal},
	}, map[string]string{"Odd": "qa"}, nil)
	assert.Equal(t, "qa", cfg.Tracker.LabelStatusMapping["Odd"])
	assert.Error(t, cfg.Tracker.ValidateMappings(cfg.Board))
}
