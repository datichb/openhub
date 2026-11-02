package views

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The notice line of the team board (the auto-plan limit left issues off the
// board) sits on the line of the project tabs, whether the tabs are shown or not.
func TestTeamBoardShowsTheNotice(t *testing.T) {
	notice := ""
	tickets := []TeamTicket{
		{ID: "1", Title: "One", Status: "in_progress", Assignee: "alice", Project: "a"},
		{ID: "2", Title: "Two", Status: "in_progress", Assignee: "alice", Project: "b"},
	}
	board := NewTeamBoardView(TeamBoardViewConfig{
		Tickets:      tickets[:1],
		IsConfigured: func() bool { return true },
		Notice:       func() string { return notice },
	})
	board.Mount(tview.NewFlex().SetDirection(tview.FlexRow), tview.NewApplication())
	defer board.Unmount()
	line := func() string { return board.tabBar.GetText(true) }

	assert.Empty(t, line(), "no notice, one project: nothing")

	notice = "3 issue(s) outside the board [x]"
	board.populateColumns(tickets[:1], board.allColumns)
	assert.Contains(t, line(), "3 issue(s) outside the board [x]", "one project: the notice alone")

	board.populateColumns(tickets, board.allColumns)
	require.Greater(t, len(board.projectTabs), 2)
	assert.Contains(t, line(), "a", "the tabs are still there")
	assert.Contains(t, line(), "3 issue(s) outside the board [x]", "and the notice after them")

	notice = ""
	board.populateColumns(tickets, board.allColumns)
	assert.NotContains(t, line(), "outside the board")
}
