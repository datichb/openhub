package views

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// ─────────────────────────────────────────────────────────────────────────────
// Board quick actions (P1-T25, 10-tui §6): the workflows that take a Beads
// ticket, launched on the selected ticket — the launch form opens on its
// options step with the ticket prefilled (location, worktree and opening are
// chosen there).
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

	// ResolveProjectByDirID resolves a project path and ID from a team-state
	// directory ID (team board: tickets of several projects).
	ResolveProjectByDirID func(dirID string) (projectID, projectPath string, ok bool)
	// FetchDescription fetches the ticket description (best-effort). May be nil.
	FetchDescription func(projectPath, ticketID string) string
}

// showQuickActionModal lists the workflows applicable to a ticket.
func showQuickActionModal(shell ShellAccess, ticket TicketContext, qa *BoardQuickActions) {
	if shell == nil || qa == nil || qa.Workflows == nil {
		return
	}
	wfs := qa.Workflows()
	if len(wfs) == 0 {
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
	opts := make([]SelectOption, len(wfs))
	for i, w := range wfs {
		opts[i] = SelectOption{Label: ticketWorkflowLabel(w), Value: w.ID}
	}
	shell.ShowSelectModal(title, opts, "", func(id string) {
		if qa.Launch != nil {
			qa.Launch(id, ticket)
		}
	})
}

// ticketWorkflowLabel renders « ★ ticket  Implémenter  write ⌂ ▣ ».
func ticketWorkflowLabel(w TicketWorkflow) string {
	star := "  "
	if w.Pinned {
		star = "★ "
	}
	label := w.Label
	if r := []rune(label); len(r) > 34 {
		label = string(r[:33]) + "…"
	}
	return strings.TrimRight(fmt.Sprintf("%s%-16s %-34s %-7s %s", star, w.ID, label, w.Risk, runtimeIcons(w.Runtimes)), " ")
}

// ticketContextCommands are the omnibar commands of a selected ticket: one
// per workflow taking a ticket (« run ticket ⟨bd-42⟩ »).
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
	return out
}
