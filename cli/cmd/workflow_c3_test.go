package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// A6: oh workflow show (and validate) resolve the chain down to the project
// layer of the current project, like oh workflow list and oh run, and accept
// project:<id>.
func TestWorkflowShowProjectLayer(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	i18ntest.Set(t, "en")
	team := "acme"
	dir := t.TempDir()
	application.Projects = &mockProjectStore{projects: []domain.Project{{ID: "web", Name: "web", Path: dir, TeamID: &team}}}

	doc := writeTemp(t, "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\ndescription: Ticket du projet\n")
	out, err := runWorkflowSub(t, workflowNewCmd, "ticket", "--layer", "project", "-p", "web", "--file", doc, "--no-edit")
	require.NoError(t, err, out)
	out, err = runWorkflowSub(t, workflowPublishCmd, "project:ticket", "-p", "web", "-m", "projet", "--yes")
	require.NoError(t, err, out)

	out, err = runWorkflowSub(t, workflowShowCmd, "ticket", "-p", "web", "--origin")
	require.NoError(t, err, out)
	assert.Contains(t, out, "project:ticket")
	assert.Contains(t, out, "Ticket du projet")

	// From the project folder, without -p.
	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(wd) })
	out, err = runWorkflowSub(t, workflowShowCmd, "ticket")
	require.NoError(t, err, out)
	assert.Contains(t, out, "project:ticket")
	out, err = runWorkflowSub(t, workflowShowCmd, "project:ticket")
	require.NoError(t, err, out)
	out, err = runWorkflowSub(t, workflowValidateCmd, "project:ticket")
	require.NoError(t, err, out)
	assert.Contains(t, out, "project:ticket: valid")

	// Outside a project: the message says how to give one.
	require.NoError(t, os.Chdir(t.TempDir()))
	_, err = runWorkflowSub(t, workflowShowCmd, "project:ticket")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-p <project>")
}

// A9: oh workflow validate <layer>:<id> validates the member's draft when
// the workflow is not published.
func TestWorkflowValidateDraft(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	i18ntest.Set(t, "en")
	out, err := runWorkflowSub(t, workflowNewCmd, "ticket-hotfix", "--extends", "hub:ticket", "--no-edit")
	require.NoError(t, err, out)
	out, err = runWorkflowSub(t, workflowValidateCmd, "team:ticket-hotfix")
	require.NoError(t, err, out)
	assert.Contains(t, out, "team:ticket-hotfix (your draft, not published): valid")

	_, err = runWorkflowSub(t, workflowValidateCmd, "team:nope")
	require.Error(t, err)
}

// A10: oh workflow new --file with a document without prompt (nor extends)
// names its own template and creates the starter one, with a message.
func TestWorkflowNewFileWithoutPrompt(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	i18ntest.Set(t, "en")
	doc := writeTemp(t, "apiVersion: oh/v1\nkind: Workflow\nid: nettoyage\nrisk: write\nentry:\n  agent: developer\n"+
		"inputs:\n  request:\n    type: text\n    max_length: 4000\nagents:\n  developer: { role: workflow, mode: primary }\n")
	out, err := runWorkflowSub(t, workflowNewCmd, "nettoyage", "--file", doc, "--no-edit")
	require.NoError(t, err, out)
	assert.Contains(t, out, "prompt.template: prompts/nettoyage.md.tmpl added")
	assert.Contains(t, out, "Starter template created")
	c, err := workflowCmdContext(workflowNewCmd())
	require.NoError(t, err)
	txt, err := newWorkflowService(t.Context()).EditText(t.Context(), c, "team", "nettoyage")
	require.NoError(t, err)
	assert.Contains(t, string(txt.YAML), "template: prompts/nettoyage.md.tmpl")
	assert.Contains(t, string(txt.Prompt), `{{ data "request" .request }}`)
}

// feature-migration: oh doctor names the published workflows that extend a
// hub workflow retired as an alias (hub:feature → spec, implement-spec).
func TestRetiredExtendsCheck(t *testing.T) {
	env := setupWorkflowCLI(t, "alice")
	i18ntest.Set(t, "en")
	repo := teamstate.NewRepo(env.bare, env.clones["alice"])
	hub := filepath.Join(repoRoot(t), "cli", "internal", "services", "workflow", "testdata", "workflows")
	_, ok := retiredExtendsCheck("acme", repo, hub)
	assert.False(t, ok, "nothing published")

	doc := writeTemp(t, "apiVersion: oh/v1\nkind: Workflow\nid: feature\nextends: hub:feature\ndescription: Feature de l'équipe\n")
	out, err := runWorkflowSub(t, workflowNewCmd, "feature", "--file", doc, "--no-edit")
	require.NoError(t, err, out)
	out, err = runWorkflowSub(t, workflowPublishCmd, "team:feature", "-m", "équipe", "--yes")
	require.NoError(t, err, out)
	_, ok = retiredExtendsCheck("acme", repo, hub)
	assert.False(t, ok, "hub:feature still a workflow")

	// The hub retires feature: spec declares it as an alias.
	retired := t.TempDir()
	for _, f := range []string{"quick.yaml", "review.yaml", "ticket.yaml"} {
		data, err := os.ReadFile(filepath.Join(hub, f))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(retired, f), data, 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(retired, "spec.yaml"), []byte("apiVersion: oh/v1\nkind: Workflow\nid: spec\naliases: [feature]\n"), 0o644))
	c, ok := retiredExtendsCheck("acme", repo, retired)
	require.True(t, ok)
	assert.False(t, c.OK)
	assert.Contains(t, c.Detail, "team:feature extends hub:feature, now an alias of spec")
}

// sweep-fanout: oh doctor names the team and project workflows that change a
// checkpoint the children of a hub fan-out pass (cp-commit of sweep).
func TestFanoutNeverFindings(t *testing.T) {
	i18ntest.Set(t, "en")
	hub := workflow.NewMemCatalog()
	require.Empty(t, hub.LoadDir(filepath.Join(repoRoot(t), "workflows"), workflow.LayerHub).Errors())
	cat := workflow.NewMemCatalog()
	add := func(src string) {
		doc, diags := workflow.Parse([]byte(src), workflow.Source{Layer: workflow.LayerTeam, Path: "team/x.yaml"})
		require.False(t, diags.HasErrors(), "%v", diags)
		require.Empty(t, cat.Add(doc))
	}
	add("apiVersion: oh/v1\nkind: Workflow\nid: sweep\nextends: hub:sweep\ncheckpoints:\n  cp-commit: { verify: { from: stack, expect: pass, kinds: [build, test, typecheck, lint] } }\n  cp-recap: { mode: { auto: pause } }\n")
	add("apiVersion: oh/v1\nkind: Workflow\nid: quick\nextends: hub:quick\ncheckpoints:\n  cp-commit: { mode: { auto: pause } }\n")
	add("apiVersion: oh/v1\nkind: Workflow\nid: implement-spec\nextends: hub:implement-spec\ncheckpoints:\n  cp-final: { mode: { auto: pause } }\n")
	found := fanoutNeverFindings(hub, "team", cat)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, "team:sweep changes cp-commit of hub:sweep, absent from fan-out sessions (validated by cp-merge)", found[0])
}
