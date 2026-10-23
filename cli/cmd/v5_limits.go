package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/prefsvc"
	"github.com/datichb/openhub/cli/internal/services/launch"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Session restrictions (I6) at launch: hub.toml [limits], team config.toml
// [limits.recommended|enforced], project preference "limits", workflow
// limits: (see internal/limits).

// projectLimitsKey is the preference key of the project restrictions.
const projectLimitsKey = launch.ProjectLimitsKey

// sessionLimits resolves the restrictions of a session of project (spec =
// the workflow, nil without one).
func sessionLimits(ctx context.Context, a *app.App, project *domain.Project, spec *workflow.Spec) limits.Resolved {
	return launch.SessionLimits(ctx, a.Config, a.Preferences, project, spec)
}

// projectLimits reads the restrictions of a project (preferences).
func projectLimits(ctx context.Context, a *app.App, projectID string) limits.Limits {
	return launch.ProjectLimits(ctx, a.Preferences, projectID)
}

// saveProjectLimits writes the restrictions of a project (none = removed).
func saveProjectLimits(ctx context.Context, a *app.App, projectID string, l limits.Limits) error {
	scope := domain.ProjectPreferenceScope(projectID)
	if l.IsZero() {
		err := a.Preferences.Delete(ctx, scope, projectLimitsKey)
		if err != nil && err != domain.ErrNotFound {
			return err
		}
		return nil
	}
	return prefsvc.New(a.Preferences, nil).Set(ctx, scope, projectLimitsKey, l)
}
