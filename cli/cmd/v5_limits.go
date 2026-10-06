package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/prefsvc"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Session restrictions (I6) at launch: hub.toml [limits], team config.toml
// [limits.recommended|enforced], project preference "limits", workflow
// limits: (see internal/limits).

// projectLimitsKey is the preference key of the project restrictions.
const projectLimitsKey = "limits"

// sessionLimits resolves the restrictions of a session of project (spec =
// the workflow, nil without one).
func sessionLimits(ctx context.Context, a *app.App, project *domain.Project, spec *workflow.Spec) limits.Resolved {
	in := limits.Input{Hub: a.Config.Limits, ProjectID: project.ID}
	in.Team = teamLimits(a, project)
	in.Project = projectLimits(ctx, a, project.ID)
	if spec != nil && spec.Limits != nil {
		if spec.Limits.BudgetUSD != nil {
			in.Workflow.SessionBudgetUSD = *spec.Limits.BudgetUSD
		}
		in.Workflow.Models = spec.Limits.Models
	}
	return limits.Resolve(in)
}

// teamLimits reads the restrictions of the project's team (nil = none).
func teamLimits(a *app.App, project *domain.Project) *limits.TeamLimits {
	team := config.ResolveTeamForProject(a.Config, project)
	if !team.Enabled || team.StatePath == "" {
		return nil
	}
	repo := teamstate.NewRepo(team.StateRepo, team.StatePath)
	if !repo.IsCloned() {
		return nil
	}
	cfg, err := repo.LoadConfig()
	if err != nil {
		slog.Warn("team restrictions not read", "team", team.TeamID, "error", err)
		return nil
	}
	return cfg.Limits
}

// projectLimits reads the restrictions of a project (preferences).
func projectLimits(ctx context.Context, a *app.App, projectID string) limits.Limits {
	var l limits.Limits
	if a.Preferences == nil || projectID == "" {
		return l
	}
	if _, err := prefsvc.New(a.Preferences, nil).Get(ctx, domain.ProjectPreferenceScope(projectID), projectLimitsKey, &l); err != nil {
		slog.Warn("project restrictions not read", "project", projectID, "error", err)
	}
	return l
}

// budgetError translates a refusal of the restrictions (daily budget spent).
func budgetError(err error) error {
	var be *runsvc.DailyBudgetError
	if errors.As(err, &be) {
		return fmt.Errorf("%s", i18n.Tf("cmd.budget.daily_spent", be.Spent, be.Allowance, be.Scope))
	}
	return err
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
