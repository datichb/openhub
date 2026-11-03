package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

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
