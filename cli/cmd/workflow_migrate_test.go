package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// memLegacyStore is the legacy column of projects (after v38).
type memLegacyStore map[string]string

func (m memLegacyStore) LegacyWorkflowConfigs(context.Context) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range m {
		if v != "" {
			out[k] = v
		}
	}
	return out, nil
}

func (m memLegacyStore) ClearLegacyWorkflowConfig(_ context.Context, id string) error {
	delete(m, id)
	return nil
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

// Fixtures of the former configurations (acceptance criterion 5).
const (
	legacyTeamConfig = `[workflow.overrides]
circuit_breaker_override = 9

[[workflow.overrides.checkpoint_overrides]]
id = "cp-1"
action = "modify"
label = "Démarrer (équipe)"

[workflow.overrides.checkpoint_overrides.behavior]
semi-auto = "pause"

[[workflow.overrides.checkpoint_overrides]]
id = "cp-routing"
action = "modify"
label = "Routage"

[[workflow.overrides.agent_overrides]]
agent_id = "documentarian"
disabled = true
`
	legacyProjectWeb  = `{"overrides":{"checkpoint_overrides":[{"id":"cp-3","action":"remove"}],"mode_overrides":{"default":"manuel"}}}`
	legacyProjectSolo = `{"overrides":{"agent_overrides":[{"agent_id":"designer","role":"independent"}]}}`
)

// Acceptance criterion 5: the former workflow configurations (team
// config.toml, projects.workflow_config, hub.toml) migrate without loss.
func TestMigrateLegacyWorkflows(t *testing.T) {
	env := setupWorkflowCLI(t, "alice", "bob")
	t.Setenv(workflowsDirEnv, filepath.Join(repoRoot(t), "workflows")) // the shipped feature workflow
	ctx := t.Context()

	alice := env.clones["alice"]
	require.NoError(t, os.WriteFile(filepath.Join(alice, "config.toml"), []byte(legacyTeamConfig), 0o644))
	gitIn(t, alice, "add", "config.toml")
	gitIn(t, alice, "commit", "-m", "former team workflow config")
	gitIn(t, alice, "push")

	a := application
	team := "acme"
	a.Projects = &mockProjectStore{projects: []domain.Project{
		{ID: "web", Name: "web", Path: t.TempDir(), TeamID: &team},
		{ID: "lonely", Name: "lonely", Path: t.TempDir()},
	}}
	require.NoError(t, config.Save(a.Config))
	appendFile(t, config.ConfigPath(), "\n[workflow.overrides]\n[[workflow.overrides.checkpoint_overrides]]\nid = \"cp-1\"\naction = \"modify\"\nlabel = \"Démarrer (hub)\"\n")
	config.Reset() // the file changed outside Save (former section written by hand)
	reloaded, err := config.Load()
	require.NoError(t, err)
	*a.Config = *reloaded
	legacy := memLegacyStore{"web": legacyProjectWeb, "lonely": legacyProjectSolo, "gone": legacyProjectWeb}

	require.True(t, legacyWorkflowPending(ctx, a, legacy))
	m := migrateLegacyWorkflows(ctx, a, legacy)
	require.Empty(t, m.Errors)
	notices := strings.Join(m.Notices, "\n")

	// Team: archived, translated, published, removed from config.toml.
	repo := teamstate.NewRepo(env.bare, alice)
	_, err = repo.LoadConfig()
	require.NoError(t, err)
	assert.Nil(t, repoTeamWorkflow(repo), "[workflow] removed from config.toml")
	raw, err := os.ReadFile(filepath.Join(alice, "workflows", "migrated", "team-config-workflow.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "Démarrer (équipe)", "nothing lost")
	assert.Contains(t, string(raw), "cp-routing")
	svc := newWorkflowService(ctx)
	res, err := svc.Resolve(ctx, workflowsvcContext(""), "team:feature", resolveNone())
	require.NoError(t, err)
	cp1, _ := res.Spec.Checkpoints.Get("cp-1")
	assert.Equal(t, "Démarrer (équipe)", cp1.Label.Text("fr"))
	assert.Equal(t, workflow.BehaviorPause, cp1.Behavior("semi-auto"))
	_, hasDoc := res.Spec.Agents.Get("documentarian")
	assert.False(t, hasDoc)
	assert.Equal(t, 9, *res.Spec.CircuitBreaker.MaxConsecutiveSubagents)
	published, err := os.ReadFile(filepath.Join(alice, "workflows", "published", "feature.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(published), "# Not translated: checkpoint cp-routing")
	assert.Contains(t, gitIn(t, env.bare, "log", "--format=%s"), "workflow: migrate the team workflow configuration")

	// Project of the team: extends team:feature.
	res, err = svc.Resolve(ctx, workflowsvcContext("web"), "project:feature", resolveNone())
	require.NoError(t, err)
	assert.Equal(t, "team:feature", res.Chain[1].String())
	_, hasCP3 := res.Spec.Checkpoints.Get("cp-3")
	assert.False(t, hasCP3)
	assert.Equal(t, "manuel", res.Spec.Modes.Default)
	assert.FileExists(t, filepath.Join(alice, "projects", "web", "workflows", "migrated", "project-workflow-config.json"))

	// Project without team: a solo space is created and attached.
	lonely, err := a.Projects.Get(ctx, "lonely")
	require.NoError(t, err)
	require.NotNil(t, lonely.TeamID)
	solo := a.Config.FindTeam(*lonely.TeamID)
	require.NotNil(t, solo)
	assert.True(t, solo.Solo)
	res, err = svc.Resolve(ctx, workflowsvcContext("lonely"), "project:feature", resolveNone())
	require.NoError(t, err)
	assert.Equal(t, "hub:feature", res.Chain[0].String())
	designer, _ := res.Spec.Agents.Get("designer")
	assert.Equal(t, workflow.RoleIndependent, designer.Role)
	assert.Contains(t, notices, "solo")

	// Hub: files under ~/.oh/migrated, section removed from hub.toml.
	assert.Nil(t, hubWorkflowConfig(a))
	hubRaw, err := os.ReadFile(filepath.Join(config.HubDir(), "migrated", "hub-workflow-overrides.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(hubRaw), "Démarrer (hub)")
	assert.FileExists(t, filepath.Join(config.HubDir(), "migrated", "feature.hub.yaml"))
	config.Reset()
	_, err = config.Load()
	require.NoError(t, err)
	hubData, err := os.ReadFile(config.ConfigPath())
	require.NoError(t, err)
	assert.NotContains(t, string(hubData), "[workflow")

	// Legacy values cleared (unknown project included), nothing left.
	assert.Empty(t, legacy)
	assert.False(t, legacyWorkflowPending(ctx, a, legacy))
	m = migrateLegacyWorkflows(ctx, a, legacy)
	assert.Empty(t, m.Notices, "idempotent")

	// Bob, after a sync: nothing to migrate (config.toml already migrated).
	env.as(t, "bob")
	gitIn(t, env.clones["bob"], "pull")
	assert.False(t, legacyWorkflowPending(ctx, application, memLegacyStore{}))
}

func TestMigrateLegacyEnforcedTeam(t *testing.T) {
	env := setupWorkflowCLI(t, "alice")
	t.Setenv(workflowsDirEnv, filepath.Join(repoRoot(t), "workflows"))
	ctx := t.Context()
	alice := env.clones["alice"]
	require.NoError(t, os.WriteFile(filepath.Join(alice, "config.toml"), []byte("[workflow]\nenforced = true\n"), 0o644))
	gitIn(t, alice, "add", "config.toml")
	gitIn(t, alice, "commit", "-m", "enforced")
	gitIn(t, alice, "push")
	team := "acme"
	application.Projects = &mockProjectStore{projects: []domain.Project{{ID: "web", Name: "web", Path: t.TempDir(), TeamID: &team}}}
	legacy := memLegacyStore{"web": legacyProjectWeb}

	m := migrateLegacyWorkflows(ctx, application, legacy)
	// The team locks its workflow: the project overrides (ignored before)
	// stay a draft, refused at publication.
	notices := strings.Join(m.Notices, "\n")
	assert.Contains(t, notices, "project:feature")
	published, err := os.ReadFile(filepath.Join(alice, "workflows", "published", "feature.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(published), `enforce: ["*"]`)
	assert.FileExists(t, filepath.Join(alice, "projects", "web", "workflows", "drafts", "alice", "feature.yaml"))
	assert.NoFileExists(t, filepath.Join(alice, "projects", "web", "workflows", "published", "feature.yaml"))
	assert.Empty(t, legacy, "the configuration lives on as the draft")
}

func workflowsvcContext(project string) workflowsvc.Context {
	return workflowsvc.Context{ProjectID: project}
}

func resolveNone() workflowsvc.ResolveOpts { return workflowsvc.ResolveOpts{} }

// hub.toml overrides are read from the file: the configuration loader does
// not map their snake_case keys (they looked empty in memory).
func TestMigrateHubOverridesFromFile(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	t.Setenv(workflowsDirEnv, filepath.Join(repoRoot(t), "workflows"))
	require.NoError(t, os.WriteFile(config.ConfigPath(), []byte("[cli]\nsetup_done = true\n\n[[workflow.overrides.checkpoint_overrides]]\nid = \"cp-1\"\naction = \"modify\"\nlabel = \"Go\"\n"), 0o644))
	config.Reset()
	cfg, err := config.Load()
	require.NoError(t, err)
	application.Config = cfg
	require.True(t, legacyWorkflowPending(t.Context(), application, nil))
	m := migrateLegacyWorkflows(t.Context(), application, nil)
	require.Empty(t, m.Errors)
	data, err := os.ReadFile(filepath.Join(config.HubDir(), "migrated", "feature.hub.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `cp-1: { label: "Go" }`)
	hub, _ := os.ReadFile(config.ConfigPath())
	assert.NotContains(t, string(hub), "workflow")
	assert.False(t, legacyWorkflowPending(t.Context(), application, nil))
}

func appendFile(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = f.WriteString(text)
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

// A7: a project configuration in an unknown format (or unreadable) is
// archived with a notice, never cleared silently.
func TestMigrateUnknownProjectConfigArchived(t *testing.T) {
	useLocale(t, "en")
	home := t.TempDir()
	t.Setenv("OH_HOME", home)
	a := newMockApp(nil, nil)
	a.Projects = &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "web"}}}
	store := memLegacyStore{
		"p1": `{"mode_overrides":{"default":"manuel"}}`,
		"p2": `not json`,
		"p3": `{"overrides":{}}`,
	}
	m := &workflowMigration{}
	migrateProjectWorkflowConfigs(t.Context(), a, nil, store, m, nil)
	require.Empty(t, m.Errors)
	assert.Empty(t, store, "archived or empty configurations are cleared")
	require.Len(t, m.Notices, 2)
	assert.Contains(t, m.Notices[0]+m.Notices[1], "web")
	for id, raw := range map[string]string{"p1": `{"mode_overrides":{"default":"manuel"}}`, "p2": `not json`} {
		data, err := os.ReadFile(filepath.Join(home, "migrated", "project-"+id+"-workflow-config.json"))
		require.NoError(t, err, id)
		assert.Equal(t, raw, string(data))
	}
	assert.NoFileExists(t, filepath.Join(home, "migrated", "project-p3-workflow-config.json"))
}
