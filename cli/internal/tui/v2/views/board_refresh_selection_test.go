package views

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A column taller than the screen is scrolled: the refresh of the board (every
// few seconds, and at the arrival of the beads summary) draws the tickets
// again, and must leave the user on the card they are on, in the column they
// are in, scrolled where they are, instead of taking them back to the top.

const (
	refreshBoardW = 120
	refreshBoardH = 40
	// The last column of the default board, the farthest one: the window of
	// the visible columns has been scrolled to reach it.
	refreshStatus = "blocked"
)

func refreshID(i int) string { return fmt.Sprintf("tk-%02d", i) }

// teamRefreshTickets are n tickets of the last column, tk-00 first.
func teamRefreshTickets(n int) []TeamTicket {
	tickets := make([]TeamTicket, n)
	for i := range tickets {
		tickets[i] = TeamTicket{ID: refreshID(i), Title: "Ticket " + refreshID(i), Status: refreshStatus, Project: "p"}
	}
	return tickets
}

// refreshTeamBoard mounts a team board whose reads give *data, scrolled to the
// last column. refresh draws the new data and waits for it.
type refreshTeamBoard struct {
	t       *testing.T
	app     *tview.Application
	content *tview.Flex
	v       *TeamBoardView
	data    atomic.Pointer[[]TeamTicket]
	summary atomic.Int32
}

func newRefreshTeamBoard(t *testing.T, tickets []TeamTicket) *refreshTeamBoard {
	t.Helper()
	b := &refreshTeamBoard{t: t}
	b.data.Store(&tickets)
	b.v = NewTeamBoardView(TeamBoardViewConfig{
		RefreshRate:  time.Hour,
		IsConfigured: func() bool { return true },
		Tickets:      tickets,
		RefreshFunc:  func() []TeamTicket { return *b.data.Load() },
		BeadsSummaryFunc: func() map[string]BeadsSummary {
			return map[string]BeadsSummary{refreshID(1): {Done: 1, Total: int(b.summary.Add(1)) + 1}}
		},
	})
	b.v.SetShell(&mockShell{})
	b.content = tview.NewFlex()
	b.app = runApp(t, b.content)
	onLoop(b.app, func() { b.v.Mount(b.content, b.app) })
	waitFor(t, b.app, func() bool { return b.v.loaded })
	onLoop(b.app, func() {
		for range len(b.v.columnCards) - 1 {
			b.v.moveFocus(1)
		}
	})
	return b
}

// down moves the selection n cards down, drawing the screen like the user's
// keys do.
func (b *refreshTeamBoard) down(n int) {
	b.t.Helper()
	for range n {
		onLoop(b.app, func() {
			b.v.columnCards[b.v.focusCol].MoveSelection(1)
			_ = screenText(b.t, b.content, refreshBoardW, refreshBoardH)
		})
	}
}

func (b *refreshTeamBoard) reload(tickets []TeamTicket) {
	b.t.Helper()
	b.data.Store(&tickets)
	onLoop(b.app, func() { b.v.reload(nil) })
	want := tickets[0].ID
	waitFor(b.t, b.app, func() bool { return b.v.allTickets[0].ID == want && len(b.v.allTickets) == len(tickets) })
}

func (b *refreshTeamBoard) reloadSummary() {
	b.t.Helper()
	before := b.summary.Load()
	onLoop(b.app, b.v.loadSummary)
	waitFor(b.t, b.app, func() bool { return b.summary.Load() > before && b.v.getBeadsSummary() != nil })
	// The summary is stored, and the board drawn again, in one step on the loop.
	onLoop(b.app, func() {})
}

// check asserts the selected ticket, that it is on the screen, and that the
// top of the column is not (the board has not gone back to the top).
func (b *refreshTeamBoard) check(selected, project string, top string) {
	b.t.Helper()
	var id, proj string
	var focus, start int
	screen := drawn(b.t, b.app, b.content, refreshBoardW, refreshBoardH)
	onLoop(b.app, func() {
		id = b.v.selectedTicketID()
		if tk, ok := b.v.selectedTeamTicket(); ok {
			proj = tk.Project
		}
		focus, start = b.v.focusCol, b.v.colViewStart
	})
	assert.Equal(b.t, selected, id, "the card the user is on")
	assert.Equal(b.t, project, proj)
	assert.Contains(b.t, screen, selected, "the selected card is on the screen")
	if top != "" {
		assert.NotContains(b.t, screen, top, "the column is not back at the top")
	}
	assert.Equal(b.t, len(b.v.columnCards)-1, focus, "the focused column")
	assert.Equal(b.t, len(b.v.columnCards)-b.v.visibleCols, start, "the window of columns")
}

func TestTeamBoardRefreshKeepsTheSelectionAndTheScroll(t *testing.T) {
	frLocale(t)
	b := newRefreshTeamBoard(t, teamRefreshTickets(30))
	b.down(14)
	b.check(refreshID(14), "p", refreshID(0))

	b.reload(teamRefreshTickets(30))
	b.check(refreshID(14), "p", refreshID(0))

	b.reloadSummary()
	b.check(refreshID(14), "p", refreshID(0))
}

func TestTeamBoardRefreshFollowsTheCardWhenTheOthersMove(t *testing.T) {
	frLocale(t)
	b := newRefreshTeamBoard(t, teamRefreshTickets(30))
	b.down(14)

	// Two tickets arrive above: the selection stays on the same card, not on
	// the same index.
	more := append([]TeamTicket{
		{ID: "new-1", Title: "New", Status: refreshStatus, Project: "p"},
		{ID: "new-2", Title: "New", Status: refreshStatus, Project: "p"},
	}, teamRefreshTickets(30)...)
	b.reload(more)
	b.check(refreshID(14), "p", refreshID(0))
}

func TestTeamBoardRefreshKeepsTheCardOfTheRightProject(t *testing.T) {
	frLocale(t)
	var tickets []TeamTicket
	for i := range 30 {
		project := "p"
		if i%2 == 1 {
			project = "q"
		}
		// The same ID in both projects.
		tickets = append(tickets, TeamTicket{ID: refreshID(i / 2), Title: "T", Status: refreshStatus, Project: project})
	}
	b := newRefreshTeamBoard(t, tickets)
	b.down(15) // tk-07 of q
	var before string
	onLoop(b.app, func() { tk, _ := b.v.selectedTeamTicket(); before = tk.Project + "/" + tk.ID })
	require.Equal(t, "q/"+refreshID(7), before)

	reversed := make([]TeamTicket, len(tickets))
	for i, tk := range tickets {
		reversed[len(tickets)-1-i] = tk
	}
	b.reload(reversed)
	b.check(refreshID(7), "q", "")
}

func TestTeamBoardRefreshKeepsTheIndexWhenTheCardIsGone(t *testing.T) {
	frLocale(t)
	b := newRefreshTeamBoard(t, teamRefreshTickets(30))
	b.down(14)

	// The selected card is gone: the nearest index stays.
	gone := append(teamRefreshTickets(14), teamRefreshTickets(30)[15:]...)
	b.reload(gone)
	b.check(refreshID(15), "p", refreshID(0))

	// The column got shorter than the selection: the last card is selected.
	b.reload(teamRefreshTickets(10))
	b.check(refreshID(9), "p", "")
}

func TestTeamBoardFilterChangeStartsAtTheTop(t *testing.T) {
	frLocale(t)
	b := newRefreshTeamBoard(t, teamRefreshTickets(30))
	b.down(14)
	onLoop(b.app, func() { b.v.filterLabel = ""; b.v.filterText = "Ticket"; b.v.filtersChanged() })
	var id string
	onLoop(b.app, func() { id = b.v.selectedTicketID() })
	assert.Equal(t, refreshID(0), id, "a new filter shows the column from its top")
	assert.Contains(t, drawn(t, b.app, b.content, refreshBoardW, refreshBoardH), refreshID(0))
}

func TestBoardViewRefreshKeepsTheSelectionAndTheScroll(t *testing.T) {
	frLocale(t)
	tickets := func(n int, prefix ...BoardTicket) []BoardTicket {
		out := append([]BoardTicket{}, prefix...)
		for i := range n {
			tk := boardTicket(refreshID(i))
			tk.Status = refreshStatus
			out = append(out, tk)
		}
		return out
	}
	initial := tickets(30)
	var data atomic.Pointer[[]BoardTicket]
	data.Store(&initial)
	v := NewBoardView(BoardViewConfig{
		RefreshRate: time.Hour,
		Tickets:     initial,
		RefreshFunc: func() []BoardTicket { return *data.Load() },
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() {
		for range len(v.columnCards) - 1 {
			v.moveFocus(1)
		}
		for range 14 {
			v.columnCards[v.focusCol].MoveSelection(1)
			_ = screenText(t, content, refreshBoardW, refreshBoardH)
		}
	})

	check := func(selected, top string) {
		t.Helper()
		var id string
		var focus int
		screen := drawn(t, app, content, refreshBoardW, refreshBoardH)
		onLoop(app, func() { id, focus = v.selectedTicketID(), v.focusCol })
		assert.Equal(t, selected, id, "the card the user is on")
		assert.Contains(t, screen, selected, "the selected card is on the screen")
		if top != "" {
			assert.NotContains(t, screen, top, "the column is not back at the top")
		}
		assert.Equal(t, len(v.columnCards)-1, focus, "the focused column")
	}
	reload := func(next []BoardTicket, marker string) {
		t.Helper()
		data.Store(&next)
		onLoop(app, v.reload)
		waitFor(t, app, func() bool { _, ok := v.ticketsByID[marker]; return ok && len(v.ticketsByID) == len(next) })
	}

	check(refreshID(14), refreshID(0))

	reload(tickets(30), refreshID(29))
	check(refreshID(14), refreshID(0))

	// Tickets arrive above the selection: it stays on its card.
	n1, n2 := boardTicket("new-1"), boardTicket("new-2")
	n1.Status, n2.Status = refreshStatus, refreshStatus
	reload(tickets(30, n1, n2), "new-2")
	check(refreshID(14), refreshID(0))

	// The selected card is gone: the nearest index (16, under the two new
	// tickets) stays, then the last card.
	reload(append(tickets(14), tickets(30)[15:]...), refreshID(29))
	check(refreshID(17), refreshID(0))
	reload(tickets(10), refreshID(9))
	check(refreshID(9), "")
}
