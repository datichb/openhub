package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Editor check on a real team-state (P2-T14): diagnostics with lines of the
// edited document, locks, prompt preview, bundle preview (golden, numbers
// normalized).
func TestTUIEditorCheck(t *testing.T) {
	setupWorkflowCLI(t, "alice")
	prev := i18n.Locale()
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	ctx := t.Context()
	c := workflowsvc.Context{}

	chk, err := editorCheck(ctx, c, workflow.LayerTeam, []byte("apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: hub:ticket\nrisk: nope\n"), nil)
	require.NoError(t, err)
	require.NotEmpty(t, chk.Diagnostics)
	assert.True(t, chk.Diagnostics[0].Error)
	assert.Equal(t, 5, chk.Diagnostics[0].Line, "line of the edited document")
	assert.Empty(t, chk.Bundle)

	text := "apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: hub:ticket\ndescription: Hotfix\nprompt: { template: prompts/ticket.md.tmpl }\n"
	chk, err = editorCheck(ctx, c, workflow.LayerTeam, []byte(text), []byte("Hotfix {{ .ticket }} sur {{ .branch }}"))
	require.NoError(t, err)
	assert.Zero(t, chk.Errors(), "%+v", chk.Diagnostics)
	require.NotNil(t, chk.Spec)
	assert.Equal(t, "Hotfix", chk.Spec.Description.Text(""))
	assert.Equal(t, "hub", string(chk.Origins["risk"].Layer))
	assert.Contains(t, chk.Prompt, "Hotfix bd-42", "prompt rendered with example values")
	assert.Empty(t, chk.BundleErr)

	got := regexp.MustCompile(`\d+`).ReplaceAllString(strings.Join(chk.Bundle, "\n")+"\n", "N")
	path := filepath.Join("testdata", "editor_preview.golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file (UPDATE_GOLDEN=1 go test ./cmd -run TestTUIEditorCheck)")
	assert.Equal(t, string(want), got)

	// Locks of the parent chain.
	_, err = runWorkflowSub(t, workflowNewCmd, "locked", "--file", writeTemp(t, "apiVersion: oh/v1\nkind: Workflow\nid: locked\nextends: hub:ticket\nenforce: [risk]\n"), "--no-edit")
	require.NoError(t, err)
	_, err = newWorkflowService(ctx).Publish(ctx, c, "team:locked", "v1")
	require.NoError(t, err)
	chk, err = editorCheck(ctx, c, workflow.LayerTeam, []byte("apiVersion: oh/v1\nkind: Workflow\nid: locked-2\nextends: team:locked\n"), nil)
	require.NoError(t, err)
	assert.Contains(t, chk.Locked, "risk")
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "w.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}
