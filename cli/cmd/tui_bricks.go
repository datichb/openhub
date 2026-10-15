package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// newBricksView wires the read-only brick catalogue (P3-T27, tui-wiki-
// bricks): bricks of the active project context (hub, merged with the team
// catalogue), with their origin, version, publication and findings (the
// workspace of `oh brick list`), read off the event loop.
func newBricksView(a *app.App) *views.BricksView {
	return views.NewBricksView(views.BricksViewConfig{
		Load: func(ctx context.Context) ([]views.BrickRow, error) {
			c := workflowsvc.Context{}
			if p, err := resolveActiveProject(a); err == nil && p != nil {
				c.ProjectID = p.ID
			}
			svc := newWorkflowService(ctx)
			ws, err := svc.BrickWorkspace(ctx, c)
			if err != nil {
				return nil, err
			}
			var lock *teamstate.BricksLock
			if b, err := svc.Bricks(ctx, c); err == nil && b.Root != "" {
				lock, _, _ = teamstate.ReadBricksLock(b.Root)
			}
			return brickRows(ws, lock), nil
		},
		OpenWorkflow: func(id string) { openLaunchForm(a, tuiLaunchRequest{WorkflowID: id}) },
	})
}

// brickRows renders the workspace of the catalogue for the view.
func brickRows(ws *workflowsvc.BrickWorkspace, lock *teamstate.BricksLock) []views.BrickRow {
	unsealed := map[string]bool{}
	for _, k := range ws.Unsealed {
		unsealed[k] = true
	}
	rows := make([]views.BrickRow, len(ws.Bricks))
	for i, e := range ws.Bricks {
		r := views.BrickRow{Kind: string(e.Kind), ID: e.ID, Name: e.Name, Label: e.Label, Description: e.Description,
			Family: e.Family, Mode: e.Mode, Origin: string(e.Origin), Skills: e.Skills, Requires: e.Requires,
			Tokens: e.Tokens, Agents: e.Agents, Workflows: e.Workflows,
			TeamID: e.TeamID, Source: e.Source, Shadows: e.Shadows, Version: e.Version}
		if e.TeamID != "" {
			key := teamstate.BrickKey(string(e.Kind), e.TeamID)
			r.Unsealed = unsealed[key]
			if le, ok := lock.Get(string(e.Kind), e.TeamID); ok {
				r.PublishedBy, r.PublishedAt = le.PublishedBy, le.PublishedAt
			}
			for _, d := range ws.Problems[key] {
				if d.Code == workflowsvc.DiagBrickShadows {
					continue // the replacement is the origin, not a problem
				}
				r.Problems = append(r.Problems, views.BrickProblem{Error: d.Severity == wf.SeverityError, Message: d.Message})
			}
		}
		rows[i] = r
	}
	return rows
}
