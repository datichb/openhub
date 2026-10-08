package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// A6: oh workflow show (and validate) resolve the chain down to the project
// layer of the current project, like oh workflow list and oh run, and accept
// project:<id>.
func TestWorkflowShowProjectLayer(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	useLocale(t, "en")
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
	useLocale(t, "en")
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
	useLocale(t, "en")
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
