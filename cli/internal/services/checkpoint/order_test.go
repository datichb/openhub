package checkpoint

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// orderWorkflow is a spec-like runtime (r5-fixes): the planner writes its
// spec once cp-understanding is passed; cp-converge waits for
// cp-understanding and cp-spec for cp-converge.
var orderWorkflow = sessionspec.WorkflowRuntime{
	ID: "spec",
	Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-understanding", Label: map[string]string{"": "Compréhension"}, Behaviors: map[string]string{"manuel": "pause", "semi-auto": "pause", "auto": "auto"}},
		{ID: "cp-converge", Label: map[string]string{"": "Convergence"}, Behaviors: map[string]string{"manuel": "auto", "semi-auto": "auto", "auto": "auto"}, After: "cp-understanding"},
		{ID: "cp-spec", Label: map[string]string{"": "Spec"}, Behaviors: map[string]string{"manuel": "pause", "semi-auto": "pause", "auto": "pause"}, Mandatory: true, After: "cp-converge"},
	},
	WritesGates: []sessionspec.WritesGate{{Agent: "planner", Paths: []string{"docs/wiki/specs/**"}, After: "cp-understanding"}},
	Writes:      map[string][]string{"planner": {"docs/wiki/specs/**", "docs/wiki/index.md"}},
}

// p6-debts: the lines of a writes: agent the rules cannot read are read by
// oh: allowed when they only name declared paths, refused with the reason
// otherwise; another agent or another command keeps the decision of the tool.
func TestEvaluateWrites(t *testing.T) {
	svc, ctx := newOrderService(t, "semi-auto")
	ok := []string{"git add docs/wiki/specs/a.md docs/wiki/index.md", `git commit -m "docs(spec): a" -m "corps" -- docs/wiki/specs/a.md docs/wiki/index.md`}
	eff, _, err := svc.EvaluateWrites(ctx, "ses_o", "planner", sessionspec.ActionShell, ok)
	require.NoError(t, err)
	assert.Equal(t, "allow", eff)
	eff, msg, err := svc.EvaluateWrites(ctx, "ses_o", "planner", sessionspec.ActionShell, []string{ok[0], `git commit -m "x" -- docs/wiki/specs/a.md src/a.go`})
	require.NoError(t, err)
	assert.Equal(t, "deny", eff)
	assert.Contains(t, msg, "src/a.go")
	for _, c := range []struct {
		agent, action string
		res           []string
	}{
		{"developer", sessionspec.ActionShell, ok},
		{"planner", sessionspec.ActionShell, []string{ok[0], "npm test"}},
		{"planner", sessionspec.ActionEdit, []string{"docs/wiki/specs/a.md"}},
		{"", sessionspec.ActionShell, ok},
	} {
		eff, _, err := svc.EvaluateWrites(ctx, "ses_o", c.agent, c.action, c.res)
		require.NoError(t, err)
		assert.Empty(t, eff, "%+v", c)
	}
}

func newOrderService(t *testing.T, mode string) (*Service, context.Context) {
	t.Helper()
	svc, ctx := newTestService(t)
	writeBundle(t, svc.BundlesDir, "h_order", &orderWorkflow)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_o", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h_order", Mode: mode, State: domain.RunActive}))
	return svc, ctx
}

func passUnderstanding(t *testing.T, svc *Service, ctx context.Context) {
	t.Helper()
	d, auto, _, err := svc.Asked(ctx, "ses_o", "ses_o", adapters.PendingDecision{ID: "per_u", Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-understanding"}}})
	require.NoError(t, err)
	require.False(t, auto)
	svc.ToolReplied(ctx, *d, true)
	_, err = svc.Reached(ctx, "ses_o", Call{ID: "cp-understanding"})
	require.NoError(t, err)
}

// r5-fixes: before cp-understanding, oh refuses the edit of the spec with a
// message that names the checkpoint to wait for; once it is passed, the
// decision of the tool stands (the planner's own allow).
func TestEvaluateEditWaitsForTheCheckpoint(t *testing.T) {
	svc, ctx := newOrderService(t, "semi-auto")
	eff, msg, err := svc.EvaluateEdit(ctx, "ses_o", sessionspec.ActionEdit, []string{"docs/wiki/specs/export.md"})
	require.NoError(t, err)
	assert.Equal(t, string(sessionspec.EffectDeny), eff)
	assert.Contains(t, msg, "docs/wiki/specs/export.md")
	assert.Contains(t, msg, "cp-understanding")
	if i18n.Locale() == "fr" {
		assert.Contains(t, msg, "attends")
	}
	for _, c := range []struct {
		action string
		res    []string
	}{
		{sessionspec.ActionEdit, []string{"docs/wiki/index.md"}},
		{sessionspec.ActionShell, []string{"docs/wiki/specs/export.md"}},
		{sessionspec.ActionEdit, nil},
	} {
		eff, _, err := svc.EvaluateEdit(ctx, "ses_o", c.action, c.res)
		require.NoError(t, err)
		assert.Empty(t, eff, "%s %v", c.action, c.res)
	}
	st, err := svc.Status(ctx, "ses_o")
	require.NoError(t, err)
	assert.Equal(t, orderWorkflow.WritesGates, st.LockedWrites)
	rules, err := svc.Rules(ctx, "ses_o")
	require.NoError(t, err)
	assert.Equal(t, sessionspec.EffectAsk, bundle.ToolEffect(rules, sessionspec.ActionEdit, "docs/wiki/specs/export.md"))

	passUnderstanding(t, svc, ctx)
	eff, _, err = svc.EvaluateEdit(ctx, "ses_o", sessionspec.ActionEdit, []string{"docs/wiki/specs/export.md"})
	require.NoError(t, err)
	assert.Empty(t, eff)
	st, _ = svc.Status(ctx, "ses_o")
	assert.Empty(t, st.LockedWrites)
	rules, _ = svc.Rules(ctx, "ses_o")
	for _, r := range rules {
		assert.NotEqual(t, sessionspec.ActionEdit, r.Action, "the ask rule is gone: %v", r)
	}

	// A session without a workflow: nothing to say.
	eff, _, err = svc.EvaluateEdit(ctx, "ses_plain", sessionspec.ActionEdit, []string{"docs/wiki/specs/export.md"})
	require.NoError(t, err)
	assert.Empty(t, eff)
}

// r5-fixes: a checkpoint called before the one it waits for is refused with
// the checkpoint to pass first; it never asks the user.
func TestCheckpointOrder(t *testing.T) {
	svc, ctx := newOrderService(t, "semi-auto")

	eff, err := svc.Evaluate(ctx, "ses_o", bundle.CheckpointAction(), map[string]any{"id": "cp-spec"})
	require.NoError(t, err)
	assert.Equal(t, "allow", eff, "let through: the call fails with the reason")
	_, auto, _, err := svc.Asked(ctx, "ses_o", "ses_o", adapters.PendingDecision{ID: "per_s", Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-spec"}}})
	require.NoError(t, err)
	assert.True(t, auto, "no decision for a checkpoint out of order")

	for _, id := range []string{"cp-converge", "cp-spec"} {
		_, err = svc.Reached(ctx, "ses_o", Call{ID: id})
		require.ErrorIs(t, err, ErrOutOfOrder, id)
		var oe *OrderError
		require.ErrorAs(t, err, &oe)
		assert.Contains(t, err.Error(), id)
		assert.Contains(t, err.Error(), oe.First)
	}
	cs, err := svc.State(ctx, "ses_o")
	require.NoError(t, err)
	assert.False(t, cs.HasPassed("cp-converge"))

	passUnderstanding(t, svc, ctx)
	_, err = svc.Reached(ctx, "ses_o", Call{ID: "cp-spec"})
	var oe *OrderError
	require.ErrorAs(t, err, &oe, "cp-spec waits for cp-converge")
	assert.Equal(t, "cp-converge", oe.First)
	_, err = svc.Reached(ctx, "ses_o", Call{ID: "cp-converge"})
	require.NoError(t, err)
	eff, _ = svc.Evaluate(ctx, "ses_o", bundle.CheckpointAction(), map[string]any{"id": "cp-spec"})
	assert.Equal(t, "ask", eff, "in order: cp-spec waits for the user again")
}

// A checkpoint skipped in the mode, or absent from the session, does not
// hold the one that waits for it.
func TestCheckpointOrderSkippedOrAbsent(t *testing.T) {
	wf := orderWorkflow
	wf.Checkpoints = append([]sessionspec.CheckpointDef(nil), orderWorkflow.Checkpoints...)
	wf.Checkpoints[0].Behaviors = map[string]string{"semi-auto": "skip"}
	assert.Empty(t, bundle.CheckpointWaitsFor(&wf, "semi-auto", nil, wf.Checkpoints[1]))
	assert.Empty(t, bundle.LockedWrites(&wf, "semi-auto", nil))
	wf.Checkpoints[1].After = "cp-tdd-only"
	assert.Empty(t, bundle.CheckpointWaitsFor(&wf, "semi-auto", nil, wf.Checkpoints[1]))
	assert.Equal(t, "cp-converge", bundle.CheckpointWaitsFor(&orderWorkflow, "semi-auto", nil, orderWorkflow.Checkpoints[2]))
}
