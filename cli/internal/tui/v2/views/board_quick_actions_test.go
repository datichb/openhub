package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pickingShell records the options of the quick action modal and picks one.
type pickingShell struct {
	recordingShell
	title string
	pick  func(string)
}

func (s *pickingShell) ShowSelectModal(title string, opts []SelectOption, _ string, pick func(string)) {
	s.title, s.options, s.pick = title, opts, pick
}

func issueQuickActions(ref string, launched *string) *BoardQuickActions {
	return &BoardQuickActions{
		Workflows: func() []TicketWorkflow {
			return []TicketWorkflow{{ID: "ticket", Label: "Implémenter", Risk: "write"}, {ID: "mixed", Risk: "read"}}
		},
		RequestWorkflows: func() []TicketWorkflow {
			return []TicketWorkflow{{ID: "spec", Label: "Spécification", Risk: "plan", Pinned: true}, {ID: "quick", Risk: "write"}, {ID: "mixed", Risk: "read"}}
		},
		IssueRef: func(TicketContext) string { return ref },
		Launch:   func(id string, tc TicketContext) { *launched = "ticket " + id + " " + tc.ID },
		LaunchRequest: func(id string, tc TicketContext, request string) {
			*launched = "request " + id + " " + tc.ID + " " + request
		},
	}
}

// A1: on the card of a GitLab issue, the modal lists the workflows taking a
// request after the ones taking a ticket, each launching with « #<iid> » as
// the request.
func TestQuickActionsOfAGitLabIssue(t *testing.T) {
	t.Parallel()
	var launched string
	sh := &pickingShell{}
	showQuickActionModal(sh, TicketContext{ID: "693", Title: "Export CSV"}, issueQuickActions("#693", &launched))

	var values []string
	for _, o := range sh.options {
		values = append(values, o.Value)
	}
	assert.Equal(t, []string{"ticket", "mixed", "request:spec", "request:quick"}, values,
		"the ticket workflows as before, then those of a request; a workflow in both is listed once")
	assert.Contains(t, sh.options[2].Label, "spec")
	assert.Contains(t, sh.options[2].Label, "#693", "the label says what the request is")
	assert.NotContains(t, sh.options[0].Label, "#693")

	sh.pick("request:spec")
	assert.Equal(t, "request spec 693 #693", launched)
	sh.pick("ticket")
	assert.Equal(t, "ticket ticket 693", launched)

	cmds := ticketContextCommands("team.board.run.", TicketContext{ID: "693"}, issueQuickActions("#693", &launched))
	require.Len(t, cmds, 4)
	assert.Equal(t, "run ticket ⟨693⟩", cmds[0].Label)
	assert.Equal(t, "run spec ⟨#693⟩", cmds[2].Label)
	cmds[2].Action()
	assert.Equal(t, "request spec 693 #693", launched)
}

// A card that is not a GitLab issue (a Beads ticket, a Jira key) gets only the
// workflows taking a ticket, as before.
func TestQuickActionsOfAnotherTicket(t *testing.T) {
	t.Parallel()
	var launched string
	sh := &pickingShell{}
	qa := issueQuickActions("", &launched)
	showQuickActionModal(sh, TicketContext{ID: "bd-42"}, qa)
	require.Len(t, sh.options, 2)
	assert.Equal(t, "ticket", sh.options[0].Value)
	assert.Len(t, ticketContextCommands("board.run.", TicketContext{ID: "bd-42"}, qa), 2)

	qa.IssueRef, qa.RequestWorkflows, qa.LaunchRequest = nil, nil, nil
	sh = &pickingShell{}
	showQuickActionModal(sh, TicketContext{ID: "693"}, qa)
	assert.Len(t, sh.options, 2, "a board without the request wiring behaves as before")
}

// Without any ticket workflow, an issue still has the request ones.
func TestQuickActionsOfAnIssueWithoutTicketWorkflow(t *testing.T) {
	t.Parallel()
	var launched string
	qa := issueQuickActions("#7", &launched)
	qa.Workflows = func() []TicketWorkflow { return nil }
	sh := &pickingShell{}
	showQuickActionModal(sh, TicketContext{ID: "7"}, qa)
	require.Len(t, sh.options, 3)
	assert.Empty(t, sh.toasts)

	qa.RequestWorkflows = func() []TicketWorkflow { return nil }
	sh = &pickingShell{}
	showQuickActionModal(sh, TicketContext{ID: "7"}, qa)
	assert.Empty(t, sh.options)
	assert.Len(t, sh.toasts, 1)
}
