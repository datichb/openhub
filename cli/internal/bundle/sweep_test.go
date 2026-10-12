package bundle

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// N7 (DC11): the sweep workflow.
func TestSweepWorkflow(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	hc, err := hubcat.New(hub)
	require.NoError(t, err)
	ref := workflow.Ref{Layer: workflow.LayerHub, ID: "sweep"}
	render := func(inputs map[string]any) string {
		r, diags := workflow.ResolveSpec(cat, ref, &workflow.SessionOptions{Inputs: inputs})
		require.False(t, diags.HasErrors(), "%v", diags)
		out, err := workflow.RenderPrompt(r, hc, workflow.PromptContext{Project: "demo", Location: "/src/demo", Mode: "auto", Runtime: "local", Lang: "fr", Workflow: "sweep"})
		require.NoError(t, err)
		return out
	}
	dry := render(map[string]any{"goal": "x", "dry_run": "true"})
	assert.Contains(t, dry, "n'appelle pas `cp-plan`")
	assert.NotContains(t, dry, "3. `cp-plan`")
	assert.NotContains(t, dry, "`cp-commit`")
	wet := render(map[string]any{"goal": "x"})
	assert.Contains(t, wet, "3. `cp-plan`")
	assert.Contains(t, wet, "`cp-commit`")
	assert.Contains(t, wet, "`reviewer`")
	assert.Contains(t, wet, "disjoints")

	// verify: custom needs verify_cmd at launch.
	_, diags := workflow.ResolveSpec(cat, ref, &workflow.SessionOptions{Inputs: map[string]any{"goal": "x", "verify": "custom"}})
	assert.Contains(t, diags.Codes(), "verify_cmd_missing")
	_, diags = workflow.ResolveSpec(cat, ref, &workflow.SessionOptions{Inputs: map[string]any{"goal": "x", "verify": "custom", "verify_cmd": "make check"}})
	assert.NotContains(t, diags.Codes(), "verify_cmd_missing")

	// The runtime: cp-plan pauses in every mode, developers and reviewer
	// stay locked before it (auto included), pathfinder is free, commit is
	// locked until cp-commit.
	r, diags := workflow.Check(cat, ref, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
	require.NoError(t, err)
	wf := b.Spec.Workflow
	cp, ok := wf.Checkpoint("cp-plan")
	require.True(t, ok)
	for _, mode := range []string{"manuel", "semi-auto", "auto"} {
		assert.Equal(t, sessionspec.CheckpointPause, cp.Behavior(mode), mode)
	}
	locked := LockedAgents(wf, "auto", nil)
	for _, a := range []string{"developer", "reviewer"} {
		assert.Contains(t, locked, a)
	}
	assert.NotContains(t, locked, "pathfinder")
	assert.Equal(t, []string{sessionspec.UnlockCommit}, LockedOps(wf, "auto", nil))
	denied := false
	for _, rule := range SessionRules(wf, "auto", nil) {
		denied = denied || (rule.Action == sessionspec.ActionShell && strings.Contains(rule.Resource, "commit") && rule.Effect == sessionspec.EffectDeny)
	}
	assert.True(t, denied, "git commit refused before cp-commit")
}
