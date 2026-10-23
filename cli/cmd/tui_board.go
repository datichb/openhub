package cmd

import (
	"context"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// buildBoardQuickActions wires the board quick actions (P1-T25): the
// workflows taking a Beads ticket, from the « Démarrer » cache, launched on
// the ticket through the launch form (options step, ticket prefilled). On the
// card of a GitLab issue, the workflows taking a free request (spec, quick…)
// are offered too, launched with « #<iid> » as the request: the form opens on
// its inputs, where the request can be completed.
func buildBoardQuickActions(a *app.App) *views.BoardQuickActions {
	return &views.BoardQuickActions{
		Workflows: func() []views.TicketWorkflow {
			if tuiStartWiring == nil {
				return nil
			}
			return tuiStartWiring.ticketWorkflows(activeScope())
		},
		Launch: func(workflowID string, ticket views.TicketContext) {
			openLaunchForm(a, boardLaunchRequest(workflowID, ticket, tuiLaunchRequest{Tickets: []string{ticket.ID}, AtOptions: true}))
		},
		RequestWorkflows: func() []views.TicketWorkflow {
			if tuiStartWiring == nil {
				return nil
			}
			return tuiStartWiring.requestWorkflows(activeScope())
		},
		IssueRef: func(ticket views.TicketContext) string { return boardIssueRef(a, ticket) },
		LaunchRequest: func(workflowID string, ticket views.TicketContext, request string) {
			openLaunchForm(a, boardLaunchRequest(workflowID, ticket, tuiLaunchRequest{Prefill: map[string]string{boardRequestInput: request}}))
		},
		ResolveProjectByDirID: buildResolveProjectByDirID(a),
		FetchDescription: func(projectPath, ticketID string) string {
			detail, err := beads.Show(projectPath, ticketID)
			if err != nil || detail == nil {
				return ""
			}
			return detail.Description
		},
	}
}

// boardLaunchRequest completes req with the workflow and the project of the
// card (the active project when the card has none).
func boardLaunchRequest(workflowID string, ticket views.TicketContext, req tuiLaunchRequest) tuiLaunchRequest {
	req.WorkflowID, req.ProjectID = workflowID, ticket.ProjectID
	if req.ProjectID == "" && tuiShell != nil {
		if p := tuiShell.ActiveProject(); p != nil {
			req.ProjectID = p.ID
		}
	}
	return req
}

// boardIssueRef returns « #<iid> » when the card is an issue of the GitLab
// tracker of its project: the team board names a GitLab issue by its bare
// IID (a Beads ticket never is a number, a Jira key has its project), so a
// number is an issue where the tracker is GitLab. "" otherwise.
func boardIssueRef(a *app.App, ticket views.TicketContext) string {
	if ticket.ID == "" || strings.Trim(ticket.ID, "0123456789") != "" || a == nil {
		return ""
	}
	var project *domain.Project
	if ticket.ProjectID != "" && a.Projects != nil {
		project, _ = a.Projects.Get(context.Background(), ticket.ProjectID)
	}
	if project == nil {
		project, _ = resolveActiveProject(a)
	}
	if teamService(a).TrackerOf(project).Type != "gitlab" {
		return ""
	}
	return "#" + ticket.ID
}

// buildResolveProjectByDirID builds a resolver that maps team-state directory IDs
// to project IDs and paths from the hub's project registry.
func buildResolveProjectByDirID(a *app.App) func(dirID string) (string, string, bool) {
	if a.Projects == nil {
		return nil
	}
	// Pre-load the mapping once — project list doesn't change during a TUI session.
	projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
	byID := make(map[string]*domain.Project, len(projects))
	for i := range projects {
		byID[projects[i].ID] = &projects[i]
	}
	return func(dirID string) (string, string, bool) {
		// dirID from team-state is usually the project's directory name / ID.
		if p, ok := byID[dirID]; ok {
			return p.ID, p.Path, true
		}
		return "", "", false
	}
}
