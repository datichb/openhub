package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// buildBoardQuickActions wires the board quick actions (P1-T25): the
// workflows taking a Beads ticket, from the « Démarrer » cache, launched on
// the ticket through the launch form (options step, ticket prefilled).
func buildBoardQuickActions(a *app.App) *views.BoardQuickActions {
	return &views.BoardQuickActions{
		Workflows: func() []views.TicketWorkflow {
			if tuiStartWiring == nil {
				return nil
			}
			return tuiStartWiring.ticketWorkflows(activeScope())
		},
		Launch: func(workflowID string, ticket views.TicketContext) {
			req := tuiLaunchRequest{WorkflowID: workflowID, ProjectID: ticket.ProjectID, Tickets: []string{ticket.ID}, AtOptions: true}
			if req.ProjectID == "" && tuiShell != nil {
				if p := tuiShell.ActiveProject(); p != nil {
					req.ProjectID = p.ID
				}
			}
			openLaunchForm(a, req)
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
