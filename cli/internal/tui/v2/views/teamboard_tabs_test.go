package views

import (
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// hintShell records the hints line of the board each time the shell is asked
// to show it again: what the user sees right after an action.
type hintShell struct {
	choosingShell
	board *TeamBoardView
	seen  []string
}

func (s *hintShell) RefreshHints() { s.seen = append(s.seen, s.board.StatusHints()) }

func (s *hintShell) lastHints() string {
	if len(s.seen) == 0 {
		return ""
	}
	return s.seen[len(s.seen)-1]
}

func tabsTickets() []TeamTicket {
	return []TeamTicket{
		{ID: "1", Title: "One", Status: "todo", Project: "88280c5b", ProjectName: "transparence-sru", AssigneeID: "alice", Assignee: "Alice"},
		{ID: "2", Title: "Two", Status: "todo", Project: "a1b2c3d4", ProjectName: "alpha", AssigneeID: "bob", Assignee: "Bob"},
		{ID: "3", Title: "Three", Status: "todo", Project: "deadbeef", ProjectName: "deadbeef"},
		{ID: "4", Title: "Four", Status: "todo", Project: "88280c5b", ProjectName: "transparence-sru"},
	}
}

func tabsBoard(t *testing.T, tickets []TeamTicket) (*TeamBoardView, *hintShell) {
	t.Helper()
	i18ntest.Set(t, "en")
	board := NewTeamBoardView(TeamBoardViewConfig{
		Tickets: tickets, IsConfigured: func() bool { return true },
		MemberID: func() string { return "alice" },
	})
	board.Mount(tview.NewFlex().SetDirection(tview.FlexRow), nil)
	t.Cleanup(board.Unmount)
	sh := &hintShell{board: board}
	board.SetShell(sh)
	board.populateColumns(tickets, board.allColumns)
	return board, sh
}

func tabText(b *TeamBoardView) string { return b.tabBar.GetText(true) }

// The tabs show the name of the project and filter on its ID; they are sorted
// by name, in the same order whatever the order of the tickets.
func TestTeamBoardTabsShowTheNameOfTheProject(t *testing.T) {
	board, _ := tabsBoard(t, tabsTickets())
	assert.Equal(t, []string{"", "a1b2c3d4", "deadbeef", "88280c5b"}, board.projectTabs, "the IDs, by name")
	text := tabText(board)
	assert.Contains(t, text, "transparence-sru")
	assert.Contains(t, text, "alpha")
	assert.NotContains(t, text, "88280c5b", "the ID is not what the tab shows")
	assert.NotContains(t, text, "a1b2c3d4")
	assert.Less(t, strings.Index(text, "alpha"), strings.Index(text, "transparence-sru"), "sorted by name")

	reversed := tabsTickets()
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	other, _ := tabsBoard(t, reversed)
	assert.Equal(t, board.projectTabs, other.projectTabs, "the order does not depend on the tickets")

	// The tab filters on the ID: the tickets of the project, whatever its name.
	board.activeTabIdx = 3
	assert.Equal(t, []string{"1", "4"}, ticketIDs(board.applyProjectFilter(board.allTickets)))
}

func ticketIDs(tickets []TeamTicket) []string {
	var out []string
	for _, t := range tickets {
		out = append(out, t.ID)
	}
	return out
}

// An ID no local project has (the resolver gave it back as the name) is marked
// as such, in both languages; a project that has a name is not.
func TestTeamBoardTabsMarkAnUnknownProject(t *testing.T) {
	board, _ := tabsBoard(t, tabsTickets())
	text := tabText(board)
	assert.Contains(t, text, "deadbeef (unknown project)")
	assert.NotContains(t, text, "alpha (unknown")
	assert.NotContains(t, text, "transparence-sru (unknown")

	i18ntest.Set(t, "fr")
	board.renderTabBar()
	assert.Contains(t, tabText(board), "deadbeef (projet inconnu)")

	// Without a resolver (no name at all) the tab shows the ID, unmarked.
	bare := []TeamTicket{{ID: "1", Status: "todo", Project: "p1"}, {ID: "2", Status: "todo", Project: "p2"}}
	b2, _ := tabsBoard(t, bare)
	assert.Contains(t, tabText(b2), "p1")
	assert.NotContains(t, tabText(b2), "inconnu")
	assert.NotContains(t, tabText(b2), "unknown")
}

// [ and ] filter the cards at once, and the active tab follows at once too,
// not at the next read of the tickets.
func TestTeamBoardActiveTabFollowsTheKeysAtOnce(t *testing.T) {
	board, sh := tabsBoard(t, tabsTickets())
	active := func(label string) bool { return strings.Contains(board.tabBar.GetText(false), projectChip(label)) }
	require.True(t, active("All"), "the « All » tab is active at first")

	board.HandleKey(runeKey(']'))
	assert.True(t, active("alpha"), "after ], the next tab is the active one")
	assert.False(t, active("All"))
	assert.Equal(t, []string{"2"}, visible(board), "and its cards are shown")

	board.HandleKey(runeKey(']'))
	assert.True(t, active("deadbeef (unknown project)"))
	board.HandleKey(runeKey('['))
	board.HandleKey(runeKey('['))
	assert.True(t, active("All"), "[ goes back")
	board.HandleKey(runeKey('['))
	assert.True(t, active("transparence-sru"), "and wraps around")
	assert.Equal(t, []string{"1", "4"}, visible(board))
	assert.NotEmpty(t, sh.seen, "the hints line was shown again")
}

// The « filtered » mark of the hints line appears with the filter and goes with
// it, at once, for every way of changing the filters.
func TestTeamBoardFilteredMarkFollowsTheFiltersAtOnce(t *testing.T) {
	board, sh := tabsBoard(t, tabsTickets())
	filtered := func() bool { return strings.Contains(sh.lastHints(), "FILTERED") }

	sh.choice = "_mine"
	board.showFilterMenu()
	assert.True(t, filtered(), "My tickets")
	assert.Equal(t, []string{"1"}, visible(board))

	sh.choice = "_clear"
	board.showFilterMenu()
	assert.False(t, filtered(), "Clear the filters")

	sh.choice = "_pool"
	board.showFilterMenu()
	assert.True(t, filtered(), "Unassigned")
	assert.Equal(t, []string{"3", "4"}, visible(board))
	sh.choice = "_clear"
	board.showFilterMenu()
	assert.False(t, filtered())

	// The text search, the assignee and the label go through the same path.
	board.filterText = "two"
	board.filtersChanged()
	assert.True(t, filtered(), "text")
	board.clearFilters()
	assert.False(t, filtered())
	board.filterAssignee = "Bob"
	board.filtersChanged()
	assert.True(t, filtered(), "assignee")
	board.clearFilters()
	board.filterLabel = "bug"
	board.filtersChanged()
	assert.True(t, filtered(), "label")
	board.clearFilters()
	assert.False(t, filtered())
}

// The detail of a ticket names the project; the ID of a project this machine
// does not know is marked, like on its tab.
func TestTeamBoardTicketDetailNamesTheProject(t *testing.T) {
	board, _ := tabsBoard(t, tabsTickets())
	known := board.formatTicketDetail(&TeamTicket{ID: "1", Title: "One", Project: "88280c5b", ProjectName: "transparence-sru"})
	assert.Contains(t, known, "transparence-sru")
	assert.NotContains(t, known, "88280c5b")

	unknown := board.formatTicketDetail(&TeamTicket{ID: "3", Title: "Three", Project: "deadbeef", ProjectName: "deadbeef"})
	assert.Contains(t, unknown, "deadbeef (unknown project)")

	i18ntest.Set(t, "fr")
	assert.Contains(t, board.formatTicketDetail(&TeamTicket{ID: "3", Project: "deadbeef", ProjectName: "deadbeef"}), "deadbeef (projet inconnu)")

	none := board.formatTicketDetail(&TeamTicket{ID: "9", Title: "No project"})
	assert.NotContains(t, none, "inconnu")
}

// The toast of a transfer names the member as the list did, not by their ID.
func TestTeamBoardTransferToastNamesTheMember(t *testing.T) {
	i18ntest.Set(t, "en")
	tickets := []TeamTicket{{ID: "1", Title: "One", Status: "todo", Project: "p", AssigneeID: "alice", Assignee: "Alice"}}
	var transferred string
	board := NewTeamBoardView(TeamBoardViewConfig{
		Tickets: tickets, IsConfigured: func() bool { return true },
		Actions: &BoardActions{
			Members: func() []SelectOption {
				return []SelectOption{{Label: "Bob Builder", Value: "bob"}, {Label: "Alice", Value: "alice"}}
			},
			OnTransfer: func(_, _, to string) error { transferred = to; return nil },
		},
	})
	content := tview.NewFlex()
	app := runApp(t, content)
	sh := &choosingShell{choice: "bob"}
	onLoop(app, func() {
		board.Mount(content, app)
		board.SetShell(sh)
		board.populateColumns(tickets, board.allColumns)
		board.transferTicket()
	})
	require.Eventually(t, func() bool {
		var n int
		onLoop(app, func() { sh.mu.Lock(); n = len(sh.toasts); sh.mu.Unlock() })
		return n > 0
	}, 3*time.Second, 20*time.Millisecond)
	onLoop(app, board.Unmount)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	assert.Equal(t, "bob", transferred, "the ID is what the action takes")
	assert.Contains(t, sh.toasts[0], "Bob Builder")
	assert.NotContains(t, sh.toasts[0], "bob")
}
