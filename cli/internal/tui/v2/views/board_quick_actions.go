package views

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Board quick actions (P1-T25, 10-tui §6): the workflows that take a Beads
// ticket, launched on the selected ticket — the launch form opens on its
// options step with the ticket prefilled (location, worktree and opening are
// chosen there). On the card of a GitLab issue the workflows that take a free
// request (spec, quick…) are offered too, with the issue (#<iid>) as the
// request: the launch form opens on its inputs, the request prefilled.
// ─────────────────────────────────────────────────────────────────────────────

// TicketContext holds the selected ticket.
type TicketContext struct {
	ID          string
	Title       string
	Description string // best-effort, may be empty
	Project     string // project directory ID (team board multi-project resolution)
	ProjectPath string // resolved filesystem path
	ProjectID   string // hub project ID
}

// TicketWorkflow is a workflow that takes a Beads ticket.
type TicketWorkflow struct {
	ID       string
	Label    string
	Risk     string
	Runtimes []string
	Pinned   bool
}

// BoardQuickActions wires the `a` key and the contextual omnibar commands
// of the boards. If nil in the view config, they are disabled.
type BoardQuickActions struct {
	// Workflows returns the workflows taking a Beads ticket (cached, event
	// loop safe), pinned ones first.
	Workflows func() []TicketWorkflow
	// Launch opens the launch form of a workflow on a ticket.
	Launch func(workflowID string, ticket TicketContext)

	// RequestWorkflows returns the workflows taking a free request (cached,
	// event loop safe); offered on the card of a GitLab issue only. May be nil.
	RequestWorkflows func() []TicketWorkflow
	// IssueRef returns the reference of the ticket as a request (« #693 »)
	// when the card is a GitLab issue, "" otherwise. May be nil.
	IssueRef func(ticket TicketContext) string
	// LaunchRequest opens the launch form of a workflow that takes a free
	// request, with the request prefilled.
	LaunchRequest func(workflowID string, ticket TicketContext, request string)

	// ResolveProjectByDirID resolves a project path and ID from a team-state
	// directory ID (team board: tickets of several projects).
	ResolveProjectByDirID func(dirID string) (projectID, projectPath string, ok bool)
	// FetchDescription fetches the ticket description (best-effort). May be nil.
	FetchDescription func(projectPath, ticketID string) string
}

// requestPrefix tells, in the values of the quick action modal, a workflow
// launched with the issue as its request from one launched on the ticket.
const requestPrefix = "request:"

// requestWorkflowsOf returns the workflows taking the card as a free request
// and the request (#<iid>); none when the card is not a GitLab issue.
func requestWorkflowsOf(ticket TicketContext, qa *BoardQuickActions) (workflows []TicketWorkflow, ref string) {
	if qa == nil || qa.RequestWorkflows == nil || qa.IssueRef == nil || qa.LaunchRequest == nil {
		return nil, ""
	}
	ref = qa.IssueRef(ticket)
	if ref == "" {
		return nil, ""
	}
	taken := map[string]bool{}
	if qa.Workflows != nil {
		for _, w := range qa.Workflows() {
			taken[w.ID] = true
		}
	}
	var out []TicketWorkflow
	for _, w := range qa.RequestWorkflows() {
		if !taken[w.ID] {
			out = append(out, w)
		}
	}
	return out, ref
}

// showQuickActionModal lists the workflows applicable to a ticket: those
// taking a Beads ticket, then, on a GitLab issue, those taking a request.
func showQuickActionModal(shell ShellAccess, ticket TicketContext, qa *BoardQuickActions) {
	if shell == nil || qa == nil || qa.Workflows == nil {
		return
	}
	wfs := qa.Workflows()
	reqs, ref := requestWorkflowsOf(ticket, qa)
	if len(wfs) == 0 && len(reqs) == 0 {
		shell.ShowToastMsg(i18n.T("tui.launch.board_none"), false)
		return
	}
	title := i18n.Tf("tui.launch.board_title", ticket.ID)
	if ticket.Title != "" {
		t := ticket.Title
		if r := []rune(t); len(r) > 40 {
			t = string(r[:37]) + "…"
		}
		title += " · " + t
	}
	opts := make([]SelectOption, 0, len(wfs)+len(reqs))
	for _, w := range wfs {
		opts = append(opts, SelectOption{Label: ticketWorkflowLabel(w), Value: w.ID})
	}
	for _, w := range reqs {
		opts = append(opts, SelectOption{Label: ticketWorkflowLabel(w) + "  " + i18n.Tf("tui.launch.board_request", ref), Value: requestPrefix + w.ID})
	}
	shell.ShowSelectModal(title, opts, "", func(id string) {
		if wfID, ok := strings.CutPrefix(id, requestPrefix); ok {
			if qa.LaunchRequest != nil {
				qa.LaunchRequest(wfID, ticket, ref)
			}
			return
		}
		if qa.Launch != nil {
			qa.Launch(id, ticket)
		}
	})
}

// ticketWorkflowLabel renders « ★ ticket  Implémenter  write ⌂ ▣ ».
func ticketWorkflowLabel(w TicketWorkflow) string {
	star := "  "
	if w.Pinned {
		star = theme.IconPinned + " "
	}
	label := w.Label
	if r := []rune(label); len(r) > 34 {
		label = string(r[:33]) + "…"
	}
	return strings.TrimRight(fmt.Sprintf("%s%-16s %-34s %-7s %s", star, w.ID, label, w.Risk, runtimeIcons(w.Runtimes)), " ")
}

// ticketContextCommands are the omnibar commands of a selected ticket: one
// per workflow taking a ticket (« run ticket ⟨bd-42⟩ »), and, on a GitLab
// issue, one per workflow taking a request (« run spec ⟨#693⟩ »).
func ticketContextCommands(prefix string, ticket TicketContext, qa *BoardQuickActions) []ContextCommand {
	if qa == nil || qa.Workflows == nil || qa.Launch == nil {
		return nil
	}
	var out []ContextCommand
	for _, w := range qa.Workflows() {
		w := w
		out = append(out, ContextCommand{
			ID:          prefix + w.ID + "." + ticket.ID,
			Label:       fmt.Sprintf("run %s ⟨%s⟩", w.ID, ticket.ID),
			Aliases:     []string{w.ID, "run " + w.ID},
			Description: i18n.T("tui.launch.board_cmd_desc"),
			Category:    i18n.T("tui.board.category_actions"),
			Action:      func() { qa.Launch(w.ID, ticket) },
		})
	}
	reqs, ref := requestWorkflowsOf(ticket, qa)
	for _, w := range reqs {
		w := w
		out = append(out, ContextCommand{
			ID:          prefix + w.ID + "." + ticket.ID,
			Label:       fmt.Sprintf("run %s ⟨%s⟩", w.ID, ref),
			Aliases:     []string{w.ID, "run " + w.ID},
			Description: i18n.Tf("tui.launch.board_cmd_request_desc", ref),
			Category:    i18n.T("tui.board.category_actions"),
			Action:      func() { qa.LaunchRequest(w.ID, ticket, ref) },
		})
	}
	return out
}
