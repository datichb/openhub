package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

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
