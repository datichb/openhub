package views

import (
	"sync"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// menuShell records the lists and inputs the board opens; it picks nothing by
// itself (closing a list with Esc is the same: no callback).
type menuShell struct {
	recordingShell
	mu         sync.Mutex
	title      string
	opts       []SelectOption
	pick       func(string)
	selects    int
	inputTitle string
	input      func(string)
}

func (s *menuShell) ShowSelectModal(title string, opts []SelectOption, _ string, pick func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.title, s.opts, s.pick = title, opts, pick
	s.selects++
}

func (s *menuShell) ShowInputModal(title, _ string, fn func(string)) {
	s.inputTitle, s.input = title, fn
}

func (s *menuShell) values() []string {
	var out []string
	for _, o := range s.opts {
		out = append(out, o.Value)
	}
	return out
}

func (s *menuShell) toastList() []string {
	s.recordingShell.mu.Lock()
	defer s.recordingShell.mu.Unlock()
	return append([]string(nil), s.toasts...)
}

// cardCatalogue is the catalogue the menu of a card looks workflows up in.
func cardCatalogue(missing ...string) func(string) (TicketWorkflow, bool) {
	return func(id string) (TicketWorkflow, bool) {
		for _, m := range missing {
			if m == id {
				return TicketWorkflow{}, false
			}
		}
		return TicketWorkflow{ID: id, Label: "Label " + id, Risk: "write", Runtimes: []string{"local"}}, true
	}
}

// cardCalls records what the menu launches.
type cardCalls struct {
	launched  []string
	cards     []CardLaunch
	cardOf    []TicketContext
	instructs []string
}

func cardQuickActions(src CardSource, ref string, canImport bool, calls *cardCalls) *BoardQuickActions {
	return &BoardQuickActions{
		Workflow:  cardCatalogue(),
		Source:    func(TicketContext) (CardSource, string) { return src, ref },
		CanImport: func(TicketContext) bool { return canImport },
		Launch: func(id string, tc TicketContext) {
			calls.launched = append(calls.launched, id+" "+tc.ID)
		},
		LaunchCard: func(l CardLaunch, tc TicketContext) {
			calls.cards = append(calls.cards, l)
			calls.cardOf = append(calls.cardOf, tc)
		},
	}
}

func cardBoard(t *testing.T, tickets []TeamTicket, layout BoardLayout, qa *BoardQuickActions) (*TeamBoardView, *menuShell) {
	t.Helper()
	i18ntest.Set(t, "en")
	board := NewTeamBoardView(TeamBoardViewConfig{
		Tickets:      tickets,
		IsConfigured: func() bool { return true },
		Layout:       func() BoardLayout { return layout },
		QuickActions: qa,
	})
	sh := &menuShell{}
	board.SetShell(sh)
	board.Mount(tview.NewFlex().SetDirection(tview.FlexRow), nil)
	t.Cleanup(board.Unmount)
	return board, sh
}

// focusOn puts the focus on the column of a status.
func focusOn(t *testing.T, board *TeamBoardView, status string) {
	t.Helper()
	for i, c := range board.allColumns {
		if c.Status == status {
			board.focusCol = i
			return
		}
	}
	t.Fatalf("no column %q", status)
}

func openMenu(t *testing.T, board *TeamBoardView, sh *menuShell, status string) {
	t.Helper()
	focusOn(t, board, status)
	sh.opts, sh.pick = nil, nil
	board.HandleKey(runeKey('a'))
	require.NotEmpty(t, sh.opts, "the menu of the %s column opened", status)
}

// The todo column offers to frame the ticket and to do it. On the card of a
// GitLab issue where Beads is available, doing it imports the issue and
// starts the work (the claim goes in progress); framing names the issue as
// the request.
func TestCardMenuOfAGitLabIssueInTodo(t *testing.T) {
	calls := &cardCalls{}
	board, sh := cardBoard(t, []TeamTicket{{ID: "623", Title: "Export CSV", Status: "todo", Project: "p"}},
		BoardLayout{}, cardQuickActions(CardGitLab, "#623", true, calls))
	openMenu(t, board, sh, "todo")

	assert.Equal(t, []string{"spec", "ticket"}, sh.values(), "frame it, do it: nothing else on a todo card")
	assert.Contains(t, sh.title, "623")
	assert.Contains(t, sh.title, "Export CSV")
	assert.Contains(t, sh.opts[0].Label, "Label spec")
	assert.Contains(t, sh.opts[1].Label, "#623 will be imported as a Beads ticket", "the line says what doing the ticket does")

	sh.pick("spec")
	sh.pick("ticket")
	require.Len(t, calls.cards, 2)
	assert.Equal(t, CardLaunch{Workflow: "spec", Request: "#623"}, calls.cards[0], "framing does not start the work")
	assert.Equal(t, CardLaunch{Workflow: "ticket", Import: true, Start: true}, calls.cards[1])
	assert.Equal(t, "p", calls.cardOf[1].Project)
	assert.Equal(t, teamstate.ColumnRoleInitial, calls.cardOf[1].Role)
	assert.Empty(t, calls.launched, "the workflow that takes a Beads id is never launched on #623")
}

// Without Beads in the project, the issue cannot become a Beads ticket: quick
// takes it, and the line says why ticket is not offered.
func TestCardMenuOfAGitLabIssueWithoutBeads(t *testing.T) {
	calls := &cardCalls{}
	board, sh := cardBoard(t, []TeamTicket{{ID: "623", Title: "Export CSV", Status: "todo", Project: "p"}},
		BoardLayout{}, cardQuickActions(CardGitLab, "#623", false, calls))
	openMenu(t, board, sh, "todo")

	assert.Equal(t, []string{"spec", "quick"}, sh.values())
	assert.Contains(t, sh.opts[1].Label, "“ticket” needs a Beads ticket and this project has no Beads")
	sh.pick("quick")
	assert.Equal(t, []CardLaunch{{Workflow: "quick", Request: "#623", Start: true}}, calls.cards)
}

// A ticket of another tracker (a Jira key) is no Beads id either.
func TestCardMenuOfAnotherTracker(t *testing.T) {
	calls := &cardCalls{}
	board, sh := cardBoard(t, []TeamTicket{{ID: "SRU-12", Title: "Jira", Status: "todo", Project: "p"}},
		BoardLayout{}, cardQuickActions(CardOther, "", true, calls))
	openMenu(t, board, sh, "todo")
	assert.Equal(t, []string{"spec", "quick"}, sh.values())
	assert.Contains(t, sh.opts[1].Label, "SRU-12 is not one")
	sh.pick("quick")
	assert.Equal(t, []CardLaunch{{Workflow: "quick", Request: "SRU-12", Start: true}}, calls.cards)
}

// A Beads ticket is done with ticket on its own id (the session claims it),
// and framed with its id as the request.
func TestCardMenuOfABeadsTicket(t *testing.T) {
	calls := &cardCalls{}
	board, sh := cardBoard(t, []TeamTicket{{ID: "t-sru-ab1", Title: "Beads", Status: "todo", Project: "p"}},
		BoardLayout{}, cardQuickActions(CardBeads, "", false, calls))
	openMenu(t, board, sh, "todo")
	assert.Equal(t, []string{"spec", "ticket"}, sh.values())
	sh.pick("ticket")
	sh.pick("spec")
	assert.Equal(t, []string{"ticket t-sru-ab1"}, calls.launched)
	assert.Equal(t, []CardLaunch{{Workflow: "spec", Request: "t-sru-ab1"}}, calls.cards)
}

// A workflow the project does not offer is a dimmed line that says so.
func TestCardMenuSaysWhatTheProjectDoesNotOffer(t *testing.T) {
	calls := &cardCalls{}
	qa := cardQuickActions(CardBeads, "", false, calls)
	qa.Workflow = cardCatalogue("spec")
	board, sh := cardBoard(t, []TeamTicket{{ID: "t-1", Title: "x", Status: "todo", Project: "p"}}, BoardLayout{}, qa)
	openMenu(t, board, sh, "todo")
	assert.Equal(t, []string{"spec", "ticket"}, sh.values())
	assert.Contains(t, sh.opts[0].Label, "“spec” is not offered in this project")

	sh.pick("spec")
	assert.Equal(t, []string{"“spec” is not offered in this project"}, sh.toastList(), "choosing it explains it")
	assert.Empty(t, calls.cards)
}

// In the work column: do the ticket, and instruct the session that runs on
// the card when there is one.
func TestCardMenuInProgress(t *testing.T) {
	calls := &cardCalls{}
	qa := cardQuickActions(CardBeads, "", false, calls)
	qa.ActiveSession = func(tc TicketContext) string {
		if tc.SessionID == "ses-1" {
			return "ses-1"
		}
		return ""
	}
	qa.Instruct = func(_ TicketContext, session, text string) {
		calls.instructs = append(calls.instructs, session+": "+text)
	}
	board, sh := cardBoard(t, []TeamTicket{
		{ID: "t-1", Title: "Running", Status: "in_progress", Project: "p", SessionID: "ses-1"},
		{ID: "t-2", Title: "Idle", Status: "in_progress", Project: "p", SessionID: "ses-gone"},
	}, BoardLayout{}, qa)

	openMenu(t, board, sh, "in_progress")
	assert.Equal(t, []string{"ticket", "instruct"}, sh.values())
	assert.Contains(t, sh.opts[1].Label, "Give an instruction to the running session")
	sh.pick("instruct")
	assert.Equal(t, "Instruction for t-1", sh.inputTitle)
	sh.input("  take the cache into account  ")
	sh.input("   ")
	assert.Equal(t, []string{"ses-1: take the cache into account"}, calls.instructs, "trimmed, and nothing for a blank")

	// The second card: its session is over.
	board.columnCards[board.focusCol].SetCurrentItem(1)
	sh.opts, sh.pick = nil, nil
	board.HandleKey(runeKey('a'))
	assert.Equal(t, []string{"ticket", "instruct"}, sh.values())
	assert.Contains(t, sh.opts[1].Label, "no session is running on this ticket")
	sh.pick("instruct")
	assert.Contains(t, sh.toastList()[0], "no session is running")
}

// A GitLab issue in progress is done again the way it is done from todo: the
// work is already started, the claim stays where it is.
func TestCardMenuInProgressOnAGitLabIssue(t *testing.T) {
	calls := &cardCalls{}
	board, sh := cardBoard(t, []TeamTicket{{ID: "623", Title: "x", Status: "in_progress", Project: "p"}},
		BoardLayout{}, cardQuickActions(CardGitLab, "#623", true, calls))
	openMenu(t, board, sh, "in_progress")
	assert.Equal(t, []string{"ticket", "instruct"}, sh.values())
	sh.pick("ticket")
	assert.Equal(t, []CardLaunch{{Workflow: "ticket", Import: true, Start: true}}, calls.cards)
}

// Review: the review of the branch of the claim, and the feedback of its
// merge request when the claim knows the URL.
func TestCardMenuInReview(t *testing.T) {
	calls := &cardCalls{}
	board, sh := cardBoard(t, []TeamTicket{
		{ID: "t-1", Title: "With MR", Status: "review", Project: "p", Branch: "feat/csv", MRURL: "https://gitlab.example.com/g/p/-/merge_requests/9"},
		{ID: "t-2", Title: "Nothing known", Status: "review", Project: "p"},
	}, BoardLayout{}, cardQuickActions(CardBeads, "", false, calls))

	openMenu(t, board, sh, "review")
	assert.Equal(t, []string{"review", "review-feedback"}, sh.values())
	assert.Contains(t, sh.opts[0].Label, "branch feat/csv")
	assert.Contains(t, sh.opts[1].Label, "merge_requests/9")
	sh.pick("review")
	sh.pick("review-feedback")
	assert.Equal(t, []CardLaunch{
		{Workflow: "review", Prefill: map[string]string{"branch": "feat/csv"}},
		{Workflow: "review-feedback", Prefill: map[string]string{"mr": "https://gitlab.example.com/g/p/-/merge_requests/9"}},
	}, calls.cards)

	board.columnCards[board.focusCol].SetCurrentItem(1)
	sh.opts, sh.pick = nil, nil
	board.HandleKey(runeKey('a'))
	assert.Equal(t, []string{"review", "review-feedback"}, sh.values())
	assert.Contains(t, sh.opts[0].Label, "branch unknown: recent changes")
	assert.Contains(t, sh.opts[1].Label, "the claim has no merge request URL")
	calls.cards = nil
	sh.pick("review")
	assert.Equal(t, []CardLaunch{{Workflow: "review"}}, calls.cards, "no branch to prefill: the form asks")
	sh.pick("review-feedback")
	assert.Len(t, calls.cards, 1, "without a URL the feedback is not launched")
	assert.Contains(t, sh.toastList()[0], "no merge request URL")
}

// The other columns have the actions of every card, and no workflow.
func TestCardMenuOfTheOtherColumns(t *testing.T) {
	calls := &cardCalls{}
	tickets := []TeamTicket{
		{ID: "1", Title: "a", Status: "validation", Project: "p"},
		{ID: "2", Title: "b", Status: "blocked", Project: "p"},
		{ID: "3", Title: "c", Status: "done", Project: "p"},
	}
	board, sh := cardBoard(t, tickets, BoardLayout{}, cardQuickActions(CardBeads, "", false, calls))
	for _, status := range []string{"validation", "blocked", "done"} {
		openMenu(t, board, sh, status)
		assert.Equal(t, []string{"detail", "status", "transfer"}, sh.values(), status)
	}
	assert.Empty(t, calls.cards)
	assert.Empty(t, calls.launched)

	// The status entry is the action of the status key.
	board.actions = &BoardActions{OnStatus: func(_, _, _ string) error { return nil }}
	before := sh.selects
	sh.pick("status")
	assert.Equal(t, before+1, sh.selects, "« status » opens the list of the columns")
}

// On a board with columns of its own: the work column is the first active one,
// the other active columns have the actions of every card, the review role
// puts the review actions on its column wherever it is.
func TestCardMenuOnACustomBoard(t *testing.T) {
	calls := &cardCalls{}
	layout := BoardLayout{Board: teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "Backlog", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "Dev", Role: teamstate.ColumnRoleActive},
		{ID: "qa", Name: "QA", Role: teamstate.ColumnRoleActive},
		{ID: "mr", Name: "MR open", Role: teamstate.ColumnRoleReview},
		{ID: "shipped", Name: "Shipped", Role: teamstate.ColumnRoleTerminal},
	}}}
	tickets := []TeamTicket{
		{ID: "1", Title: "a", Status: "backlog", Project: "p"},
		{ID: "2", Title: "b", Status: "dev", Project: "p"},
		{ID: "3", Title: "c", Status: "qa", Project: "p"},
		{ID: "4", Title: "d", Status: "mr", Project: "p", Branch: "feat/d"},
		{ID: "5", Title: "e", Status: "shipped", Project: "p"},
	}
	board, sh := cardBoard(t, tickets, layout, cardQuickActions(CardBeads, "", false, calls))
	want := map[string][]string{
		"backlog": {"spec", "ticket"},
		"dev":     {"ticket", "instruct"},
		"qa":      {"detail", "status", "transfer"},
		"mr":      {"review", "review-feedback"},
		"shipped": {"detail", "status", "transfer"},
	}
	for status, values := range want {
		openMenu(t, board, sh, status)
		assert.Equal(t, values, sh.values(), status)
	}
}

// A team whose config predates the role: the column of the review status is
// the review column.
func TestCardMenuReviewStatusStandsInForTheRole(t *testing.T) {
	calls := &cardCalls{}
	layout := BoardLayout{Board: teamstate.DefaultBoardConfig()}
	board, sh := cardBoard(t, []TeamTicket{{ID: "1", Title: "a", Status: "review", Project: "p"}}, layout, cardQuickActions(CardBeads, "", false, calls))
	openMenu(t, board, sh, "review")
	assert.Equal(t, []string{"review", "review-feedback"}, sh.values())
}

// The omnibar offers what the menu offers: no « run ticket » on a GitLab
// issue, no workflow in a column that has none.
func TestCardCommandsFollowTheMenu(t *testing.T) {
	calls := &cardCalls{}
	board, _ := cardBoard(t, []TeamTicket{
		{ID: "623", Title: "x", Status: "todo", Project: "p"},
		{ID: "1", Title: "y", Status: "done", Project: "p"},
	}, BoardLayout{}, cardQuickActions(CardGitLab, "#623", false, calls))

	focusOn(t, board, "todo")
	var labels []string
	for _, c := range board.ContextCommands() {
		labels = append(labels, c.Label)
	}
	assert.Equal(t, []string{"run spec ⟨#623⟩", "run quick ⟨#623⟩"}, labels)
	board.ContextCommands()[1].Action()
	assert.Equal(t, []CardLaunch{{Workflow: "quick", Request: "#623", Start: true}}, calls.cards)

	focusOn(t, board, "done")
	assert.Empty(t, board.ContextCommands())
}

// Columns without roles (a board built by hand) keep the menu of the Beads
// board.
func TestCardMenuWithoutRolesKeepsTheOldMenu(t *testing.T) {
	calls := &cardCalls{}
	qa := cardQuickActions(CardBeads, "", false, calls)
	qa.Workflows = func() []TicketWorkflow { return []TicketWorkflow{{ID: "ticket", Label: "Do"}} }
	board, sh := cardBoard(t, []TeamTicket{{ID: "t-1", Title: "x", Status: "todo", Project: "p"}}, BoardLayout{}, qa)
	for i := range board.allColumns {
		board.allColumns[i].Role = ""
	}
	focusOn(t, board, "todo")
	board.HandleKey(runeKey('a'))
	assert.Equal(t, []string{"ticket"}, sh.values())
}

// Esc closes the list of the status (or of the transfer) without calling
// back: the keys of the board still answer. The flag that guards the actions
// was taken when the list opened, and nothing released it.
func TestEscapeOnAListLeavesTheActionKeysFree(t *testing.T) {
	i18ntest.Set(t, "en")
	var mu sync.Mutex
	var claimed []string
	tickets := []TeamTicket{{ID: "1", Title: "One", Status: "todo", Project: "p2", AssigneeID: "alice", Assignee: "Alice"}}
	board := NewTeamBoardView(TeamBoardViewConfig{
		Tickets: tickets, IsConfigured: func() bool { return true },
		Actions: &BoardActions{
			Members: func() []SelectOption { return []SelectOption{{Label: "Bob", Value: "bob"}} },
			OnClaim: func(project, id string) error {
				mu.Lock()
				claimed = append(claimed, project+"/"+id)
				mu.Unlock()
				return nil
			},
			OnStatus:   func(string, string, string) error { return nil },
			OnTransfer: func(string, string, string) error { return nil },
		},
	})
	sh := &menuShell{}
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() {
		board.Mount(content, app)
		board.SetShell(sh)
		board.populateColumns(tickets, board.allColumns)
	})

	for _, key := range []rune{'s', 't'} {
		before := 0
		onLoop(app, func() {
			before = sh.selects
			board.HandleKey(runeKey(key))
			assert.Equal(t, before+1, sh.selects, "%c opens its list", key)
			assert.False(t, board.actionInProgress.Load(), "an open list takes nothing: Esc has no callback to release it")
			// Esc: nothing is called. The key answers again.
			board.HandleKey(runeKey(key))
			assert.Equal(t, before+2, sh.selects, "%c answers after Esc", key)
		})
	}

	onLoop(app, func() { board.HandleKey(runeKey('c')) })
	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(claimed) == 1 }, 3*time.Second, 10*time.Millisecond)
	mu.Lock()
	assert.Equal(t, []string{"p2/1"}, claimed, "the take works after Esc, on the project of the card")
	mu.Unlock()
	waitFor(t, app, func() bool { return !board.actionInProgress.Load() })

	// Choosing in the list does take the flag, and frees it with the answer.
	statuses := make(chan string, 1)
	board.actions.OnStatus = func(project, id, status string) error {
		statuses <- project + "/" + id + "→" + status
		return nil
	}
	onLoop(app, func() {
		board.HandleKey(runeKey('s'))
		sh.pick("review")
	})
	select {
	case got := <-statuses:
		assert.Equal(t, "p2/1→review", got, "the status change works on the project of the card")
	case <-time.After(3 * time.Second):
		t.Fatal("the status change was not made")
	}
	waitFor(t, app, func() bool { return !board.actionInProgress.Load() })
	onLoop(app, board.Unmount)
}

// Two projects may have a ticket of the same ID: the action works on the card
// that is selected, not on the first of that ID.
func TestSelectedCardIsTheOneOfItsProject(t *testing.T) {
	i18ntest.Set(t, "en")
	tickets := []TeamTicket{
		{ID: "7", Title: "In p1", Status: "todo", Project: "p1"},
		{ID: "7", Title: "In p2", Status: "todo", Project: "p2"},
	}
	board := NewTeamBoardView(TeamBoardViewConfig{Tickets: tickets, IsConfigured: func() bool { return true }})
	board.Mount(tview.NewFlex(), nil)
	t.Cleanup(board.Unmount)
	focusOn(t, board, "todo")
	board.columnCards[board.focusCol].SetCurrentItem(1)
	card, ok := board.selectedCard()
	require.True(t, ok)
	assert.Equal(t, cardRef{project: "p2", ticket: "7"}, card)
}
