package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
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
