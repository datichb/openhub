package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

func rolesOf(cols []BoardColumnDef) map[string]string {
	out := map[string]string{}
	for _, c := range cols {
		out[c.Status] = c.Role
	}
	return out
}

// The columns carry the role of the board config, which the actions of a card
// depend on; a column with none is active.
func TestColumnsCarryTheirRole(t *testing.T) {
	t.Parallel()
	assert.Equal(t, map[string]string{
		"todo": "initial", "in_progress": "active", "review": "review", "validation": "active", "done": "terminal", "blocked": "blocked",
	}, rolesOf(DefaultColumns()), "the default board")

	cols := ColumnsFromConfig(teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "Dev"},
		{ID: "mr", Name: "MR", Role: teamstate.ColumnRoleReview},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
	}})
	assert.Equal(t, map[string]string{"backlog": "initial", "dev": "active", "mr": "review", "shipped": "terminal"}, rolesOf(cols))
}

// When no column declares the review role, the column of the review status is
// the review column: a team whose config predates the role.
func TestReviewStatusStandsInForTheReviewRole(t *testing.T) {
	t.Parallel()
	saved := ColumnsFromConfig(teamstate.DefaultBoardConfig())
	assert.Equal(t, "review", rolesOf(saved)["review"], "the default columns saved declare it")
	assert.Equal(t, rolesOf(DefaultColumns()), rolesOf(saved), "the board of the TUI by default and the one the assistants save agree")
	assert.Equal(t, "active", rolesOf(saved)["validation"])

	// A column declares it: the review status is an ordinary active column.
	declared := ColumnsFromConfig(teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "todo", Name: "To do", Role: teamstate.ColumnRoleInitial},
		{ID: "review", Name: "Review", Role: teamstate.ColumnRoleActive},
		{ID: "qa", Name: "QA", Role: teamstate.ColumnRoleReview},
		{ID: "done", Name: "Done", Role: teamstate.ColumnRoleTerminal},
	}})
	assert.Equal(t, map[string]string{"todo": "initial", "review": "active", "qa": "review", "done": "terminal"}, rolesOf(declared))

	// No review status on the board: no review column.
	none := ColumnsFromConfig(teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "todo", Name: "To do", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "Dev"},
		{ID: "done", Name: "Done", Role: teamstate.ColumnRoleTerminal},
	}})
	assert.NotContains(t, rolesOf(none), "review")
	for _, c := range none {
		assert.NotEqual(t, teamstate.ColumnRoleReview, c.Role)
	}
}
