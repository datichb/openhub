package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// workflowTeamLayers is the team-state context of a workflow command: the
// team-state holding the team layer and, optionally, the project whose
// layer is loaded too.
type workflowTeamLayers struct {
	Repo    *teamstate.Repo
	Project string // project id; empty = team layer only
	TeamID  string
	Member  string
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
	return &workflowTeamLayers{Repo: repo, Project: projectID, TeamID: tc.TeamID, Member: tc.MemberID}, nil
}

// workflowTeamState is the WorkflowService resolver of the team-state of a
// context: the project's team (solo included), else the given team, else
// the active team. No team-state: hub layer only.
func workflowTeamState(ctx context.Context, c workflowsvc.Context) (*workflowsvc.TeamState, error) {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil, nil
	}
	var layers *workflowTeamLayers
	var err error
	switch {
	case c.ProjectID != "":
		if a.Projects == nil {
			return nil, nil
		}
		if _, gerr := a.Projects.Get(ctx, c.ProjectID); gerr != nil {
			return nil, nil //nolint:nilerr // unknown project: hub layer only
		}
		layers, err = resolveWorkflowTeamLayers(ctx, a, c.ProjectID)
	case c.TeamID != "":
		t := a.Config.FindTeam(c.TeamID)
		if t == nil || !t.Enabled {
			return nil, nil
		}
		path := t.StatePath
		if path == "" {
			path = config.DefaultTeamStatePath()
		}
		if repo := teamstate.NewRepo(t.StateRepo, path); repo.IsCloned() {
			layers = &workflowTeamLayers{Repo: repo, TeamID: t.ID, Member: t.MemberID}
		}
	default:
		layers, err = resolveWorkflowTeamLayers(ctx, a, "")
	}
	if err != nil || layers == nil {
		return nil, err
	}
	return &workflowsvc.TeamState{Repo: layers.Repo, TeamID: layers.TeamID, Member: layers.Member, Project: layers.Project}, nil
}
