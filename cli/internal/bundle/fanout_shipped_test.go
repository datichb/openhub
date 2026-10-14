package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// R6: implement-spec launched with fanout=true is a fan-out session: the
// checkpoints of the children are gone, cp-merge substitutes their cp-2
// (CP-R3, declared by the hub layer), the developer is not in the bundle.
// Without the input, the session keeps exactly the checkpoints of before.
func TestShippedImplementSpecFanout(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	ref := workflow.Ref{Layer: workflow.LayerHub, ID: "implement-spec"}

	r, diags := workflow.Check(cat, ref, &workflow.SessionOptions{Inputs: map[string]any{"tickets": []string{"ev-1"}, "fanout": true}}, env)
	require.False(t, diags.HasErrors(), "%v", diags.Errors())
	require.True(t, r.Fanout)
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, Fanout: true, Provider: "bedrock"})
	require.NoError(t, err)
	wf := b.Spec.Workflow
	var ids []string
	for _, c := range wf.Checkpoints {
		ids = append(ids, c.ID)
	}
	assert.Equal(t, []string{"cp-plan", "cp-merge", "cp-final"}, ids)
	require.NotNil(t, wf.Fanout)
	assert.Equal(t, sessionspec.FanoutDef{Child: "ticket", Tickets: "tickets", Parallel: 3, Merge: "cp-merge", Delegates: []string{"cp-2"}, Merger: "merge-conflicts",
		Fix: "quick", FixInput: "request"}, *wf.Fanout)
	assert.Equal(t, map[string]string{"method": "tdd", "instructions": "x"}, r.Spec.ChildInputs(map[string]any{"method": "tdd", "instructions": "x", "spec": "s.md"}),
		"the children get the method and the instructions, nothing else")
	assert.True(t, wf.Substitutes("cp-2"), "cp-merge replaces the cp-2 of the children")
	assert.False(t, wf.Substitutes("cp-1"))
	assert.False(t, b.Spec.HasAgent("developer"), "the children implement")
	for _, c := range wf.Checkpoints {
		assert.NotContains(t, c.Unlocks, sessionspec.UnlockCommit, "the parent never commits: %s", c.ID)
	}

	std, diags := workflow.Check(cat, ref, &workflow.SessionOptions{Inputs: map[string]any{"tickets": []string{"ev-1"}}}, env)
	require.False(t, diags.HasErrors())
	sb, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: std.Spec, Provider: "bedrock"})
	require.NoError(t, err)
	assert.Nil(t, sb.Spec.Workflow.Fanout, "a standard session has no fan-out")
	ids = nil
	for _, c := range sb.Spec.Workflow.Checkpoints {
		ids = append(ids, c.ID)
	}
	assert.Equal(t, []string{"cp-plan", "cp-1", "cp-2", "cp-3", "cp-final"}, ids)

	// The child workflow holds the delegated checkpoint, with its own
	// verification (still run by oh in the children).
	child, _ := workflow.ResolveSpec(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "ticket"}, nil)
	cp2, ok := child.Spec.Checkpoints.Get("cp-2")
	require.True(t, ok)
	assert.NotNil(t, cp2.Verify)
}

// CP-R3: a team layer that patches the shipped implement-spec cannot loosen
// its fan-out (attempts to get around it on the shipped workflow).
func TestShippedImplementSpecFanoutTeamLayer(t *testing.T) {
	patches := map[string]string{
		"fanout.delegates":                              "fanout: { delegates: [cp-2, cp-1] }\n",
		"fanout.merge":                                  "fanout: { merge: cp-final }\n",
		"checkpoints.cp-merge.mandatory":                "checkpoints:\n  cp-merge: { mandatory: false }\n",
		"checkpoints.cp-merge.verify":                   "checkpoints:\n  cp-merge: { verify: { from: stack, expect: pass } }\n",
		"checkpoints.cp-merge.mode.auto":                "checkpoints:\n  cp-merge: { mode: { auto: auto } }\n",
		"checkpoints.cp-2.fanout":                       "checkpoints:\n  cp-2: { fanout: only }\n",
		"fanout.child":                                  "fanout: { child: quick }\n",
		"checkpoints.cp-merge.disabled":                 "checkpoints:\n  cp-merge: { disabled: true }\n",
		"fanout.parallel":                               "fanout: { parallel: 0 }\n",
		"checkpoints.cp-merge.fanout":                   "checkpoints:\n  cp-merge: { fanout: never }\n",
		"checkpoints.cp-final.unlocks[0]":               "checkpoints:\n  cp-final: { unlocks: [merge] }\n",
		"checkpoints.cp-merge.verify (shorter timeout)": "checkpoints:\n  cp-merge: { verify: { from: stack, expect: pass, on: integration, timeout: 1m } }\n",
		"fanout.fix":                                    "fanout: { fix: { workflow: libre, input: request } }\n",
		"fanout.inputs":                                 "fanout: { inputs: [method, instructions, spec] }\n",
	}
	for path, body := range patches {
		t.Run(path, func(t *testing.T) {
			_, cat, env := shippedWorkflows(t)
			doc, pd := workflow.Parse([]byte("apiVersion: oh/v1\nkind: Workflow\nid: implement-spec\nextends: hub:implement-spec\n"+body),
				workflow.Source{Layer: workflow.LayerTeam, Path: "team/implement-spec.yaml"})
			require.False(t, pd.HasErrors(), "%v", pd)
			require.Empty(t, cat.Add(doc))
			_, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerTeam, ID: "implement-spec"}, nil, env)
			assert.True(t, diags.HasErrors(), "%s is refused: %v", path, diags)
		})
	}
}
