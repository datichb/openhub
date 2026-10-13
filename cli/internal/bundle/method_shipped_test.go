package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// R7: ticket and feature offer the tdd method: cp-seams then cp-red, passed
// by the developer after cp-1 and before cp-2, cp-red verified by oh for a
// test written first; a standard session has neither.
func TestShippedTDD(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	for _, id := range []string{"ticket", "feature"} {
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

		b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, Method: workflow.MethodTDD})
		require.NoError(t, err)
		seams, ok := b.Spec.Workflow.Checkpoint("cp-seams")
		require.True(t, ok, id)
		assert.Equal(t, []string{"developer"}, seams.By, id)
		assert.True(t, seams.Base, id)
		std, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
		require.NoError(t, err)
		_, ok = std.Spec.Workflow.Checkpoint("cp-red")
		assert.False(t, ok, "%s: no TDD checkpoint in a standard session", id)
	}
}

func indexOf(keys []string, k string) int {
	for i, x := range keys {
		if x == k {
			return i
		}
	}
	return -1
}
