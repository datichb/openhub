package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// workflowIntegrityChecks reports, for every configured team-state, the
// published workflows skipped by the integrity check (workflows.lock).
func workflowIntegrityChecks() []doctor.Result {
	a := TryApp()
	if a == nil {
		return nil
	}
	var out []doctor.Result
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
		if c, ok := retiredExtendsCheck(t.DisplayName(), repo, hubWorkflowsDir()); ok {
			out = append(out, c)
		}
		if c, ok := fanoutNeverCheck(t.DisplayName(), repo, hubWorkflowsDir()); ok {
			out = append(out, c)
		}
	}
	return out
}

// retiredExtendsCheck reports the published team and project workflows
// that extend a hub workflow retired as an alias (hub:feature, hub:cadrage:
// feature-migration); ok is false when there is none (no check shown).
func retiredExtendsCheck(name string, repo *teamstate.Repo, hubWorkflows string) (doctor.Result, bool) {
	hub := workflow.NewMemCatalog()
	_ = hub.LoadDir(hubWorkflows, workflow.LayerHub)
	scopes := []teamstate.WorkflowScope{teamstate.TeamScope()}
	projects, _ := repo.WorkflowProjects()
	for _, p := range projects {
		scopes = append(scopes, teamstate.ProjectScope(p))
	}
	var found []string
	for _, sc := range scopes {
		cat := workflow.NewMemCatalog()
		_ = repo.LoadWorkflowScopes(cat, sc)
		for _, ref := range cat.Refs() {
			doc, _ := cat.Lookup(ref)
			parent, err := workflow.ParseRef(doc.Spec.Extends)
			if doc.Spec.Extends == "" || err != nil || parent.Layer != workflow.LayerHub {
				continue
			}
			if _, exists := hub.Lookup(parent); exists {
				continue
			}
			if targets := workflow.RetiredAliasTargets(hub, parent); len(targets) > 0 {
				found = append(found, i18n.Tf("teamstate.workflow.doctor.retired_item", sc.String()+":"+ref.ID, parent.String(), strings.Join(targets, ", ")))
			}
		}
	}
	if len(found) == 0 {
		return doctor.Result{}, false
	}
	return doctor.Result{Name: i18n.Tf("teamstate.workflow.doctor.retired_name", name), OK: false,
		Detail: i18n.Tf("teamstate.workflow.doctor.retired", strings.Join(found, " · "))}, true
}

// fanoutNeverCheck reports the published team and project workflows that
// change a checkpoint the children of a hub fan-out pass (fanout: never:
// cp-commit of sweep, cp-2 of implement-spec): the change does not reach a
// fan-out session (sweep-fanout); ok is false when there is none.
func fanoutNeverCheck(name string, repo *teamstate.Repo, hubWorkflows string) (doctor.Result, bool) {
	hub := workflow.NewMemCatalog()
	_ = hub.LoadDir(hubWorkflows, workflow.LayerHub)
	scopes := []teamstate.WorkflowScope{teamstate.TeamScope()}
	projects, _ := repo.WorkflowProjects()
	for _, p := range projects {
		scopes = append(scopes, teamstate.ProjectScope(p))
	}
	var found []string
	for _, sc := range scopes {
		cat := workflow.NewMemCatalog()
		_ = repo.LoadWorkflowScopes(cat, sc)
		found = append(found, fanoutNeverFindings(hub, sc.String(), cat)...)
	}
	if len(found) == 0 {
		return doctor.Result{}, false
	}
	return doctor.Result{Name: i18n.Tf("teamstate.workflow.doctor.fanout_never_name", name), OK: false,
		Detail: i18n.Tf("teamstate.workflow.doctor.fanout_never", strings.Join(found, " · "))}, true
}

// fanoutNeverFindings lists the documents of cat (scope) that extend a hub
// workflow with a fan-out and patch one of its `fanout: never` checkpoints.
func fanoutNeverFindings(hub *workflow.MemCatalog, scope string, cat *workflow.MemCatalog) []string {
	var found []string
	for _, ref := range cat.Refs() {
		doc, _ := cat.Lookup(ref)
		parent, err := workflow.ParseRef(doc.Spec.Extends)
		if doc.Spec.Extends == "" || err != nil || parent.Layer != workflow.LayerHub {
			continue
		}
		base, ok := hub.Lookup(parent)
		if !ok || base.Spec.Fanout == nil {
			continue
		}
		var cps []string
		for _, k := range doc.Spec.Checkpoints.Keys() {
			if cp, ok := base.Spec.Checkpoints.Get(k); ok && cp.Fanout == workflow.FanoutNever {
				cps = append(cps, k)
			}
		}
		if len(cps) > 0 {
			found = append(found, i18n.Tf("teamstate.workflow.doctor.fanout_never_item", scope+":"+ref.ID, strings.Join(cps, ", "), parent.String(), base.Spec.Fanout.Merge))
		}
	}
	return found
}

// workflowIntegrityCheck loads every scope of repo and summarizes the
// integrity warnings.
func workflowIntegrityCheck(name string, repo *teamstate.Repo) doctor.Result {
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
	// Refused team bricks (catalog/), reported with the skipped workflows.
	svc := &workflowsvc.Service{HubDir: findHubDir(), TeamState: func(context.Context, workflowsvc.Context) (*workflowsvc.TeamState, error) {
		return &workflowsvc.TeamState{Repo: repo}, nil
	}}
	if b, err := svc.Bricks(context.Background(), workflowsvc.Context{}); err == nil {
		for _, d := range b.Diagnostics {
			skipped = append(skipped, filepath.Base(d.Source))
		}
	}
	check := doctor.Result{Name: i18n.Tf("teamstate.workflow.doctor.name", name), OK: len(skipped) == 0}
	if check.OK {
		check.Detail = i18n.Tf("teamstate.workflow.doctor.ok", len(cat.Refs()))
	} else {
		check.Detail = i18n.Tf("teamstate.workflow.doctor.issues", len(cat.Refs()), len(skipped), strings.Join(skipped, ", "))
	}
	return check
}

// hubWorkflowsDir is the directory of the hub workflows (OH_WORKFLOWS_DIR
// overrides it, as for oh run).
func hubWorkflowsDir() string {
	if d := os.Getenv(workflowsDirEnv); d != "" {
		return d
	}
	return filepath.Join(findHubDir(), hubcat.WorkflowsDir)
}
