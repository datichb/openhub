package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// The form follows the aliases of the YAML like oh run: feature with
// tickets opens implement-spec (title « feature → implement-spec (avec
// tickets) »), without tickets spec; the omnibar route opens its target.
func TestLaunchFormConfigAlias(t *testing.T) {
	testHubEnv(t)
	t.Setenv(workflowsDirEnv, "") // shipped workflows
	a := &app.App{Config: &config.Config{}}
	project := &domain.Project{ID: "p1", Name: "demo", Path: t.TempDir()}
	ctx := context.Background()
	for _, c := range []struct {
		req             tuiLaunchRequest
		id, alias, with string
		fanoutNote      bool
	}{
		{tuiLaunchRequest{WorkflowID: "feature", Tickets: []string{"bd-1"}}, "implement-spec", "feature", "tickets", true},
		{tuiLaunchRequest{WorkflowID: "feature"}, "spec", "feature", "", false},
		{tuiLaunchRequest{WorkflowID: "feature", AliasTarget: "implement-spec"}, "implement-spec", "feature", "tickets", true},
		{tuiLaunchRequest{WorkflowID: "implement-spec"}, "implement-spec", "", "", true},
	} {
		cfg, err := launchFormConfig(ctx, a, project, c.req)
		require.NoError(t, err, "%+v", c.req)
		assert.Equal(t, c.id, cfg.WorkflowID, "%+v", c.req)
		assert.Equal(t, c.alias, cfg.Alias, "%+v", c.req)
		assert.Equal(t, c.with, cfg.AliasWith, "%+v", c.req)
		assert.Equal(t, c.fanoutNote, cfg.InputNotes["fanout"] != "", "%+v: fan-out disabled on this machine", c.req)
	}
}

// `libre` offers its entry agents, the default one first (F-15); the
// agent asked is preselected.
func TestLaunchFormConfigEntryAgents(t *testing.T) {
	testHubEnv(t)
	t.Setenv(workflowsDirEnv, "")
	a := &app.App{Config: &config.Config{}}
	project := &domain.Project{ID: "p1", Name: "demo", Path: t.TempDir()}
	cfg, err := launchFormConfig(context.Background(), a, project, tuiLaunchRequest{WorkflowID: "libre", Agent: "debugger"})
	require.NoError(t, err)
	require.NotEmpty(t, cfg.EntryAgents)
	assert.Equal(t, "orchestrator", cfg.EntryAgents[0])
	assert.Contains(t, cfg.EntryAgents, "debugger")
	assert.Equal(t, "debugger", cfg.Agent)

	cfg, err = launchFormConfig(context.Background(), a, project, tuiLaunchRequest{WorkflowID: "spec"})
	require.NoError(t, err)
	assert.Empty(t, cfg.EntryAgents, "entry not selectable")
}

// What oh will compute for an input with `from:`: the session read, or
// that no session fits; never an error.
func TestTUIFromPreview(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions := sqlite.NewSessionStore(st)
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_41c0", ProjectID: "p1", WorkflowID: "spec", StartedAt: time.Now(),
		Status: domain.SessionStatusCompleted, State: domain.RunCompleted, Outputs: map[string]any{"spec": "docs/wiki/specs/export.md"}}))
	doc, diags := workflow.Parse([]byte(`apiVersion: oh/v1
kind: Workflow
id: implement-spec
risk: write
inputs:
  spec: { type: path, from: session.output(spec.spec) }
  tickets: { type: beads-ids, from: session.output(spec.tickets) }
  digest: { type: text, from: session.trace(spec) }
  mr: { type: string }
  feedback: { type: text, from: mr.discussions(mr) }
`), workflow.Source{Layer: workflow.LayerHub})
	require.False(t, diags.HasErrors(), "%v", diags)
	preview := tuiFromPreview(&app.App{Sessions: sessions}, &domain.Project{ID: "p1"}, "", doc.Spec)
	assert.Equal(t, i18n.Tf("tui.launch.input.from_value", "session.output(spec.spec)", "ses_41c0", "docs/wiki/specs/export.md"), preview(ctx, "spec", nil))
	assert.Equal(t, i18n.Tf("tui.launch.input.from_none", "session.output(spec.tickets)"), preview(ctx, "tickets", nil))
	assert.Equal(t, i18n.Tf("tui.launch.input.from_session", "session.trace(spec)", "ses_41c0"), preview(ctx, "digest", nil))
	assert.Equal(t, i18n.Tf("tui.launch.input.from_at_launch", "mr.discussions(mr)"), preview(ctx, "feedback", nil))
	assert.Empty(t, preview(ctx, "mr", nil))
}

// A path is looked for in the project, a branch among the local and
// remote branches; a warning otherwise.
func TestTUICheckInput(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
	check := tuiCheckInput(dir)
	ctx := context.Background()
	assert.Empty(t, check(ctx, workflow.Input{Type: workflow.InputPath}, "docs"))
	assert.Equal(t, i18n.T("tui.launch.input.path_missing"), check(ctx, workflow.Input{Type: workflow.InputPath}, "nope.md"))
	assert.Empty(t, check(ctx, workflow.Input{Type: workflow.InputBranch}, "feat/x"), "outside git: nothing to check")

	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.email=a@b", "-c", "user.name=a", "commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	assert.Empty(t, check(ctx, workflow.Input{Type: workflow.InputBranch}, "main"))
	assert.Equal(t, i18n.Tf("tui.launch.input.branch_missing", "feat/x"), check(ctx, workflow.Input{Type: workflow.InputBranch}, "feat/x"))
}
