package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// newBricksView wires the read-only brick catalogue (P3-T27): bricks of the
// active project context (hub, merged with the team catalogue), read by the
// workflow service off the event loop.
func newBricksView(a *app.App) *views.BricksView {
	return views.NewBricksView(views.BricksViewConfig{
		Load: func(ctx context.Context) ([]views.BrickRow, error) {
			c := workflowsvc.Context{}
			if p, err := resolveActiveProject(a); err == nil && p != nil {
				c.ProjectID = p.ID
			}
			entries, err := newWorkflowService(ctx).BrickCatalog(ctx, c)
			if err != nil {
				return nil, err
			}
			rows := make([]views.BrickRow, len(entries))
			for i, e := range entries {
				rows[i] = views.BrickRow{Kind: string(e.Kind), ID: e.ID, Name: e.Name, Label: e.Label, Description: e.Description,
					Family: e.Family, Mode: e.Mode, Origin: string(e.Origin), Skills: e.Skills, Requires: e.Requires,
					Tokens: e.Tokens, Agents: e.Agents, Workflows: e.Workflows}
			}
			return rows, nil
		},
		OpenWorkflow: func(id string) { openLaunchForm(a, tuiLaunchRequest{WorkflowID: id}) },
	})
}
