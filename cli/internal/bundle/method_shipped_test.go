package bundle

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// R7: ticket and implement-spec offer the tdd method: cp-seams then cp-red, passed
// by the developer after cp-1 and before cp-2, cp-red verified by oh for a
// test written first; a standard session has neither.
func TestShippedTDD(t *testing.T) {
	_, cat, env := shippedWorkflows(t)
	for _, id := range []string{"ticket", "implement-spec"} {
		r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: id}, nil, env)
		require.False(t, diags.HasErrors(), "%v", diags)
		in, ok := r.Spec.Inputs.Get(workflow.MethodInput)
		require.True(t, ok, id)
		assert.Equal(t, []string{"standard", "tdd"}, in.Values, id)
		assert.Equal(t, workflow.MethodStandard, r.Spec.MethodOf(nil), "%s: standard by default", id)

		keys := r.Spec.Checkpoints.Keys()
		assert.Less(t, indexOf(keys, "cp-1"), indexOf(keys, "cp-seams"), id)
		assert.Less(t, indexOf(keys, "cp-seams"), indexOf(keys, "cp-red"), id)
		assert.Less(t, indexOf(keys, "cp-red"), indexOf(keys, "cp-2"), id)
		red, _ := r.Spec.Checkpoints.Get("cp-red")
		require.NotNil(t, red.Verify, id)
		assert.Equal(t, workflow.VerifyFail, red.Verify.ExpectOrPass(), id)
		assert.True(t, red.Verify.MustTouchTests, id)
		assert.Equal(t, "cp-seams", red.Verify.Since, id)

		ref := workflow.Ref{Layer: workflow.LayerHub, ID: id}
		_, b := shippedBuildWith(t, ref, shippedOpts{Method: workflow.MethodTDD})
		seams, ok := b.Spec.Workflow.Checkpoint("cp-seams")
		require.True(t, ok, id)
		assert.Equal(t, []string{"developer"}, seams.By, id)
		assert.True(t, seams.Base, id)
		_, std := shippedBuild(t, "", ref)
		_, ok = std.Spec.Workflow.Checkpoint("cp-red")
		assert.False(t, ok, "%s: no TDD checkpoint in a standard session", id)
	}
}

// feature-migration: spec takes the method of the implementation without
// using it (no TDD checkpoint), records it (record: input) and passes it on
// to implement-spec (next:); feature routes to both (aliases).
func TestShippedSpecPassesMethodOn(t *testing.T) {
	t.Parallel()
	_, cat, env := shippedWorkflows(t)
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "spec"}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	in, ok := r.Spec.Inputs.Get(workflow.MethodInput)
	require.True(t, ok)
	assert.Equal(t, []string{"standard", "tdd"}, in.Values)
	assert.False(t, r.Spec.HasMethod(workflow.MethodTDD), "no TDD checkpoint in spec")
	out, ok := r.Spec.Output(workflow.MethodInput)
	require.True(t, ok)
	assert.Equal(t, workflow.RecordInput, out.Record)
	assert.Equal(t, map[string]any{"method": "tdd"}, r.Spec.RecordedInputs(map[string]any{"method": "tdd"}))
	require.Len(t, r.Spec.Next, 1)
	assert.Equal(t, "session.output(method)", r.Spec.Next[0].Inputs["method"])

	_, exists := cat.Top("feature")
	assert.False(t, exists, "feature.yaml removed")
	ref, ok := cat.ResolveAlias("feature", nil)
	require.True(t, ok)
	assert.Equal(t, "spec", ref.ID)
	ref, ok = cat.ResolveAlias("feature", func(input string) bool { return input == "tickets" })
	require.True(t, ok)
	assert.Equal(t, "implement-spec", ref.ID)
	ref, _ = cat.ResolveAlias("cadrage", func(string) bool { return true })
	assert.Equal(t, "spec", ref.ID)
	// tui-docs: terms of the former omnibar table kept as aliases.
	for alias, want := range map[string]string{"secu": "security-fix", "perf": "benchmark", "dbg": "debug", "feedback": "review-feedback", "retours": "review-feedback"} {
		ref, ok = cat.ResolveAlias(alias, nil)
		if assert.True(t, ok, alias) {
			assert.Equal(t, want, ref.ID, alias)
		}
	}
	assert.Empty(t, workflow.CheckAliases(cat), "no ambiguous alias")
}

// B1: spike records its decision and the verdict of the decision line its
// prompt asks for; incident its postmortem; the session runtime carries the
// paths and the expression.
func TestShippedWrittenOutputs(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	r, b := shippedBuild(t, "", workflow.Ref{Layer: workflow.LayerHub, ID: "spike"})
	var verdict sessionspec.OutputDef
	for _, o := range b.Spec.Workflow.Outputs {
		if o.ID == "verdict" {
			verdict = o
		}
	}
	require.Equal(t, "written", verdict.Record)
	assert.Equal(t, []string{"docs/wiki/decisions/**"}, verdict.Paths)
	re := regexp.MustCompile(verdict.Match)
	for line, want := range map[string]string{"**Décision : go**": "go", "**Décision : Kill**": "Kill", "**Decision: clarify** (critère 2)": "clarify"} {
		m := re.FindStringSubmatch("# Spike — x\n\n" + line + "\n")
		require.NotNil(t, m, line)
		assert.Equal(t, want, m[1])
	}
	hc, err := hubcat.New(hub)
	require.NoError(t, err)
	src, err := r.ReadPromptSource(hc)
	require.NoError(t, err)
	assert.Contains(t, string(src), "**Décision : go | clarify | kill**", "the line the verdict is read from")

	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "incident"}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	pm, ok := r.Spec.Output("postmortem")
	require.True(t, ok)
	assert.Equal(t, workflow.RecordWritten, pm.Record)
	assert.Equal(t, r.Spec.WritesOf("debugger"), pm.Paths, "the paths the debugger writes")
}

func indexOf(keys []string, k string) int {
	for i, x := range keys {
		if x == k {
			return i
		}
	}
	return -1
}
