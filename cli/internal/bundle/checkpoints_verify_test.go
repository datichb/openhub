package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// CP-R2 (R3): every shipped workflow stays within its pause budget in
// semi-auto: 2 human decisions, plus 1 per checkpoint that unlocks an
// irreversible action (workflow.PauseBudget). A team workflow over budget
// is only warned by oh workflow validate (pause_budget_exceeded).
func TestCheckpointPauseBudget(t *testing.T) {
	_, cat, _ := shippedWorkflows(t)
	for _, ref := range cat.Refs() {
		r, diags := workflow.ResolveSpec(cat, ref, nil)
		require.Empty(t, diags.Errors(), ref.ID)
		c := r.Spec.PauseBudget()
		assert.False(t, c.Exceeded(), "%s: %d pauses in %s (%v), budget %d (%d irreversible: %v)",
			ref.ID, len(c.Pauses), c.Mode, c.Pauses, c.Budget, len(c.Irreversible), c.Irreversible)
	}
}

// E1 (R3): the commit checkpoints of the writing workflows are verified by
// oh (build, tests, type check of the stack), expecting a pass.
func TestShippedWorkflowsVerifyCommit(t *testing.T) {
	_, cat, _ := shippedWorkflows(t)
	want := map[string]string{"ticket": "cp-2", "implement-spec": "cp-2", "review-feedback": "cp-2", "quick": "cp-commit", "debug": "cp-commit", "sweep": "cp-commit"}
	for id, cpID := range want {
		r, diags := workflow.ResolveSpec(cat, workflow.Ref{Layer: workflow.LayerHub, ID: id}, nil)
		require.Empty(t, diags.Errors(), id)
		cp, ok := r.Spec.Checkpoints.Get(cpID)
		require.True(t, ok, "%s.%s", id, cpID)
		require.NotNil(t, cp.Verify, "%s.%s has no verify:", id, cpID)
		assert.Equal(t, workflow.VerifyFromStack, cp.Verify.From, id)
		assert.Equal(t, workflow.VerifyPass, cp.Verify.ExpectOrPass(), id)
		rt := WorkflowRuntime(r.Spec)
		def, ok := rt.Checkpoint(cpID)
		require.True(t, ok)
		require.NotNil(t, def.Verify, "%s: verify reaches the session runtime", id)
		assert.Equal(t, []string{"build", "test", "typecheck"}, def.Verify.Kinds)
		assert.False(t, def.Verify.Ask, "%s writes: the verification is not asked", id)
	}
}

// CP-R10: the pauses planned in each mode; auto keeps the mandatory ones.
func TestPlannedPauses(t *testing.T) {
	_, cat, _ := shippedWorkflows(t)
	r, diags := workflow.ResolveSpec(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "implement-spec"}, nil)
	require.Empty(t, diags.Errors())
	sp := r.Spec.ForFanout(false)
	rt := WorkflowRuntime(sp.ForMethod(workflow.MethodStandard))
	ids := func(mode string) []string {
		var out []string
		for _, p := range PlannedPauses(rt, mode) {
			out = append(out, p.ID)
		}
		return out
	}
	assert.Equal(t, []string{"cp-plan", "cp-2", "cp-final"}, ids("semi-auto"))
	assert.Equal(t, []string{"cp-plan", "cp-2"}, ids("auto"), "auto = everything but the mandatory decisions")
	assert.Len(t, PlannedPauses(rt, "manuel"), 5)
	rt = WorkflowRuntime(sp.ForMethod(workflow.MethodTDD))
	assert.Equal(t, []string{"cp-plan", "cp-seams", "cp-2", "cp-final"}, ids("semi-auto"), "R7: TDD adds cp-seams, cp-red passes on its own")
	assert.Equal(t, []string{"cp-plan", "cp-2"}, ids("auto"))
	p := PlannedPauses(rt, "auto")
	assert.True(t, p[0].Mandatory)
	// No shipped workflow has a conditional checkpoint any more (cp-spec
	// left with feature, feature-migration).
	cond := &sessionspec.WorkflowRuntime{DefaultMode: "auto", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-ui", Behaviors: map[string]string{"auto": sessionspec.CheckpointConditional}, Condition: "une spec UI"}}}
	require.Len(t, PlannedPauses(cond, ""), 1)
	assert.True(t, PlannedPauses(cond, "")[0].Conditional)
	assert.Nil(t, PlannedPauses(nil, "auto"))
}

// R3: conditions make the tool ask (oh evaluates them); a read-only
// workflow asks before verifying; the developer lock past the fix rounds.
func TestSessionRulesR3(t *testing.T) {
	rt := &sessionspec.WorkflowRuntime{ID: "x", DefaultMode: "auto", MaxFixRounds: 2, Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-1", Behaviors: map[string]string{"auto": "auto"}, When: &sessionspec.WhenDef{Labels: []string{"ux"}}},
	}}
	rules := SessionRules(rt, "auto", nil)
	assert.Equal(t, sessionspec.EffectAsk, rules[0].Effect)
	rules = SessionRules(rt, "auto", &domain.CheckpointState{Unresolved: "pt-1"})
	assert.Contains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "developer*", Effect: sessionspec.EffectDeny})
	rt.MaxFixRounds = 0
	rules = SessionRules(rt, "auto", &domain.CheckpointState{Unresolved: "pt-1"})
	assert.NotContains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "developer*", Effect: sessionspec.EffectDeny})

	spec := &workflow.Spec{ID: "audit", Risk: workflow.RiskRead, Checkpoints: workflow.OrderedMap[workflow.CheckpointSpec]{}}
	spec.Checkpoints.Set("cp-1", workflow.CheckpointSpec{Mode: map[string]workflow.CheckpointBehavior{"auto": "auto"},
		Verify: &workflow.VerifySpec{Cmd: []string{"make check"}}})
	def, _ := WorkflowRuntime(spec).Checkpoint("cp-1")
	require.NotNil(t, def.Verify)
	assert.True(t, def.Verify.Ask)
	assert.Equal(t, sessionspec.CheckpointPause, def.Behavior("auto"), "risk: read: asked before the commands run")
	assert.Empty(t, def.Verify.Kinds, "no kinds without from: stack")
	assert.Equal(t, 600, def.Verify.TimeoutSec)
}
