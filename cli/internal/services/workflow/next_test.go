package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	wf "github.com/datichb/openhub/cli/internal/workflow"
)

// R5 on the shipped hub: spec proposes implement-spec with its recorded
// outputs, cadrage is an alias of spec, implement-spec reads the tickets of
// the last spec session.

func shippedService(t *testing.T) *Service {
	return &Service{HubDir: repoHub(t), Lang: "fr", Isolation: sessionspec.IsolationFull}
}

func TestNextStepsOfSpec(t *testing.T) {
	svc := shippedService(t)
	ctx := context.Background()
	outputs := map[string]any{"tickets": []any{"ev-1", "ev-2"}, "spec": "docs/wiki/specs/export.md"}
	steps, err := svc.NextSteps(ctx, Context{}, "spec", outputs)
	require.NoError(t, err)
	require.Len(t, steps, 1)
	s := steps[0]
	assert.Equal(t, "implement-spec", s.WorkflowID)
	assert.True(t, s.Declared)
	assert.Equal(t, []string{"ev-1", "ev-2"}, s.Tickets)
	assert.Equal(t, map[string]string{"spec": "docs/wiki/specs/export.md"}, s.Prefill)

	steps, err = svc.NextSteps(ctx, Context{}, "spec", map[string]any{"spec": "docs/wiki/specs/export.md"})
	require.NoError(t, err)
	assert.Empty(t, steps, "no ticket created: nothing to implement")

	// « Enchaîner avec… » lists the declared step first, once.
	sg, err := svc.Chain(ctx, Context{}, "spec", outputs, nil)
	require.NoError(t, err)
	require.NotEmpty(t, sg)
	assert.Equal(t, "implement-spec", sg[0].WorkflowID)
	assert.True(t, sg[0].Declared)
	for _, x := range sg[1:] {
		assert.NotEqual(t, "implement-spec", x.WorkflowID)
		assert.False(t, x.Declared)
	}
}

func TestCadrageIsAnAliasOfSpec(t *testing.T) {
	svc := shippedService(t)
	ctx := context.Background()
	target, ok := svc.ResolveAlias(ctx, Context{}, "cadrage", nil)
	require.True(t, ok)
	assert.Equal(t, "spec", target)
	res, err := svc.Resolve(ctx, Context{}, "cadrage", ResolveOpts{})
	require.NoError(t, err)
	assert.Equal(t, "spec", res.Spec.ID)
	assert.True(t, svc.Has(ctx, Context{}, "cadrage"))
	_, ok = svc.ResolveAlias(ctx, Context{}, "spec", nil)
	assert.False(t, ok, "a workflow is not an alias")
}

func TestComputeInputsFromSession(t *testing.T) {
	svc := shippedService(t)
	ctx := context.Background()
	res, err := svc.Resolve(ctx, Context{}, "implement-spec", ResolveOpts{})
	require.NoError(t, err)
	var calls []string
	svc.InputSources = map[string]InputSource{
		wf.SourceSessionOutput: func(_ context.Context, c Context, arg string) (string, error) {
			calls = append(calls, arg+"@"+c.FromSession)
			if arg == "spec.tickets" {
				return "ev-1,ev-2", nil
			}
			return "", nil
		},
	}
	values := map[string]any{}
	require.NoError(t, svc.ComputeInputs(ctx, Context{ProjectID: "p", FromSession: "ses_1"}, res.Spec, values))
	assert.Equal(t, map[string]any{"tickets": "ev-1,ev-2"}, values)
	assert.ElementsMatch(t, []string{"spec.tickets@ses_1", "spec.spec@ses_1"}, calls)

	// Given tickets win; no source on this machine leaves the input empty.
	values = map[string]any{"tickets": "ev-9"}
	calls = nil
	require.NoError(t, svc.ComputeInputs(ctx, Context{}, res.Spec, values))
	assert.Equal(t, "ev-9", values["tickets"])
	assert.Equal(t, []string{"spec.spec@"}, calls)
	values = map[string]any{}
	require.NoError(t, (&Service{}).ComputeInputs(ctx, Context{}, res.Spec, values))
	assert.Empty(t, values)
}
