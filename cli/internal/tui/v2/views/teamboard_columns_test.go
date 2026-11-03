package views

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func threeColumns() teamstate.BoardConfig {
	return teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "todo", Name: "To do", Role: teamstate.ColumnRoleInitial},
		{ID: "doing", Name: "Doing", Role: teamstate.ColumnRoleActive},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
	}}
}

// C opens the editor of the columns (the action of the Team mode), the hint
// and the help name it, and nothing happens without an editor to open.
func TestTeamBoardCOpensTheColumnEditor(t *testing.T) {
	i18ntest.Set(t, "en")
	opened := 0
	var onSaved func()
	board := NewTeamBoardView(TeamBoardViewConfig{
		IsConfigured:  func() bool { return true },
		OnBoardConfig: func(done func()) { opened++; onSaved = done },
	})
	board.Mount(tview.NewFlex().SetDirection(tview.FlexRow), nil)
	t.Cleanup(board.Unmount)

	assert.Nil(t, board.HandleKey(runeKey('C')), "C is the board's")
	assert.Equal(t, 1, opened)
	require.NotNil(t, onSaved)
	assert.Contains(t, board.StatusHints(), "C board columns", "the hint bar, and the ? overlay that shows it")

	// Not C: the board's other keys are as before.
	assert.NotNil(t, board.HandleKey(runeKey('Z')))
	assert.Equal(t, 1, opened)

	// Without an editor to open, C does nothing.
	bare := NewTeamBoardView(TeamBoardViewConfig{IsConfigured: func() bool { return true }})
	bare.Mount(tview.NewFlex().SetDirection(tview.FlexRow), nil)
	t.Cleanup(bare.Unmount)
	assert.Nil(t, bare.HandleKey(runeKey('C')))
}

// Once the editor saved, the board reads its tickets again and takes the new
// columns: the tickets are in them, the old ones are gone, and the focus stays
// inside the columns.
func TestTeamBoardTakesTheColumnsTheEditorSaved(t *testing.T) {
	i18ntest.Set(t, "en")
	layout := BoardLayout{}
	var onSaved func()
	tickets := []TeamTicket{
		{ID: "1", Title: "One", Status: "todo", Project: "p"},
		{ID: "2", Title: "Two", Status: "in_progress", Project: "p"},
	}
	board := NewTeamBoardView(TeamBoardViewConfig{
		IsConfigured:  func() bool { return true },
		Tickets:       tickets,
		Layout:        func() BoardLayout { return layout },
		OnBoardConfig: func(done func()) { onSaved = done },
	})
	board.Mount(tview.NewFlex().SetDirection(tview.FlexRow), nil)
	t.Cleanup(board.Unmount)
	require.Len(t, board.columnCards, 6, "the default board")
	assert.Equal(t, []string{"1"}, board.ticketIDLookup[0])

	// The editor saves 3 columns, the labels mapped to them are new too.
	board.HandleKey(runeKey('C'))
	require.NotNil(t, onSaved)
	board.focusCol = 5
	layout = BoardLayout{Board: threeColumns(), LabelStatusMapping: map[string]string{"wip": "doing"}}
	board.cfg.RefreshFunc = func() []TeamTicket {
		return []TeamTicket{
			{ID: "1", Title: "One", Status: "todo", Project: "p"},
			{ID: "2", Title: "Two", Status: "doing", Project: "p"},
			{ID: "3", Title: "Three", Status: "shipped", Project: "p"},
		}
	}
	onSaved()

	require.Len(t, board.columnCards, 3, "the board takes the 3 columns")
	assert.Equal(t, []string{"todo", "doing", "shipped"}, []string{board.allColumns[0].Status, board.allColumns[1].Status, board.allColumns[2].Status})
	assert.Equal(t, [][]string{{"1"}, {"2"}, {"3"}}, board.ticketIDLookup, "each ticket in its column")
	assert.Equal(t, 2, board.focusCol, "the focus is kept inside the columns")
	assert.Equal(t, layout.LabelStatusMapping, board.cfg.LabelStatusMapping)
	assert.Equal(t, 3, board.columnFlex.GetItemCount()-2, "3 columns and their 2 separators are on the board")

	// A read that brings the same columns changes nothing on the board.
	cards := board.columnCards
	board.reload(nil)
	assert.Equal(t, cards, board.columnCards)

	// The next mount starts from the columns of the layout.
	board.Unmount()
	board.Mount(tview.NewFlex().SetDirection(tview.FlexRow), nil)
	require.Len(t, board.columnCards, 3)
}
