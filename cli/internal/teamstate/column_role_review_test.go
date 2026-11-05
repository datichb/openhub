package teamstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reviewBoard() BoardConfig {
	return BoardConfig{Columns: []BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: ColumnRoleActive},
		{ID: "mr", Name: "MR open", Role: ColumnRoleReview},
		{ID: "shipped", Name: "Shipped", Role: ColumnRoleTerminal},
	}}
}

// The review role is a role of a column like the others: the board validates
// with it, any number of columns may carry it.
func TestValidateAcceptsTheReviewRole(t *testing.T) {
	t.Parallel()
	require.NoError(t, reviewBoard().Validate())

	two := reviewBoard()
	two.Columns = append(two.Columns, BoardColumnConfig{ID: "qa", Name: "QA", Role: ColumnRoleReview})
	require.NoError(t, two.Validate(), "a team may have more than one column waiting for a review")

	bad := BoardConfig{Columns: []BoardColumnConfig{{ID: "a", Name: "A", Role: "reviewing"}, {ID: "done", Name: "Done", Role: ColumnRoleTerminal}}}
	err := bad.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initial, active, review, terminal, blocked", "the message lists the review role")
}

// ReviewStatuses is the columns that declare the role; the review status
// stands in when none does (the default board, a team that never set it).
func TestReviewStatuses(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"mr"}, reviewBoard().ReviewStatuses())
	assert.Equal(t, []string{ClaimStatusReview}, BoardConfig{}.ReviewStatuses(), "default board")
	assert.Equal(t, []string{ClaimStatusReview}, DefaultBoardConfig().ReviewStatuses(), "default columns saved: the review column declares the role")
	own := BoardConfig{Columns: []BoardColumnConfig{{ID: "a", Name: "A", Role: ColumnRoleInitial}, {ID: "z", Name: "Z", Role: ColumnRoleTerminal}}}
	assert.Equal(t, []string{ClaimStatusReview}, own.ReviewStatuses())
}

// A review column is alive (an active status) but its claims wait for a
// person: they are not work in progress, with or without a label.
func TestReviewRoleColumnIsAliveButNotInProgress(t *testing.T) {
	t.Parallel()
	board := reviewBoard()
	assert.Equal(t, []string{"dev", "mr"}, board.ActiveStatuses())
	assert.Equal(t, "dev", board.DefaultWorkStatus(), "the work status is still the first active column")

	for status, want := range map[string]bool{"backlog": false, "dev": true, "mr": false, "shipped": false} {
		c := claimOf("1", "alice", status)
		assert.Equal(t, want, c.CountsInProgress(board), status)
	}
	claims := []Claim{claimOf("1", "alice", "dev"), claimOf("2", "alice", "mr"), claimOf("3", "alice", "mr")}
	assert.Equal(t, 3, CountActive(claims, board), "the figure of active claims keeps the ones in review")
}
