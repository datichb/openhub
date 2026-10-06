package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/prefsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// TUI wiring of the workflow catalogue (P1-T26, read only): the
// WorkflowService catalogue mapped to the view, pins in the active context.

const tuiCatalogProblems = 3

// newWorkflowCatalogView builds the catalogue view.
func newWorkflowCatalogView(start *tuiStart) *views.WorkflowCatalogView {
	return views.NewWorkflowCatalogView(views.WorkflowCatalogConfig{
		Load: func(ctx context.Context) ([]views.CatalogEntry, error) { return loadCatalogEntries(ctx, start) },
		Launch: func(id string) {
			openLaunchForm(start.a, tuiLaunchRequest{WorkflowID: id, ProjectID: activeScope().ProjectID})
		},
		TogglePin: func(id string) { start.togglePin(activeScope(), views.StartEntry{ID: id}) },
	})
}

// openWorkflowCatalog shows the catalogue.
func openWorkflowCatalog() {
	if tuiShell != nil {
		tuiShell.NavigateTo("workflows")
	}
}

// activeScope is the « Démarrer » scope of the current TUI context.
func activeScope() views.StartScope {
	if tuiShell == nil {
		return views.StartScope{}
	}
	switch tuiShell.Mode() {
	case views.ModeProject:
		if p := tuiShell.ActiveProject(); p != nil {
			return views.StartScope{ProjectID: p.ID}
		}
	case views.ModeTeam:
		if t := tuiShell.ActiveTeam(); t != nil {
			return views.StartScope{TeamID: t.ID}
		}
	}
	return views.StartScope{}
}

func loadCatalogEntries(ctx context.Context, start *tuiStart) ([]views.CatalogEntry, error) {
	scope := activeScope()
	list, err := newWorkflowService(ctx).Catalog(ctx, workflowsvc.Context{ProjectID: scope.ProjectID, TeamID: scope.TeamID})
	if err != nil {
		return nil, err
	}
	pinned := map[string]bool{}
	if entries, err := start.prefs.Start(ctx, prefsvc.Context{ProjectID: scope.ProjectID, TeamID: scope.TeamID}, nil); err == nil {
		for _, p := range entries.Pinned {
			pinned[p.WorkflowID] = true
		}
	}
	out := make([]views.CatalogEntry, 0, len(list))
	for _, s := range list {
		e := views.CatalogEntry{ID: s.ID, Layer: string(s.Layer), Version: s.Version, Label: s.Label, Desc: s.Description,
			Risk: string(s.Risk), Entry: s.EntryAgent, Chain: s.Chain, Valid: s.Valid, Pinned: pinned[s.ID]}
		for _, r := range s.Runtimes {
			e.Runtimes = append(e.Runtimes, string(r))
		}
		for _, in := range s.Inputs {
			label := in.ID + " (" + string(in.Type) + ")"
			if in.Required {
				label += " *"
			}
			e.Inputs = append(e.Inputs, label)
		}
		for _, d := range s.Diagnostics.Errors() {
			if len(e.Problems) == tuiCatalogProblems {
				break
			}
			e.Problems = append(e.Problems, d.Message)
		}
		out = append(out, e)
	}
	return out, nil
}
