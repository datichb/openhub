package teamstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func claimsIn(statuses ...string) []Claim {
	out := make([]Claim, len(statuses))
	for i, s := range statuses {
		out[i] = Claim{TicketID: s, Status: s}
	}
	return out
}

func TestCountByRoleFollowsTheBoard(t *testing.T) {
	t.Parallel()
	// The default board: in progress and validation are work.
	assert.Equal(t, RoleCounts{Active: 2, Review: 1, Blocked: 1},
		CountByRole(claimsIn("in_progress", "validation", "review", "blocked", "planned", "done"), BoardConfig{}))

	// A board of its own: its roles, whatever the names.
	board := BoardConfig{Columns: []BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: ColumnRoleActive},
		{ID: "qa", Name: "QA"},
		{ID: "mr", Name: "MR", Role: ColumnRoleReview},
		{ID: "stuck", Name: "Stuck", Role: ColumnRoleBlocked},
		{ID: "shipped", Name: "Shipped", Role: ColumnRoleTerminal},
	}}
	assert.Equal(t, RoleCounts{Active: 2, Review: 1, Blocked: 1},
		CountByRole(claimsIn("dev", "qa", "mr", "stuck", "backlog", "shipped", "in_progress", "planned"), board),
		"in_progress is not a status of this board")

	// A saved default board without the role: the review status stands in.
	old := DefaultBoardConfig()
	for i := range old.Columns {
		if old.Columns[i].ID == "review" {
			old.Columns[i].Role = ColumnRoleActive
		}
	}
	assert.Equal(t, RoleCounts{Active: 1, Review: 1}, CountByRole(claimsIn("in_progress", "review"), old))
}
