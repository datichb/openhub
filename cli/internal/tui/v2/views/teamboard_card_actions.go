package views

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Actions of a card of the team board (a): the menu follows the column of the
// card and the source of its ticket.
//
//	initial (todo)       frame it (spec), do the ticket
//	active, work column  do the ticket, instruct the session running on it
//	review               review it, apply the review feedback of its MR
//	any other column     detail, status, transfer
//
// What the card cannot have is listed too, dimmed, with the reason: choosing
// it says why. « Do the ticket » (ticket) needs a Beads ticket: on the card of
// a GitLab issue it imports the issue into Beads and launches on the ticket;
// without Beads in the project it is the workflow quick that takes the issue.
// ─────────────────────────────────────────────────────────────────────────────

// The workflows the card menu names; the project decides what else it offers.
const (
	workflowSpec           = "spec"
	workflowTicket         = "ticket"
	workflowQuick          = "quick"
	workflowReview         = "review"
	workflowReviewFeedback = "review-feedback"
)

// CardSource is where the ticket of a card lives.
type CardSource int

const (
	// CardBeads: a Beads ticket (the id names it in the Beads of the project).
	CardBeads CardSource = iota
	// CardGitLab: an issue of the GitLab tracker of the project.
	CardGitLab
	// CardOther: a ticket of another tracker (a Jira key…), which no workflow
	// that takes a Beads ticket can read.
	CardOther
)

// CardLaunch is the launch of a workflow on a card.
type CardLaunch struct {
	Workflow string
	// Request is the free request the form opens with (« #693 »).
	Request string
	// Prefill fills other inputs of the form (review: the branch).
	Prefill map[string]string
	// Import launches on the Beads ticket that mirrors the issue of the card,
	// created when the project has none.
	Import bool
	// Start: the launch starts the work on the card, so its claim goes in
	// progress (within the limit of work in progress).
	Start bool
}

// cardEntry is a line of the menu of a card.
type cardEntry struct {
	ID string
	// Workflow is the workflow the entry launches, "" for the other actions.
	Workflow string
	TW       TicketWorkflow
	// Label is the text of an entry that launches no workflow.
	Label string
	// Note is said after the line (why this workflow, which branch).
	Note string
	// Why is the reason an entry is not offered: the line is dimmed and
	// choosing it says it. Run is nil then.
	Why string
	Run func()
	// Request is the request an omnibar command of the entry names.
	Request string
}

// cardSourceOf tells where the ticket of a card lives.
func cardSourceOf(ticket TicketContext, qa *BoardQuickActions) (CardSource, string) {
	if qa == nil {
		return CardBeads, ""
	}
	if qa.Source != nil {
		return qa.Source(ticket)
	}
	if qa.IssueRef != nil {
		if ref := qa.IssueRef(ticket); ref != "" {
			return CardGitLab, ref
		}
	}
	return CardBeads, ""
}

// workflowEntry is a line that launches a workflow; a workflow the project
// does not have gives a dimmed line with the reason.
func workflowEntry(qa *BoardQuickActions, id string, run func(), request string) cardEntry {
	if qa != nil && qa.Workflow != nil {
		if w, ok := qa.Workflow(id); ok {
			return cardEntry{ID: id, Workflow: id, TW: w, Run: run, Request: request}
		}
	}
	return cardEntry{ID: id, Why: i18n.Tf("v53f.card.no_workflow", id)}
}

// cardEntries lists the menu of a card of the team board.
func (v *TeamBoardView) cardEntries(tc TicketContext) []cardEntry {
	qa := v.cfg.QuickActions
	src, ref := cardSourceOf(tc, qa)
	launch := func(l CardLaunch) func() {
		return func() {
			if qa != nil && qa.LaunchCard != nil {
				qa.LaunchCard(l, tc)
			}
		}
	}
	request := firstNonEmpty(ref, tc.ID)
	switch tc.Role {
	case teamstate.ColumnRoleInitial:
		return []cardEntry{
			workflowEntry(qa, workflowSpec, launch(CardLaunch{Workflow: workflowSpec, Request: request}), request),
			v.doTicketEntry(tc, src, ref, launch),
		}
	case teamstate.ColumnRoleActive:
		if tc.Work {
			return []cardEntry{v.doTicketEntry(tc, src, ref, launch), v.instructEntry(tc)}
		}
	case teamstate.ColumnRoleReview:
		return v.reviewEntries(tc, launch)
	}
	return v.basicEntries()
}

// doTicketEntry is « do the ticket ».
func (v *TeamBoardView) doTicketEntry(tc TicketContext, src CardSource, ref string, launch func(CardLaunch) func()) cardEntry {
	qa := v.cfg.QuickActions
	request := firstNonEmpty(ref, tc.ID)
	quick := func(note string) cardEntry {
		e := workflowEntry(qa, workflowQuick, launch(CardLaunch{Workflow: workflowQuick, Request: request, Start: true}), request)
		if e.Why == "" {
			e.Note = note
		}
		return e
	}
	switch src {
	case CardGitLab:
		if qa != nil && qa.CanImport != nil && qa.CanImport(tc) {
			e := workflowEntry(qa, workflowTicket, launch(CardLaunch{Workflow: workflowTicket, Import: true, Start: true}), ref)
			if e.Why == "" {
				e.Note = i18n.Tf("v53f.card.import_note", ref)
			}
			return e
		}
		return quick(i18n.T("v53f.card.no_beads_note"))
	case CardOther:
		return quick(i18n.Tf("v53f.card.not_beads_note", tc.ID))
	}
	return workflowEntry(qa, workflowTicket, func() {
		if qa != nil && qa.Launch != nil {
			qa.Launch(workflowTicket, tc)
		}
	}, tc.ID)
}

// instructEntry is the instruction to the session running on the card.
func (v *TeamBoardView) instructEntry(tc TicketContext) cardEntry {
	qa := v.cfg.QuickActions
	session := ""
	if qa != nil && qa.ActiveSession != nil && qa.Instruct != nil {
		session = qa.ActiveSession(tc)
	}
	if session == "" {
		return cardEntry{ID: "instruct", Why: i18n.T("v53f.card.no_session")}
	}
	return cardEntry{ID: "instruct", Label: i18n.T("v53f.card.instruct"), Run: func() {
		v.shell.ShowInputModal(i18n.Tf("v53f.card.instruct_title", tc.ID), "", func(text string) {
			if text = strings.TrimSpace(text); text != "" {
				qa.Instruct(tc, session, text)
			}
		})
	}}
}

// reviewEntries are the review of the branch of the claim and the review
// feedback of its merge request.
func (v *TeamBoardView) reviewEntries(tc TicketContext, launch func(CardLaunch) func()) []cardEntry {
	qa := v.cfg.QuickActions
	review := workflowEntry(qa, workflowReview, nil, "")
	if review.Why == "" {
		l := CardLaunch{Workflow: workflowReview}
		review.Note = i18n.T("v53f.card.branch_unknown_note")
		if tc.Branch != "" {
			l.Prefill = map[string]string{"branch": tc.Branch}
			review.Note = i18n.Tf("v53f.card.branch_note", tc.Branch)
		}
		review.Run = launch(l)
	}
	feedback := workflowEntry(qa, workflowReviewFeedback, nil, "")
	if feedback.Why == "" {
		if tc.MRURL == "" {
			feedback = cardEntry{ID: workflowReviewFeedback, Why: i18n.T("v53f.card.no_mr")}
		} else {
			feedback.Note = tc.MRURL
			feedback.Run = launch(CardLaunch{Workflow: workflowReviewFeedback, Prefill: map[string]string{"mr": tc.MRURL}})
		}
	}
	return []cardEntry{review, feedback}
}

// basicEntries are the actions of every card: detail, status, transfer.
func (v *TeamBoardView) basicEntries() []cardEntry {
	return []cardEntry{
		{ID: "detail", Label: i18n.T("v53f.card.detail"), Run: v.showTicketDetail},
		{ID: "status", Label: i18n.T("v53f.card.status"), Run: v.changeStatus},
		{ID: "transfer", Label: i18n.T("v53f.card.transfer"), Run: v.transferTicket},
	}
}

// showCardMenu opens the menu of a card of the team board.
func (v *TeamBoardView) showCardMenu(tc TicketContext) {
	entries := v.cardEntries(tc)
	title := i18n.Tf("tui.launch.board_title", tc.ID)
	if tc.Title != "" {
		title += " · " + tc.Title
	}
	var wfs []TicketWorkflow
	for _, e := range entries {
		if e.Workflow != "" {
			wfs = append(wfs, e.TW)
		}
	}
	rows := newWorkflowRows(wfs)
	opts := make([]SelectOption, 0, len(entries))
	byID := map[string]cardEntry{}
	n := 0
	for _, e := range entries {
		var label string
		switch {
		case e.Why != "":
			label = theme.Paint(theme.TextMutedHex, tview.Escape("– "+e.Why))
		case e.Workflow != "":
			label = rows.label(n)
			n++
		default:
			label = "  " + e.Label
		}
		if e.Note != "" && e.Why == "" {
			label += "  " + theme.Paint(theme.TextMutedHex, tview.Escape(e.Note))
		}
		opts = append(opts, SelectOption{Label: label, Value: e.ID})
		byID[e.ID] = e
	}
	v.shell.ShowSelectModal(title, opts, "", func(id string) {
		e, ok := byID[id]
		switch {
		case !ok:
		case e.Why != "":
			v.shell.ShowToastMsg(e.Why, false)
		case e.Run != nil:
			e.Run()
		}
	})
}

// cardCommands are the omnibar commands of a card of the team board: one per
// workflow its menu launches (« run ticket ⟨#693⟩ »).
func (v *TeamBoardView) cardCommands(prefix string, tc TicketContext) []ContextCommand {
	var out []ContextCommand
	for _, e := range v.cardEntries(tc) {
		if e.Workflow == "" || e.Run == nil {
			continue
		}
		out = append(out, ContextCommand{
			ID:          prefix + e.ID + "." + tc.ID,
			Label:       fmt.Sprintf("run %s ⟨%s⟩", e.Workflow, firstNonEmpty(e.Request, tc.ID)),
			Aliases:     []string{e.Workflow, "run " + e.Workflow},
			Description: i18n.T("tui.launch.board_cmd_desc"),
			Category:    i18n.T("tui.board.category_actions"),
			Action:      e.Run,
		})
	}
	return out
}
