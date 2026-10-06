package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/datichb/openhub/cli/internal/teamstate"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// Team-state layers (v5 phase 2): the published team and project workflows
// of the context's team-state, integrity-checked against workflows.lock,
// and the current member's drafts on demand.

// TeamState is the team-state of a context.
type TeamState struct {
	Repo *teamstate.Repo
	// TeamID identifies the team (display).
	TeamID string
	// Member is the current member (drafts, publications).
	Member string
	// Project is the project id whose layer is loaded ("" = team only).
	Project string
}

// TeamStateFunc returns the team-state of a context, nil when none is
// configured (hub layer only).
type TeamStateFunc func(ctx context.Context, c Context) (*TeamState, error)

// ErrNoTeamState is returned by the editing methods without a team-state.
var ErrNoTeamState = errors.New("no team-state for this context")

// teamState returns the team-state of c (nil without resolver or team).
func (s *Service) teamState(ctx context.Context, c Context) (*TeamState, error) {
	if s.TeamState == nil {
		return nil, nil
	}
	return s.TeamState(ctx, c)
}

// scopes are the scopes loaded for ts, team first.
func (ts *TeamState) scopes() []teamstate.WorkflowScope {
	out := []teamstate.WorkflowScope{teamstate.TeamScope()}
	if ts.Project != "" {
		out = append(out, teamstate.ProjectScope(ts.Project))
	}
	return out
}

// scopeOf maps a document layer to the scope of ts.
func (ts *TeamState) scopeOf(layer wf.Layer) (teamstate.WorkflowScope, error) {
	switch layer {
	case wf.LayerTeam:
		return teamstate.TeamScope(), nil
	case wf.LayerProject:
		if ts.Project == "" {
			return teamstate.WorkflowScope{}, errors.New("project layer without a project")
		}
		return teamstate.ProjectScope(ts.Project), nil
	}
	return teamstate.WorkflowScope{}, fmt.Errorf("layer %q is not stored in the team-state", layer)
}

// addTeamLayers loads the published workflows of the context's team-state
// into cat (called by load).
func (s *Service) addTeamLayers(ctx context.Context, c Context, cat *catalog) error {
	ts, err := s.teamState(ctx, c)
	if err != nil || ts == nil {
		return err
	}
	cat.diags = append(cat.diags, ts.Repo.LoadWorkflowLayers(cat.docs, ts.Project)...)
	cat.env.Prompts = teamstate.PromptSource{Repo: ts.Repo, Fallback: cat.env.Prompts}
	cat.team = ts
	return s.useTeamBricks(cat, ts)
}

// Integrity returns the team-state files skipped by the integrity check
// (hand-edited or unpublished files, workflows.lock problems) and the
// refused team bricks, for the catalogue and Doctor.
func (s *Service) Integrity(ctx context.Context, c Context) (wf.Diagnostics, error) {
	cat, err := s.load(ctx, c)
	if err != nil {
		return nil, err
	}
	var out wf.Diagnostics
	for _, d := range cat.diags {
		if teamstate.IsIntegrityDiag(d) || isBrickDiag(d) {
			out = append(out, d)
		}
	}
	return out, nil
}

func isBrickDiag(d wf.Diagnostic) bool {
	switch d.Code {
	case DiagBrickCollision, DiagBrickBadExtends, DiagBrickDuplicate, DiagBrickAnnexConflict:
		return true
	}
	return false
}
