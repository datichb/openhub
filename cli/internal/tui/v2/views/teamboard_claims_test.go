package views

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// claimShell keeps the confirmation the board asks and the toasts it shows.
type claimShell struct {
	menuShell
	confirmTitle, confirmDetail string
	yes                         func()
}

func (s *claimShell) Confirm(title, detail string, _ bool, onYes func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.confirmTitle, s.confirmDetail, s.yes = title, detail, onYes
}

func (s *claimShell) lastToast() string {
	s.recordingShell.mu.Lock()
	defer s.recordingShell.mu.Unlock()
	if len(s.toasts) == 0 {
		return ""
	}
	return s.toasts[len(s.toasts)-1]
}

func claimBoard(t *testing.T, tickets []TeamTicket, actions *BoardActions) (*TeamBoardView, *claimShell, func(func())) {
	t.Helper()
	i18ntest.Set(t, "en")
	board := NewTeamBoardView(TeamBoardViewConfig{
		Tickets: tickets, IsConfigured: func() bool { return true },
		MemberID: func() string { return "me" },
		Actions:  actions,
	})
	sh := &claimShell{}
	content := tview.NewFlex()
	app := runApp(t, content)
	run := func(fn func()) { onLoop(app, fn) }
	run(func() {
		board.Mount(content, app)
		board.SetShell(sh)
		board.populateColumns(tickets, board.allColumns)
	})
	t.Cleanup(func() { run(board.Unmount) })
	return board, sh, run
}

// The take key on a ticket someone holds says who holds it; it neither claims
// nor announces a claim that was not made. On one's own ticket it says so.
func TestTeamBoardSaysWhoHoldsATicketInsteadOfClaiming(t *testing.T) {
	var mu sync.Mutex
	claims := 0
	tickets := []TeamTicket{
		{ID: "1", Title: "Held", Status: "todo", Project: "p", AssigneeID: "alice", Assignee: "Alice A."},
	}
	board, sh, run := claimBoard(t, tickets, &BoardActions{
		OnClaim: func(string, string) error { mu.Lock(); claims++; mu.Unlock(); return nil },
	})

	run(func() { board.claimTicket() })
	assert.Equal(t, "1 is already held by Alice A..", sh.lastToast())
	mu.Lock()
	assert.Zero(t, claims, "a held ticket is not claimed")
	mu.Unlock()
	assert.Nil(t, sh.yes, "a claim that has not expired is not taken over")

	own := []TeamTicket{{ID: "2", Title: "Mine", Status: "todo", Project: "p", AssigneeID: "me", Assignee: "Me"}}
	board2, sh2, run2 := claimBoard(t, own, &BoardActions{OnClaim: func(string, string) error { mu.Lock(); claims++; mu.Unlock(); return nil }})
	run2(func() { board2.claimTicket() })
	assert.Equal(t, "2 is already yours.", sh2.lastToast())
}

// A claim that has expired is taken over after a confirmation that names its
// owner.
func TestTeamBoardTakesOverAnExpiredClaimAfterAConfirmation(t *testing.T) {
	var mu sync.Mutex
	var taken []string
	tickets := []TeamTicket{{ID: "1", Title: "Stale", Status: "todo", Project: "p", AssigneeID: "alice", Assignee: "Alice A.",
		LeaseUntil: time.Now().Add(-time.Hour), Expired: true}}
	board, sh, run := claimBoard(t, tickets, &BoardActions{
		OnClaim:    func(string, string) error { t.Error("an expired claim goes through the takeover"); return nil },
		OnTakeOver: func(p, id string) error { mu.Lock(); taken = append(taken, p+"/"+id); mu.Unlock(); return nil },
	})

	run(func() { board.claimTicket() })
	require.NotNil(t, sh.yes)
	assert.Equal(t, "Take over 1?", sh.confirmTitle)
	assert.Contains(t, sh.confirmDetail, "Alice A.", "the confirmation names the owner")
	mu.Lock()
	assert.Empty(t, taken)
	mu.Unlock()

	run(sh.yes)
	require.Eventually(t, func() bool { return strings.Contains(sh.lastToast(), "taken over") }, 3*time.Second, 20*time.Millisecond)
	mu.Lock()
	assert.Equal(t, []string{"p/1"}, taken)
	mu.Unlock()
}

// A claim that changed hands between the read and the key is told by the
// action itself.
func TestTeamBoardTellsAClaimThatWasTakenMeanwhile(t *testing.T) {
	tickets := []TeamTicket{{ID: "1", Title: "Pool", Status: "todo", Project: "p"}}
	board, sh, run := claimBoard(t, tickets, &BoardActions{
		OnClaim: func(string, string) error { return &ClaimHeldError{Owner: "Bob B."} },
	})
	run(func() { board.claimTicket() })
	require.Eventually(t, func() bool { return strings.Contains(sh.lastToast(), "Bob B.") }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "1 is already held by Bob B..", sh.lastToast())

	other := &BoardActions{OnClaim: func(string, string) error { return errors.New("offline") }}
	board2, sh2, run2 := claimBoard(t, tickets, other)
	run2(func() { board2.claimTicket() })
	require.Eventually(t, func() bool { return strings.Contains(sh2.lastToast(), "offline") }, 3*time.Second, 20*time.Millisecond)
}

// Releasing or handing over the claim of another member names them.
func TestTeamBoardReleaseAndTransferNameTheOwner(t *testing.T) {
	tickets := []TeamTicket{
		{ID: "1", Title: "Alice's", Status: "todo", Project: "p", AssigneeID: "alice", Assignee: "Alice A."},
	}
	board, sh, run := claimBoard(t, tickets, &BoardActions{
		OnRelease:  func(string, string) error { return nil },
		OnTransfer: func(string, string, string) error { return nil },
		Members:    func() []SelectOption { return []SelectOption{{Label: "Bob", Value: "bob"}} },
	})

	run(func() { board.releaseTicket() })
	assert.Equal(t, "Release the claim of Alice A.?", sh.confirmTitle)
	assert.Contains(t, sh.confirmDetail, "Alice A.")
	assert.Contains(t, sh.confirmDetail, "not by you")

	sh.yes, sh.confirmTitle = nil, ""
	run(func() { board.transferTicket() })
	assert.Equal(t, "Hand over the claim of Alice A.?", sh.confirmTitle, "the list of members comes after the question")
	assert.Zero(t, sh.selects)
	run(sh.yes)
	assert.Equal(t, 1, sh.selects)

	// one's own claim: the common question, no list first
	own := []TeamTicket{{ID: "2", Title: "Mine", Status: "todo", Project: "p", AssigneeID: "me", Assignee: "Me"}}
	board2, sh2, run2 := claimBoard(t, own, &BoardActions{
		OnRelease: func(string, string) error { return nil },
	})
	run2(func() { board2.releaseTicket() })
	assert.Equal(t, "Release the ticket?", sh2.confirmTitle)
}

// The card shows the lease, the expiry once past, and the session that ended.
func TestTeamBoardCardsShowTheLeaseAndTheEndedSession(t *testing.T) {
	i18ntest.Set(t, "en")
	board := NewTeamBoardView(TeamBoardViewConfig{IsConfigured: func() bool { return true }})
	live := stripTviewTags(board.buildTeamCard(TeamTicket{ID: "1", Status: "in_progress", Assignee: "Alice",
		LeaseUntil: time.Now().Add(2 * time.Hour)}).MetaText)
	assert.Contains(t, live, "@Alice")
	assert.Contains(t, live, "lease ")
	assert.NotContains(t, live, "expired")

	stale := stripTviewTags(board.buildTeamCard(TeamTicket{ID: "2", Status: "in_progress", Assignee: "Alice",
		LeaseUntil: time.Now().Add(-time.Hour), Expired: true}).MetaText)
	assert.Contains(t, stale, "expired")
	assert.NotContains(t, stale, "lease ")

	ended := stripTviewTags(board.buildTeamCard(TeamTicket{ID: "3", Status: "in_progress", Assignee: "Alice",
		Labels: []string{"session-ended"}, SessionEnded: true}).MetaText)
	assert.Contains(t, ended, "session ended")
	assert.Equal(t, 1, strings.Count(ended, "session"), "the label is the badge, not shown twice")

	pool := stripTviewTags(board.buildTeamCard(TeamTicket{ID: "4", Status: "todo"}).MetaText)
	assert.NotContains(t, pool, "lease")

	detail := board.formatTicketDetail(&TeamTicket{ID: "2", Title: "Stale", Assignee: "Alice", LeaseUntil: time.Now().Add(-time.Hour), Expired: true})
	assert.Contains(t, detail, "expired on")
}

// The palette hands over or releases all the claims of a member: whose, to
// whom, then a confirmation with the number and the names.
func TestTeamBoardReassignsTheClaimsOfAMember(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	tickets := []TeamTicket{
		{ID: "1", Title: "One", Status: "in_progress", Project: "p", AssigneeID: "alice", Assignee: "Alice A."},
		{ID: "2", Title: "Two", Status: "review", Project: "p", AssigneeID: "alice", Assignee: "Alice A."},
		{ID: "3", Title: "Done", Status: "done", Project: "p", AssigneeID: "alice", Assignee: "Alice A."},
		{ID: "4", Title: "Four", Status: "in_progress", Project: "p", AssigneeID: "ghost", Assignee: "ghost"},
	}
	board, sh, run := claimBoard(t, tickets, &BoardActions{
		Members: func() []SelectOption {
			return []SelectOption{{Label: "Alice A.", Value: "alice"}, {Label: "Bob B.", Value: "bob"}}
		},
		OnReassign: func(from, to string) (int, error) {
			mu.Lock()
			calls = append(calls, from+">"+to)
			mu.Unlock()
			return 2, nil
		},
	})
	var found bool
	for _, c := range board.ContextCommands() {
		found = found || c.ID == "team.board.reassign"
	}
	require.True(t, found, "the command is in the palette")

	run(func() { board.reassignClaims() })
	assert.Equal(t, "Whose claims?", sh.title)
	assert.Equal(t, []string{"Alice A. (2)", "ghost (1)"}, optionLabels(sh.opts), "who holds open claims, finished ones not counted, a member without a card too")

	run(func() { sh.pick("alice") })
	assert.Equal(t, "Claims of Alice A. — to whom?", sh.title)
	assert.Equal(t, []string{"Release them", "Bob B."}, optionLabels(sh.opts), "not to the member themselves")
	run(func() { sh.pick("bob") })
	assert.Equal(t, "Hand 2 claim(s) of Alice A. to Bob B.?", sh.confirmTitle)
	mu.Lock()
	assert.Empty(t, calls)
	mu.Unlock()
	run(sh.yes)
	require.Eventually(t, func() bool { return strings.Contains(sh.lastToast(), "handed to Bob B.") }, 3*time.Second, 20*time.Millisecond)
	mu.Lock()
	assert.Equal(t, []string{"alice>bob"}, calls)
	mu.Unlock()

	run(func() { board.reassignClaims() })
	run(func() { sh.pick("ghost") })
	run(func() { sh.pick(releaseTarget) })
	assert.Equal(t, "Release 1 claim(s) of ghost?", sh.confirmTitle)
	run(sh.yes)
	require.Eventually(t, func() bool { return strings.Contains(sh.lastToast(), "released") }, 3*time.Second, 20*time.Millisecond)
	mu.Lock()
	assert.Equal(t, []string{"alice>bob", "ghost>"}, calls)
	mu.Unlock()
}

func optionLabels(opts []SelectOption) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.Label
	}
	return out
}
