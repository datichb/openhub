package cmd

import (
	"context"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/bundle"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// teamSkillProblems checks the skills of the team catalogue of the active
// team (catalog/skills/ of its team-state, solo space included) with the
// unknown_key rule of the hub. No team-state: no problem.
func teamSkillProblems(ctx context.Context) ([]bundle.SkillProblem, error) {
	ts, err := workflowTeamState(ctx, workflowsvc.Context{})
	if err != nil || ts == nil || ts.Repo == nil {
		return nil, nil //nolint:nilerr // no usable team-state: the hub is checked alone
	}
	return bundle.CheckTeamSkills(filepath.Join(ts.Repo.Path(), teamstate.CatalogDirName))
}
