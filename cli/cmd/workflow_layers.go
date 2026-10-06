package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// workflowTeamLayers is the team-state context of a workflow command: the
// team-state holding the team layer and, optionally, the project whose
// layer is loaded too.
type workflowTeamLayers struct {
	Repo    *teamstate.Repo
	Project string // project id; empty = team layer only
}

// resolveWorkflowTeamLayers returns the team-state of project (by id or
// name), or of the active team when projectRef is empty. It returns nil when
// no team-state is configured or cloned: the hub layer is then used alone.
func resolveWorkflowTeamLayers(ctx context.Context, a *app.App, projectRef string) (*workflowTeamLayers, error) {
	if a == nil {
		return nil, nil
	}
	var tc config.ResolvedTeamConfig
	projectID := ""
	if projectRef != "" {
		p, err := resolveProject(ctx, a, projectRef)
		if err != nil {
			return nil, err
		}
		projectID = p.ID
		tc = resolvedTeamConfig(a, p)
	} else {
		tc = resolvedTeamConfig(a, nil)
	}
	if !tc.Enabled && !tc.Solo {
		return nil, nil
	}
	path := tc.StatePath
	if path == "" {
		path = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(tc.StateRepo, path)
	if !repo.IsCloned() {
		return nil, nil
	}
	return &workflowTeamLayers{Repo: repo, Project: projectID}, nil
}
