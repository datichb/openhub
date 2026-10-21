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
	t.Parallel()
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
	t.Parallel()
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

// sweep-fanout: sweep launched with fanout=true is a fan-out on the tasks oh
// recorded at cp-plan: one sweep-task child each, their cp-commit replaced
// by cp-merge (CP-R3), no developer and no commit in the parent. Without
// the input, sweep keeps its sequential version (developer, cp-commit),
// cp-plan reading the breakdown in both.
func TestShippedSweepFanout(t *testing.T) {
	t.Parallel()
	hub, cat, env := shippedWorkflows(t)
	ref := workflow.Ref{Layer: workflow.LayerHub, ID: "sweep"}

	r, diags := workflow.Check(cat, ref, &workflow.SessionOptions{Inputs: map[string]any{"goal": "g", "fanout": true}}, env)
	require.False(t, diags.HasErrors(), "%v", diags.Errors())
	require.True(t, r.Fanout)
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, Fanout: true, Provider: "bedrock"})
	require.NoError(t, err)
	wf := b.Spec.Workflow
	var ids []string
	for _, c := range wf.Checkpoints {
		ids = append(ids, c.ID)
	}
	assert.Equal(t, []string{"cp-plan", "cp-merge", "cp-recap"}, ids)
	require.NotNil(t, wf.Fanout)
	assert.Equal(t, sessionspec.FanoutDef{Child: "sweep-task", Source: sessionspec.FanoutSourceTasks, Parallel: 3, Merge: "cp-merge", Delegates: []string{"cp-commit"},
		Merger: "merge-conflicts", Fix: "quick", FixInput: "request"}, *wf.Fanout)
	assert.True(t, wf.Substitutes("cp-commit"), "cp-merge replaces the cp-commit of the children")
	assert.False(t, b.Spec.HasAgent("developer"), "the children do the subtasks")
	assert.Equal(t, "cp-plan", wf.TasksCheckpoint())
	plan, _ := wf.Checkpoint("cp-plan")
	require.NotNil(t, plan.Tasks)
	assert.Equal(t, 12, plan.Tasks.Max)
	for _, c := range wf.Checkpoints {
		assert.NotContains(t, c.Unlocks, sessionspec.UnlockCommit, "the parent never commits: %s", c.ID)
	}
	assert.Equal(t, map[string]string{"goal": "g", "verify": "tests"}, r.Spec.ChildInputs(map[string]any{"goal": "g", "verify": "tests", "include": "x/**"}))

	std, diags := workflow.Check(cat, ref, &workflow.SessionOptions{Inputs: map[string]any{"goal": "g"}}, env)
	require.False(t, diags.HasErrors())
	sb, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: std.Spec, Provider: "bedrock"})
	require.NoError(t, err)
	assert.Nil(t, sb.Spec.Workflow.Fanout, "the sequential sweep has no fan-out")
	assert.True(t, sb.Spec.HasAgent("developer"))
	ids = nil
	for _, c := range sb.Spec.Workflow.Checkpoints {
		ids = append(ids, c.ID)
	}
	assert.Equal(t, []string{"cp-plan", "cp-commit", "cp-recap"}, ids)
	assert.Equal(t, "cp-plan", sb.Spec.Workflow.TasksCheckpoint(), "the breakdown is read in both versions")

	child, _ := workflow.ResolveSpec(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "sweep-task"}, nil)
	cc, ok := child.Spec.Checkpoints.Get("cp-commit")
	require.True(t, ok)
	assert.NotNil(t, cc.Verify, "the children keep their verification")
	assert.Equal(t, []string{"show", "list", "search", "query"}, child.Spec.Beads.Allow, "no ticket: Beads in read only")
}

// CP-R3 on the shipped sweep: a team layer cannot loosen its fan-out nor
// its breakdown; a team layer of before (cp-commit hardened, developer
// model) stays valid, warned that its cp-commit does not reach a fan-out.
func TestShippedSweepFanoutTeamLayer(t *testing.T) {
	patches := map[string]string{
		"fanout.source":                 "fanout: { source: tickets }\n",
		"fanout.delegates":              "fanout: { delegates: [cp-commit, cp-recap] }\n",
		"fanout.child":                  "fanout: { child: quick }\n",
		"checkpoints.cp-merge.verify":   "checkpoints:\n  cp-merge: { verify: { from: stack, expect: pass } }\n",
		"checkpoints.cp-merge.disabled": "checkpoints:\n  cp-merge: { disabled: true }\n",
		"checkpoints.cp-plan.tasks":     "checkpoints:\n  cp-plan: { tasks: null }\n",
		"checkpoints.cp-plan.tasks.max": "checkpoints:\n  cp-plan: { tasks: { max: 20 } }\n",
		"agents.developer.fanout":       "agents:\n  developer: { fanout: only }\n",
		"checkpoints.cp-commit.fanout":  "checkpoints:\n  cp-commit: { fanout: only }\n",
		"fanout.inputs":                 "fanout: { inputs: [goal, verify, verify_cmd, include] }\n",
	}
	team := func(t *testing.T, body string) workflow.Diagnostics {
		_, cat, env := shippedWorkflows(t)
		doc, pd := workflow.Parse([]byte("apiVersion: oh/v1\nkind: Workflow\nid: sweep\nextends: hub:sweep\n"+body),
			workflow.Source{Layer: workflow.LayerTeam, Path: "team/sweep.yaml"})
		require.False(t, pd.HasErrors(), "%v", pd)
		require.Empty(t, cat.Add(doc))
		_, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerTeam, ID: "sweep"}, nil, env)
		return diags
	}
	for path, body := range patches {
		t.Run(path, func(t *testing.T) {
			diags := team(t, body)
			assert.True(t, diags.HasErrors(), "%s is refused: %v", path, diags)
		})
	}
	t.Run("layer of before", func(t *testing.T) {
		diags := team(t, "checkpoints:\n  cp-commit: { verify: { from: stack, expect: pass, kinds: [build, test, typecheck, lint] } }\nmodels:\n  agents: { developer: bedrock/eu.anthropic.claude-haiku-4-5 }\n")
		require.False(t, diags.HasErrors(), "%v", diags.Errors())
		found := false
		for _, d := range diags {
			found = found || d.Code == "fanout_never_patched"
		}
		assert.True(t, found, "warned: %v", diags)
	})
}
