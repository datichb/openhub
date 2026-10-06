package cmd

import (
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// workflowIntegrityChecks reports, for every configured team-state, the
// published workflows skipped by the integrity check (workflows.lock).
func workflowIntegrityChecks() []views.DoctorCheck {
	a := TryApp()
	if a == nil {
		return nil
	}
	var out []views.DoctorCheck
	for _, t := range a.Config.Teams {
		if !t.Enabled {
			continue
		}
		path := t.StatePath
		if path == "" {
			path = config.DefaultTeamStatePath()
		}
		repo := teamstate.NewRepo(t.StateRepo, path)
		if !repo.IsCloned() {
			continue
		}
		out = append(out, workflowIntegrityCheck(t.DisplayName(), repo))
	}
	return out
}

// workflowIntegrityCheck loads every scope of repo and summarizes the
// integrity warnings.
func workflowIntegrityCheck(name string, repo *teamstate.Repo) views.DoctorCheck {
	scopes := []teamstate.WorkflowScope{teamstate.TeamScope()}
	projects, _ := repo.WorkflowProjects()
	for _, p := range projects {
		scopes = append(scopes, teamstate.ProjectScope(p))
	}
	cat := workflow.NewMemCatalog()
	diags := repo.LoadWorkflowScopes(cat, scopes...)
	var skipped []string
	for _, d := range diags {
		if teamstate.IsIntegrityDiag(d) {
			skipped = append(skipped, filepath.Base(d.Source))
		}
	}
	check := views.DoctorCheck{Name: i18n.Tf("teamstate.workflow.doctor.name", name), OK: len(skipped) == 0}
	if check.OK {
		check.Detail = i18n.Tf("teamstate.workflow.doctor.ok", len(cat.Refs()))
	} else {
		check.Detail = i18n.Tf("teamstate.workflow.doctor.issues", len(cat.Refs()), len(skipped), strings.Join(skipped, ", "))
	}
	return check
}
