package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/prefsvc"
	"github.com/datichb/openhub/cli/internal/services/launch"
)

// Session restrictions (I6) at launch: hub.toml [limits], team config.toml
// [limits.recommended|enforced], project preference "limits", workflow
// limits: (see internal/limits).

// saveProjectLimits writes the restrictions of a project (none = removed).
func saveProjectLimits(ctx context.Context, a *app.App, projectID string, l limits.Limits) error {
	scope := domain.ProjectPreferenceScope(projectID)
	if l.IsZero() {
		err := a.Preferences.Delete(ctx, scope, launch.ProjectLimitsKey)
		if err != nil && err != domain.ErrNotFound {
			return err
		}
		return nil
	}
	return prefsvc.New(a.Preferences, nil).Set(ctx, scope, launch.ProjectLimitsKey, l)
}
