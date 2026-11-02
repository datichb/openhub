package cmd

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
)

// The --json options write through cliout.JSON (indented, two spaces) or
// cliout.JSONCompact (one line). A script that reads them must not see the
// choice change, so each one is frozen here: the options that have always
// been compact stay compact.

func assertCompact(t *testing.T, name, out string) {
	t.Helper()
	require.True(t, strings.HasSuffix(out, "\n"), "%s: ends with a newline", name)
	assert.Equal(t, 1, strings.Count(out, "\n"), "%s: one line: %q", name, out)
	assert.True(t, strings.HasPrefix(out, "{") || strings.HasPrefix(out, "[") || out == "null\n", "%s: JSON document: %q", name, out)
}

func assertIndented(t *testing.T, name, out string) {
	t.Helper()
	require.True(t, strings.HasSuffix(out, "\n"), "%s: ends with a newline", name)
	assert.Greater(t, strings.Count(out, "\n"), 1, "%s: several lines: %q", name, out)
	assert.Contains(t, out, "\n  ", "%s: two-space indent: %q", name, out)
}

func TestJSONShapeMCPListIsCompact(t *testing.T) {
	c := mcpListCmd()
	require.NoError(t, c.Flags().Set("json", "true"))
	var out bytes.Buffer
	c.SetOut(&out)
	require.NoError(t, c.RunE(c, nil))
	assertCompact(t, "mcp list", out.String())
	assert.Contains(t, out.String(), `"name":"figma"`)
}

func TestJSONShapeGCDryRunIsCompact(t *testing.T) {
	_, _ = gcTestApp(t)
	withTerminal(t, false)
	out, err := gcRun(t, "--dry-run", "--json")
	require.NoError(t, err)
	assertCompact(t, "gc --dry-run --json", out)
	assert.Contains(t, out, `"applied":false`)
}

func TestJSONShapeBudgetReportIsIndented(t *testing.T) {
	seedBudgetReport(t)
	out, err := budgetReportRun(t, "--json")
	require.NoError(t, err)
	assertIndented(t, "budget report --json", out)
}

func TestJSONShapeWorkflowListIsIndented(t *testing.T) {
	var out bytes.Buffer
	ws := &workflowsvc.Workspace{Workflows: []workflowsvc.Summary{{}}}
	require.NoError(t, printWorkflowList(&out, ws, true))
	assertIndented(t, "workflow list --json", out.String())
}

func TestJSONShapeServeResponseIsCompact(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, map[string]string{"a": "b"})
	assert.Equal(t, "{\"a\":\"b\"}\n", rec.Body.String())
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	rec = httptest.NewRecorder()
	writeError(rec, assert.AnError, 500)
	assertCompact(t, "serve error", rec.Body.String())
}
