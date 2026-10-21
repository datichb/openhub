package checkpoint

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func TestIdentityMatches(t *testing.T) {
	ada := Identity{Name: "Ada Lovelace", Email: "Ada@Example.com"}
	assert.True(t, ada.Matches([]string{"ada  lovelace"}), "name, case and spaces ignored")
	assert.True(t, ada.Matches([]string{"Bob", "ada@example.com"}), "e-mail")
	assert.False(t, ada.Matches([]string{"Bob", "lead@example.com"}))
	assert.False(t, ada.Matches(nil))
	assert.False(t, Identity{Name: "ada@example.com"}.Matches([]string{"ada@example.com"}), "an e-mail entry is compared with the e-mail")
	assert.False(t, Identity{}.Matches([]string{""}))
	assert.Equal(t, "Ada Lovelace <Ada@Example.com>", ada.String())
	assert.Equal(t, "Ada", Identity{Name: "Ada"}.String())
	assert.Equal(t, "<a@b.c>", Identity{Email: "a@b.c"}.String())
}

// approverWorkflow: cp-2 names its approvers (team layer, CP-R9).
func approverService(t *testing.T, who Identity) (*Service, context.Context) {
	t.Helper()
	svc, ctx := newTestService(t)
	wf := TestWorkflow
	wf.Checkpoints = append([]sessionspec.CheckpointDef(nil), TestWorkflow.Checkpoints...)
	wf.Checkpoints[1].Approver = []string{"Grace Hopper", "lead@example.com"}
	writeBundle(t, svc.BundlesDir, "h_wf", &wf)
	svc.Identity = func(context.Context, string) Identity { return who }
	return svc, ctx
}

// On this machine a validation by someone else than the approver is
// warned and traced, never refused (step 1).
func TestApproverMismatchTraced(t *testing.T) {
	svc, ctx := approverService(t, Identity{Name: "Ada Lovelace", Email: "ada@example.com"})
	tool := &fakeTool{}

	ac := svc.Approver(ctx, "ses_a", "cp-2")
	assert.Equal(t, []string{"Grace Hopper", "lead@example.com"}, ac.Approver)
	assert.True(t, ac.Mismatch)
	assert.Equal(t, "Grace Hopper, lead@example.com", ac.ApproverText())
	assert.False(t, svc.Approver(ctx, "ses_a", "cp-1").Expected())
	assert.False(t, svc.Approver(ctx, "ses_a", "cp-1").Mismatch)

	d := cpDecision(t, svc, ctx, "cp-2")
	assert.Equal(t, []string{"Grace Hopper", "lead@example.com"}, PayloadApprover(d.Payload.Data))
	assert.Equal(t, []string{"a", "b"}, PayloadApprover(map[string]any{DataApprover: []any{"a", "b"}}), "read back from JSON")

	// Refused: who refused is traced too.
	require.NoError(t, svc.Resolve(ctx, tool, *d, ChoiceFix, "encore"))
	d = cpDecision(t, svc, ctx, "cp-2")
	require.NoError(t, svc.Resolve(ctx, tool, *d, ChoiceApprove, ""))
	_, err := svc.Reached(ctx, "ses_a", Call{ID: "cp-2"})
	require.NoError(t, err, "not refused on this machine")

	st, _ := svc.State(ctx, "ses_a")
	var refused, passed *domain.CheckpointEvent
	for i, e := range st.Timeline {
		switch {
		case e.Kind == domain.CheckpointRefused && e.ID == "cp-2":
			refused = &st.Timeline[i]
		case e.Kind == domain.CheckpointPassed && e.ID == "cp-2":
			passed = &st.Timeline[i]
		}
	}
	require.NotNil(t, refused)
	require.NotNil(t, passed)
	for _, e := range []*domain.CheckpointEvent{refused, passed} {
		assert.Equal(t, "Ada Lovelace <ada@example.com>", e.Identity)
		assert.True(t, e.ApproverMismatch)
		assert.Equal(t, []string{"Grace Hopper", "lead@example.com"}, e.Approver)
		assert.Equal(t, "oh", e.By)
		assert.False(t, e.At.IsZero())
	}
}

func TestApproverMatchTraced(t *testing.T) {
	svc, ctx := approverService(t, Identity{Name: "Someone", Email: "LEAD@example.com"})
	tool := &fakeTool{}
	assert.False(t, svc.Approver(ctx, "ses_a", "cp-2").Mismatch)

	d := cpDecision(t, svc, ctx, "cp-2")
	require.NoError(t, svc.Resolve(ctx, tool, *d, ChoiceApprove, ""))
	_, err := svc.Reached(ctx, "ses_a", Call{ID: "cp-2"})
	require.NoError(t, err)
	st, _ := svc.State(ctx, "ses_a")
	last := st.Timeline[len(st.Timeline)-1]
	assert.Equal(t, domain.CheckpointPassed, last.Kind)
	assert.Equal(t, "Someone <LEAD@example.com>", last.Identity)
	assert.False(t, last.ApproverMismatch)
}

// Every validation names who made it, with or without an approver; the
// mode passes nobody's.
func TestValidationIdentityWithoutApprover(t *testing.T) {
	svc, ctx := newTestService(t)
	_, err := svc.Reached(ctx, "ses_a", Call{ID: "cp-1"})
	require.NoError(t, err)
	d := cpDecision(t, svc, ctx, "cp-2")
	_, hasApprover := d.Payload.Data[DataApprover]
	assert.False(t, hasApprover)

	// Validated in the tool (observed by oh).
	svc.ToolReplied(ctx, *d, true)
	_, err = svc.Reached(ctx, "ses_a", Call{ID: "cp-2"})
	require.NoError(t, err)
	st, _ := svc.State(ctx, "ses_a")
	first, last := st.Timeline[0], st.Timeline[len(st.Timeline)-1]
	assert.Equal(t, "mode", first.By)
	assert.Empty(t, first.Identity)
	assert.Equal(t, string(domain.ResolvedByTool), last.By)
	assert.Equal(t, "Ada Lovelace <ada@example.com>", last.Identity)
	assert.False(t, last.ApproverMismatch)
	assert.Empty(t, last.Approver)
}

func TestGitIdentity(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.name", "Ada Lovelace"}, {"config", "user.email", "ada@example.com"}} {
		gittest.Run(t, dir, args...)
	}
	assert.Equal(t, Identity{Name: "Ada Lovelace", Email: "ada@example.com"}, GitIdentity(context.Background(), dir))
}
