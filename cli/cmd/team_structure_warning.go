package cmd

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// structureWarningText words a failed commit of the structure of the
// team-state (teamstate.StructureWarning) for the user; "" for nil.
func structureWarningText(err error) string {
	if err == nil {
		return ""
	}
	var w *teamstate.StructureWarning
	if errors.As(err, &w) {
		err = w.Cause
	}
	return i18n.Tf("cmd.team.init.structure_warning", localizeError(err))
}

// ensureTeamStructure creates the structure of the team-state and commits it.
// A commit that fails is not an error: the folders exist and the team works,
// the next call commits them. It is the warning the caller shows ("" when
// there is none).
func ensureTeamStructure(ctx context.Context, repo *teamstate.Repo) (warning string, err error) {
	err = repo.EnsureStructure(ctx)
	if teamstate.IsStructureWarning(err) {
		return structureWarningText(err), nil
	}
	return "", err
}
