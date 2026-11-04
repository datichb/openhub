package views

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// choosingShell picks the same option of every select modal, at once.
type choosingShell struct {
	recordingShell
	choice  string
	offered []string
}

func (s *choosingShell) ShowSelectModal(_ string, opts []SelectOption, _ string, pick func(string)) {
	s.offered = nil
	for _, o := range opts {
		s.offered = append(s.offered, o.Value)
	}
	pick(s.choice)
}

func filterBoard(t *testing.T, me string) (*TeamBoardView, *choosingShell) {
	t.Helper()
	i18ntest.Set(t, "en")
	tickets := []TeamTicket{
		{ID: "1", Title: "Mine", Status: "in_progress", Assignee: "Alice A.", AssigneeID: "alice", Project: "p"},
		{ID: "2", Title: "Bob's", Status: "in_progress", Assignee: "Bob B.", AssigneeID: "bob", Project: "p"},
		{ID: "3", Title: "Pool", Status: "todo", Project: "p"},
		{ID: "4", Title: "Pool too", Status: "todo", Project: "p"},
		{ID: "5", Title: "Mine, planned", Status: "todo", Assignee: "Alice A.", AssigneeID: "alice", Project: "p"},
	}
	cfg := TeamBoardViewConfig{Tickets: tickets, IsConfigured: func() bool { return true }}
	if me != "" {
		cfg.MemberID = func() string { return me }
	}
	board := NewTeamBoardView(cfg)
	board.Mount(tview.NewFlex().SetDirection(tview.FlexRow), tview.NewApplication())
	t.Cleanup(board.Unmount)
	sh := &choosingShell{}
	board.SetShell(sh)
	board.populateColumns(tickets, board.allColumns)
	return board, sh
}

// visible are the IDs of the tickets the board shows, in no particular order.
func visible(b *TeamBoardView) []string {
	var ids []string
	for _, col := range b.ticketIDLookup {
		ids = append(ids, col...)
	}
	slices.Sort(ids)
	return ids
}

// « Mes tickets » keeps the tickets held by the member who works on the board
// (their ID, not the name shown on the card, and not the first member of
// anything); « Non assignés » keeps the pool, the tickets nobody holds.
func TestTeamBoardFilterMineAndUnassigned(t *testing.T) {
	board, sh := filterBoard(t, "alice")
	assert.Equal(t, []string{"1", "2", "3", "4", "5"}, visible(board))
	assert.NotContains(t, board.StatusHints(), "FILTERED")

	sh.choice = "_mine"
	board.showFilterMenu()
	assert.Equal(t, []string{"1", "5"}, visible(board), "the tickets of alice")
	assert.Contains(t, board.StatusHints(), "FILTERED", "the hint says the board is filtered")
	assert.Contains(t, sh.toasts[len(sh.toasts)-1], "my tickets")

	sh.choice = "_pool"
	board.showFilterMenu()
	assert.Equal(t, []string{"3", "4"}, visible(board), "the pool: no owner")
	assert.Contains(t, board.StatusHints(), "FILTERED")
	assert.Contains(t, sh.toasts[len(sh.toasts)-1], "unassigned")
	assert.Equal(t, ownerPool, board.filterOwner)

	// The option is offered, with the others.
	assert.Equal(t, []string{"_text", "_mine", "_pool", "_assignee", "_label", "_clear"}, sh.offered)

	sh.choice = "_clear"
	board.showFilterMenu()
	assert.Equal(t, []string{"1", "2", "3", "4", "5"}, visible(board))
	assert.NotContains(t, board.StatusHints(), "FILTERED", "« Effacer les filtres » takes the hint off")
	assert.Equal(t, ownerAny, board.filterOwner)
}

// The member is the one of the config, whoever comes first among the tickets.
func TestTeamBoardFilterMineIsTheMemberOfTheConfig(t *testing.T) {
	board, sh := filterBoard(t, "bob")
	sh.choice = "_mine"
	board.showFilterMenu()
	assert.Equal(t, []string{"2"}, visible(board))
}

// Without an identity the filter says so and filters nothing.
func TestTeamBoardFilterMineWithoutIdentity(t *testing.T) {
	board, sh := filterBoard(t, "")
	sh.choice = "_mine"
	board.showFilterMenu()
	assert.Equal(t, []string{"1", "2", "3", "4", "5"}, visible(board), "no empty board")
	assert.NotContains(t, board.StatusHints(), "FILTERED")
	require.NotEmpty(t, sh.toasts)
	assert.Contains(t, sh.toasts[len(sh.toasts)-1], "identity unknown")
}

// Picking an assignee replaces the owner filter; the filters combine with the
// text search.
func TestTeamBoardOwnerFilterCombinesWithTheOthers(t *testing.T) {
	board, sh := filterBoard(t, "alice")
	sh.choice = "_mine"
	board.showFilterMenu()
	board.filterText = "planned"
	board.repopulateWithFilters()
	assert.Equal(t, []string{"5"}, visible(board), "mine and the text")

	board.filterText = ""
	board.filterAssignee, board.filterOwner = "Bob B.", ownerAny
	board.repopulateWithFilters()
	assert.Equal(t, []string{"2"}, visible(board))

	sh.choice = "_pool"
	board.showFilterMenu()
	assert.Equal(t, []string{"3", "4"}, visible(board), "the pool replaces the assignee")
}

// The tickets carry the ID of the member who holds the claim, and none for a
// ticket of the pool, which the filters read.
func TestFetchTeamTicketsCarriesTheIDOfTheOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("git")
	}
	bare, clone := gittest.BareClone(t)
	repo := teamstate.NewRepo(bare, clone)
	ctx := t.Context()
	require.NoError(t, repo.AddMember(ctx, teamstate.Member{ID: "alice", DisplayName: "Alice A."}))
	_, err := repo.CreateClaim(ctx, teamstate.Claim{TicketID: "T-1", Project: "p", ClaimedBy: "alice", Status: teamstate.ClaimStatusInProgress})
	require.NoError(t, err)
	_, err = repo.CreateClaim(ctx, teamstate.Claim{TicketID: "T-2", Project: "p", Status: teamstate.ClaimStatusPlanned})
	require.NoError(t, err)

	byID := map[string]TeamTicket{}
	for _, tk := range FetchTeamTickets(ctx, repo, nil) {
		byID[tk.ID] = tk
	}
	require.Len(t, byID, 2)
	assert.Equal(t, "alice", byID["T-1"].AssigneeID)
	assert.Equal(t, "Alice A.", byID["T-1"].Assignee, "the card still shows the name")
	assert.Empty(t, byID["T-2"].AssigneeID, "the pool has no owner")
}

// The assignees of the filter are listed by name, in the same order each time
// (the order of a map changes with each opening).
func TestTeamBoardAssigneeFilterIsSortedByName(t *testing.T) {
	i18ntest.Set(t, "en")
	var tickets []TeamTicket
	for i, name := range []string{"Zoé", "bob", "Alice", "Charlie", "alice b.", "David", "Eve", "Frank", "Grace"} {
		tickets = append(tickets, TeamTicket{ID: strconv.Itoa(i), Status: "todo", Assignee: name, AssigneeID: strings.ToLower(name), Project: "p"})
	}
	board := NewTeamBoardView(TeamBoardViewConfig{Tickets: tickets, IsConfigured: func() bool { return true }})
	board.Mount(tview.NewFlex().SetDirection(tview.FlexRow), tview.NewApplication())
	t.Cleanup(board.Unmount)
	sh := &choosingShell{choice: "_assignee"}
	board.SetShell(sh)
	board.populateColumns(tickets, board.allColumns)

	var first []string
	for range 5 {
		sh.offered = nil
		board.showAssigneeFilter()
		if first == nil {
			first = sh.offered
			continue
		}
		assert.Equal(t, first, sh.offered, "the same order at each opening")
	}
	assert.Equal(t, []string{"Alice", "alice b.", "bob", "Charlie", "David", "Eve", "Frank", "Grace", "Zoé"}, first)
}
