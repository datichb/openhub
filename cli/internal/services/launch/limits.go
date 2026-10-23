package launch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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

// ProjectLimitsKey is the preference key of the project restrictions.
const ProjectLimitsKey = "limits"

// SessionLimits resolves the restrictions of a session of project (spec =
// the workflow, nil without one).
func SessionLimits(ctx context.Context, cfg *config.Config, prefs domain.PreferenceStore, project *domain.Project, spec *workflow.Spec) limits.Resolved {
	in := limits.Input{Hub: cfg.Limits, ProjectID: project.ID}
	in.Team = TeamLimits(cfg, project)
	in.Project = ProjectLimits(ctx, prefs, project.ID)
	if spec != nil && spec.Limits != nil {
		if spec.Limits.BudgetUSD != nil {
			in.Workflow.SessionBudgetUSD = *spec.Limits.BudgetUSD
		}
		in.Workflow.Models = spec.Limits.Models
	}
	return limits.Resolve(in)
}

// TeamLimits reads the restrictions of the project's team (nil = none).
func TeamLimits(cfg *config.Config, project *domain.Project) *limits.TeamLimits {
	team := config.ResolveTeamForProject(cfg, project)
	if !team.Enabled || team.StatePath == "" {
		return nil
	}
	repo := teamstate.NewRepo(team.StateRepo, team.StatePath)
	if !repo.IsCloned() {
		return nil
	}
	tcfg, err := repo.LoadConfig()
	if err != nil {
		slog.Warn("team restrictions not read", "team", team.TeamID, "error", err)
		return nil
	}
	return tcfg.Limits
}

// ProjectLimits reads the restrictions of a project (preferences).
func ProjectLimits(ctx context.Context, prefs domain.PreferenceStore, projectID string) limits.Limits {
	var l limits.Limits
	if prefs == nil || projectID == "" {
		return l
	}
	if _, err := prefsvc.New(prefs, nil).Get(ctx, domain.ProjectPreferenceScope(projectID), ProjectLimitsKey, &l); err != nil {
		slog.Warn("project restrictions not read", "project", projectID, "error", err)
	}
	return l
}

// BudgetError translates a refusal of the restrictions (daily budget spent).
func BudgetError(err error) error {
	var be *runsvc.DailyBudgetError
	if errors.As(err, &be) {
		return fmt.Errorf("%s", i18n.Tf("cmd.budget.daily_spent", be.Spent, be.Allowance, be.Scope))
	}
	return err
}
