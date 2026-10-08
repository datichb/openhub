package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Migration of the former workflow configurations to the team-state (v5
// phase 2, P2-T11, migration v38). The former overrides modified the
// hard-coded workflow, whose checkpoints and agents live on in the shipped
// `feature` workflow:
//
//   - team-state config.toml [workflow] → team draft `feature` (extends
//     hub:feature; `enforced` → `enforce: ["*"]`), published when the team
//     has no `feature` yet; [workflow] removed from config.toml;
//   - projects.workflow_config (moved to workflow_config_legacy by v38) →
//     project draft `feature` in the project's team-state (a solo space is
//     created for a project without team), published when valid;
//   - hub.toml [workflow.overrides] (hub development only) → files under
//     ~/.oh/migrated/, not loaded; the section is removed from hub.toml.
//
// Nothing is lost: the raw configuration is archived next to the result
// (workflows/migrated/…), what has no oh/v1 equivalent is listed in the
// draft. Every step is idempotent and retried at the next start when it
// could not finish (offline, hub not extracted).

// legacyWorkflowStore reads the former project configurations.
type legacyWorkflowStore interface {
	LegacyWorkflowConfigs(ctx context.Context) (map[string]string, error)
	ClearLegacyWorkflowConfig(ctx context.Context, id string) error
}

// workflowMigration reports what the migration did.
type workflowMigration struct {
	Notices []string
	Errors  []error
}

func (m *workflowMigration) notice(key string, args ...any) {
	m.Notices = append(m.Notices, i18n.Tf("teamstate.workflow.migrate."+key, args...))
}

// The former configuration sections, read only here (their types left the
// configuration packages with the former workflow view, P2-T18): hub.toml
// and team-state config.toml [workflow], projects.workflow_config_legacy.
type (
	legacyHubWorkflow struct {
		Overrides *workflow.WorkflowOverride `toml:"overrides,omitempty"`
	}
	legacyTeamWorkflow struct {
		Overrides *workflow.WorkflowOverride `toml:"overrides,omitempty"`
		Enforced  *bool                      `toml:"enforced,omitempty"`
	}
	legacyProjectWorkflow struct {
		Overrides *workflow.WorkflowOverride `json:"overrides,omitempty"`
	}
)

func (w *legacyTeamWorkflow) isEnforced() bool { return w != nil && w.Enforced != nil && *w.Enforced }

// hubWorkflowConfig returns hub.toml [workflow], read from the file itself
// (the configuration has no such section any more; its loader never mapped
// the snake_case keys of the overrides anyway).
func hubWorkflowConfig(_ *app.App) *legacyHubWorkflow {
	data, err := os.ReadFile(config.ConfigPath())
	if err != nil {
		return nil
	}
	var raw struct {
		Workflow *legacyHubWorkflow `toml:"workflow"`
	}
	if toml.Unmarshal(data, &raw) != nil {
		return nil
	}
	return raw.Workflow
}

// teamWorkflowSection returns config.toml [workflow] of a team-state.
func teamWorkflowSection(data []byte) *legacyTeamWorkflow {
	var raw struct {
		Workflow *legacyTeamWorkflow `toml:"workflow"`
	}
	if toml.Unmarshal(data, &raw) != nil {
		return nil
	}
	return raw.Workflow
}

func repoTeamWorkflow(repo *teamstate.Repo) *legacyTeamWorkflow {
	data, err := os.ReadFile(filepath.Join(repo.Path(), "config.toml"))
	if err != nil {
		return nil
	}
	return teamWorkflowSection(data)
}

// legacyWorkflowPending reports cheaply whether something is left to migrate.
func legacyWorkflowPending(ctx context.Context, a *app.App, store legacyWorkflowStore) bool {
	if hubWorkflowConfig(a) != nil {
		return true
	}
	if store != nil {
		if m, err := store.LegacyWorkflowConfigs(ctx); err == nil && len(m) > 0 {
			return true
		}
	}
	for _, t := range a.Config.Teams {
		if repo := teamRepoOf(t); repo != nil && repoTeamWorkflow(repo) != nil {
			return true
		}
	}
	return false
}

// migrateLegacyWorkflowsAtStartup runs the migration when something is left
// and prints a short notice (stderr).
func migrateLegacyWorkflowsAtStartup(ctx context.Context, w io.Writer) {
	a := TryApp()
	if a == nil || store == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ps := legacyStoreOf(a)
	if !legacyWorkflowPending(ctx, a, ps) {
		return
	}
	m := migrateLegacyWorkflows(ctx, a, ps)
	for _, n := range m.Notices {
		fmt.Fprintln(w, "oh: "+n)
	}
	for _, err := range m.Errors {
		slog.Warn("workflow migration", "error", err)
	}
}

func legacyStoreOf(a *app.App) legacyWorkflowStore {
	if s, ok := a.Projects.(legacyWorkflowStore); ok {
		return s
	}
	return nil
}

func teamRepoOf(t config.TeamConfig) *teamstate.Repo {
	if !t.Enabled {
		return nil
	}
	path := t.StatePath
	if path == "" {
		path = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(t.StateRepo, path)
	if !repo.IsCloned() {
		return nil
	}
	return repo
}

// migrateLegacyWorkflows migrates hub, team and project configurations.
func migrateLegacyWorkflows(ctx context.Context, a *app.App, store legacyWorkflowStore) *workflowMigration {
	m := &workflowMigration{}
	svc := newWorkflowService(ctx)
	feature, err := svc.Resolve(ctx, workflowsvc.Context{}, "hub:"+workflow.LegacyWorkflowID, workflowsvc.ResolveOpts{})
	if feature == nil {
		m.Errors = append(m.Errors, fmt.Errorf("hub:%s unavailable, migration postponed: %w", workflow.LegacyWorkflowID, err))
		return m
	}
	known := feature.Spec.Checkpoints.Keys()

	migrateHubWorkflowConfig(a, m, known)
	for _, t := range a.Config.Teams {
		migrateTeamWorkflowConfig(ctx, svc, t, m, known)
	}
	if store != nil {
		migrateProjectWorkflowConfigs(ctx, a, svc, store, m, known)
	}
	return m
}

// migrateHubWorkflowConfig moves hub.toml [workflow] to ~/.oh/migrated/.
func migrateHubWorkflowConfig(a *app.App, m *workflowMigration, known []string) {
	wc := hubWorkflowConfig(a)
	if wc == nil {
		return
	}
	if wc.Overrides != nil && !wc.Overrides.IsEmpty() {
		dir := filepath.Join(config.HubDir(), "migrated")
		raw, err := toml.Marshal(struct {
			Workflow *legacyHubWorkflow `toml:"workflow"`
		}{wc})
		if err == nil {
			err = os.MkdirAll(dir, 0o755)
		}
		patch := workflow.TranslateLegacyOverride(wc.Overrides, workflow.LegacyWorkflowID, "hub:"+workflow.LegacyWorkflowID, wc.Overrides.Enforced, known)
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, "hub-workflow-overrides.toml"), raw, 0o644)
		}
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, workflow.LegacyWorkflowID+".hub.yaml"), patch.YAML, 0o644)
		}
		if err != nil {
			m.Errors = append(m.Errors, fmt.Errorf("hub workflow overrides: %w", err))
			return
		}
		m.notice("hub", dir)
	}
	// Saving the configuration drops the section (no such field any more).
	if err := config.Save(a.Config); err != nil {
		m.Errors = append(m.Errors, fmt.Errorf("writing hub.toml: %w", err))
	}
}

// migrateTeamWorkflowConfig moves config.toml [workflow] of a team-state to
// a team draft `feature`, published when the team has none.
func migrateTeamWorkflowConfig(ctx context.Context, svc *workflowsvc.Service, t config.TeamConfig, m *workflowMigration, known []string) {
	repo := teamRepoOf(t)
	if repo == nil || t.MemberID == "" {
		return
	}
	if repoTeamWorkflow(repo) == nil {
		return
	}
	id := workflow.LegacyWorkflowID
	draftCreated := false
	err := repo.Transact(ctx, func(_ context.Context, tx *teamstate.Tx) (teamstate.TxResult, error) {
		draftCreated = false
		data, err := tx.ReadFile("config.toml")
		if err != nil {
			return teamstate.TxResult{}, err
		}
		section := teamWorkflowSection(data)
		if section == nil { // migrated by another member meanwhile
			return teamstate.TxResult{}, nil
		}
		cfg, err := tx.Config()
		if err != nil {
			return teamstate.TxResult{}, err
		}
		raw, err := toml.Marshal(struct {
			Workflow *legacyTeamWorkflow `toml:"workflow"`
		}{section})
		if err != nil {
			return teamstate.TxResult{}, err
		}
		patch := workflow.TranslateLegacyOverride(section.Overrides, id, "hub:"+id, section.isEnforced(), known)
		dir := filepath.Join(teamstate.WorkflowsDirName, "migrated")
		if err := tx.WriteFile(filepath.Join(dir, "team-config-workflow.toml"), raw); err != nil {
			return teamstate.TxResult{}, err
		}
		if err := tx.WriteFile(filepath.Join(dir, id+".yaml"), patch.YAML); err != nil {
			return teamstate.TxResult{}, err
		}
		if draftCreated, err = writeMigrationDraft(tx, teamstate.TeamScope(), t.MemberID, id, patch.YAML); err != nil {
			return teamstate.TxResult{}, err
		}
		// Writing the configuration drops [workflow] (no such field any more).
		if err := tx.WriteConfig(cfg); err != nil {
			return teamstate.TxResult{}, err
		}
		return teamstate.TxResult{Message: "workflow: migrate the team workflow configuration to team:" + id + " (v38)"}, nil
	})
	if err != nil {
		m.Errors = append(m.Errors, fmt.Errorf("team %s: %w", t.ID, err))
		return
	}
	m.notice("team", t.DisplayName())
	if draftCreated {
		publishMigration(ctx, svc, workflowsvc.Context{TeamID: t.ID}, repo, teamstate.TeamScope(), id, m)
	}
}

// migrateProjectWorkflowConfigs moves the former project configurations to
// project drafts `feature`.
func migrateProjectWorkflowConfigs(ctx context.Context, a *app.App, svc *workflowsvc.Service, store legacyWorkflowStore, m *workflowMigration, known []string) {
	legacy, err := store.LegacyWorkflowConfigs(ctx)
	if err != nil {
		m.Errors = append(m.Errors, err)
		return
	}
	for pid, raw := range legacy {
		var wc legacyProjectWorkflow
		var keys map[string]json.RawMessage
		jerr := json.Unmarshal([]byte(raw), &wc)
		if jerr == nil {
			jerr = json.Unmarshal([]byte(raw), &keys)
		}
		if jerr != nil || (wc.Overrides == nil || wc.Overrides.IsEmpty()) && len(keys) > 0 && !onlyEmptyOverrides(keys) {
			// Unreadable or unknown format (A7): archived, never dropped silently.
			if path, err := archiveLegacyProjectConfig(pid, raw); err != nil {
				m.Errors = append(m.Errors, fmt.Errorf("project %s: unknown workflow_config format (kept): %w", pid, err))
			} else {
				m.notice("project_unknown", projectLabel(ctx, a, pid), path)
				_ = store.ClearLegacyWorkflowConfig(ctx, pid)
			}
			continue
		}
		if wc.Overrides == nil || wc.Overrides.IsEmpty() {
			_ = store.ClearLegacyWorkflowConfig(ctx, pid)
			continue
		}
		project, err := a.Projects.Get(ctx, pid)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				_ = store.ClearLegacyWorkflowConfig(ctx, pid)
			}
			continue
		}
		repo, member, err := projectTeamStateForMigration(ctx, a, project, m)
		if err != nil {
			m.Errors = append(m.Errors, fmt.Errorf("project %s: %w", pid, err))
			continue
		}
		scope := teamstate.ProjectScope(project.ID)
		id := workflow.LegacyWorkflowID
		parent := "hub:" + id
		if lock, err := repo.ReadWorkflowLock(); err == nil {
			if _, ok := lock.Get(teamstate.TeamScope(), id); ok {
				parent = "team:" + id // must extend the nearest layer
			}
		}
		patch := workflow.TranslateLegacyOverride(wc.Overrides, id, parent, false, known)
		draftCreated := false
		err = repo.Transact(ctx, func(_ context.Context, tx *teamstate.Tx) (teamstate.TxResult, error) {
			wdir := filepath.Join("projects", project.ID, teamstate.WorkflowsDirName, "migrated")
			if err := tx.WriteFile(filepath.Join(wdir, "project-workflow-config.json"), []byte(raw)); err != nil {
				return teamstate.TxResult{}, err
			}
			if err := tx.WriteFile(filepath.Join(wdir, id+".yaml"), patch.YAML); err != nil {
				return teamstate.TxResult{}, err
			}
			var err error
			if draftCreated, err = writeMigrationDraft(tx, scope, member, id, patch.YAML); err != nil {
				return teamstate.TxResult{}, err
			}
			return teamstate.TxResult{Message: "workflow: migrate the workflow configuration of project " + project.ID + " (v38)"}, nil
		})
		if err != nil {
			m.Errors = append(m.Errors, fmt.Errorf("project %s: %w", pid, err))
			continue
		}
		if err := store.ClearLegacyWorkflowConfig(ctx, pid); err != nil {
			m.Errors = append(m.Errors, err)
		}
		m.notice("project", project.Name)
		if draftCreated {
			publishMigration(ctx, svc, workflowsvc.Context{ProjectID: project.ID}, repo, scope, id, m)
		}
	}
}

// onlyEmptyOverrides reports a configuration whose only key is an empty
// "overrides" (nothing to migrate, nothing to keep).
func onlyEmptyOverrides(keys map[string]json.RawMessage) bool {
	if len(keys) != 1 {
		return false
	}
	_, ok := keys["overrides"]
	return ok
}

// archiveLegacyProjectConfig keeps a project configuration oh cannot
// translate under ~/.oh/migrated/ and returns its path.
func archiveLegacyProjectConfig(projectID, raw string) (string, error) {
	dir := filepath.Join(config.HubDir(), "migrated")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "project-"+projectID+"-workflow-config.json")
	return path, os.WriteFile(path, []byte(raw), 0o600)
}

// projectLabel is the name of a project for a notice (its ID when unknown).
func projectLabel(ctx context.Context, a *app.App, id string) string {
	if a.Projects != nil {
		if p, err := a.Projects.Get(ctx, id); err == nil && p.Name != "" {
			return p.Name
		}
	}
	return id
}

// projectTeamStateForMigration returns the team-state of project, creating
// (or reusing) a solo space when it has no team.
func projectTeamStateForMigration(ctx context.Context, a *app.App, project *domain.Project, m *workflowMigration) (*teamstate.Repo, string, error) {
	tc := config.ResolveTeamForProject(a.Config, project)
	if tc.Enabled || tc.Solo {
		path := tc.StatePath
		if path == "" {
			path = config.DefaultTeamStatePath()
		}
		repo := teamstate.NewRepo(tc.StateRepo, path)
		if !repo.IsCloned() {
			return nil, "", fmt.Errorf("team-state %s not cloned", path)
		}
		return repo, tc.MemberID, nil
	}
	sp, err := attachProjectSolo(ctx, a, project)
	if err != nil {
		return nil, "", err
	}
	if sp.Created {
		m.notice("solo_created", sp.Team.ID, project.Name)
	} else {
		m.notice("solo_attached", project.Name, sp.Team.ID)
	}
	return sp.Repo, sp.Team.MemberID, nil
}

// writeMigrationDraft writes the translated document as the member's draft
// unless they already have one (the translation stays in migrated/).
func writeMigrationDraft(tx *teamstate.Tx, scope teamstate.WorkflowScope, member, id string, data []byte) (bool, error) {
	rel, err := teamstate.DraftRel(scope, member, id)
	if err != nil {
		return false, err
	}
	if _, err := tx.ReadFile(rel); err == nil {
		return false, nil
	}
	return true, tx.WriteFile(rel, data)
}

// publishMigration publishes the migrated draft when the scope has no
// published `feature` yet; otherwise, or when invalid, the draft is kept.
func publishMigration(ctx context.Context, svc *workflowsvc.Service, c workflowsvc.Context, repo *teamstate.Repo, scope teamstate.WorkflowScope, id string, m *workflowMigration) {
	ref := string(scope.Layer()) + ":" + id
	if lock, err := repo.ReadWorkflowLock(); err == nil {
		if _, ok := lock.Get(scope, id); ok {
			m.notice("draft_kept", ref)
			return
		}
	}
	p, err := svc.Publish(ctx, c, ref, i18n.T("teamstate.workflow.migrate.message"))
	switch {
	case err != nil:
		m.notice("draft_invalid", ref)
		m.Errors = append(m.Errors, fmt.Errorf("%s: %w", ref, err))
	case p.Queued:
		m.notice("queued", ref)
	default:
		m.notice("published", ref, p.Version)
	}
}
