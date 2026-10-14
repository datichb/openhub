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

// B1: a security audit proposes security-fix, which reads the report from
// the audit session (not prefilled: its content, not its id); another audit
// proposes nothing.
func TestNextStepsOfAudit(t *testing.T) {
	svc := shippedService(t)
	ctx := context.Background()
	steps, err := svc.NextSteps(ctx, Context{}, "audit", map[string]any{"findings": "rp-2", "type": "security"})
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "security-fix", steps[0].WorkflowID)
	assert.Empty(t, steps[0].Prefill, "the findings are read from the parent session")
	assert.Equal(t, []wf.OutputType{wf.OutputReport}, steps[0].Matched)

	steps, err = svc.NextSteps(ctx, Context{}, "audit", map[string]any{"findings": "rp-2", "type": "performance"})
	require.NoError(t, err)
	assert.Empty(t, steps)
	steps, err = svc.NextSteps(ctx, Context{}, "audit", map[string]any{"type": "security"})
	require.NoError(t, err)
	assert.Empty(t, steps, "no report deposited")

	res, err := svc.Resolve(ctx, Context{}, "security-fix", ResolveOpts{})
	require.NoError(t, err)
	in, _ := res.Spec.Inputs.Get("findings")
	assert.Equal(t, "session.output(audit.findings)", in.From)
}

// B1: a spike decided go proposes spec with the decision; clarify or kill,
// nothing. An incident proposes ticket on its action tickets and spec on its
// postmortem.
func TestNextStepsOfSpikeAndIncident(t *testing.T) {
	svc := shippedService(t)
	ctx := context.Background()
	dec := "docs/wiki/decisions/cache.md"
	steps, err := svc.NextSteps(ctx, Context{}, "spike", map[string]any{"decision": dec, "verdict": "go"})
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "spec", steps[0].WorkflowID)
	assert.Equal(t, map[string]string{"source": dec}, steps[0].Prefill)
	for _, v := range []string{"clarify", "kill", ""} {
		steps, err = svc.NextSteps(ctx, Context{}, "spike", map[string]any{"decision": dec, "verdict": v})
		require.NoError(t, err)
		assert.Empty(t, steps, v)
	}

	pm := "docs/wiki/postmortems/2026-10-14-panne.md"
	steps, err = svc.NextSteps(ctx, Context{}, "incident", map[string]any{"postmortem": pm, "actions": []any{"ev-4", "ev-5"}})
	require.NoError(t, err)
	require.Len(t, steps, 2)
	assert.Equal(t, "ticket", steps[0].WorkflowID)
	assert.Equal(t, []string{"ev-4", "ev-5"}, steps[0].Tickets)
	assert.Equal(t, "spec", steps[1].WorkflowID)
	assert.Equal(t, map[string]string{"source": pm}, steps[1].Prefill)
	steps, err = svc.NextSteps(ctx, Context{}, "incident", map[string]any{"postmortem": pm})
	require.NoError(t, err)
	require.Len(t, steps, 1, "no action ticket: spec only")
	assert.Equal(t, "spec", steps[0].WorkflowID)
}

// r9b: retro proposes triage on its action tickets, triage proposes ticket
// on the tickets it created; nothing without them.
func TestNextStepsOfRetroAndTriage(t *testing.T) {
	svc := shippedService(t)
	ctx := context.Background()
	steps, err := svc.NextSteps(ctx, Context{}, "retro", map[string]any{"retro": "docs/wiki/retros/2026-10-14-semaine.md", "actions": []any{"ev-7", "ev-8"}})
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "triage", steps[0].WorkflowID)
	assert.Equal(t, []string{"ev-7", "ev-8"}, steps[0].Tickets)
	steps, err = svc.NextSteps(ctx, Context{}, "retro", map[string]any{"retro": "docs/wiki/retros/2026-10-14-semaine.md"})
	require.NoError(t, err)
	assert.Empty(t, steps)

	steps, err = svc.NextSteps(ctx, Context{}, "triage", map[string]any{"updated": []any{"ev-1"}, "created": []any{"ev-9"}})
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "ticket", steps[0].WorkflowID)
	assert.Equal(t, []string{"ev-9"}, steps[0].Tickets)
	steps, err = svc.NextSteps(ctx, Context{}, "triage", map[string]any{"updated": []any{"ev-1"}})
	require.NoError(t, err)
	assert.Empty(t, steps)
}

// r9b: the trace summaries of retro are computed from the sessions input
// (session.traces), with the workflow launched in the context; a summary
// given at launch wins.
func TestComputeInputsOfRetro(t *testing.T) {
	svc := shippedService(t)
	ctx := context.Background()
	res, err := svc.Resolve(ctx, Context{}, "retro", ResolveOpts{})
	require.NoError(t, err)
	var calls []string
	svc.InputSources = map[string]InputSource{
		wf.SourceSessionTraces: func(_ context.Context, c Context, arg string) (string, error) {
			calls = append(calls, arg+"@"+c.Workflow)
			return "=== Session 1/1 ===", nil
		},
	}
	values := map[string]any{"sessions": "ticket 7d"}
	require.NoError(t, svc.ComputeInputs(ctx, Context{ProjectID: "p"}, res.Spec, values))
	assert.Equal(t, "=== Session 1/1 ===", values["trace"])
	assert.Equal(t, []string{"ticket 7d@retro"}, calls)

	values = map[string]any{"trace": "rejouée"}
	calls = nil
	require.NoError(t, svc.ComputeInputs(ctx, Context{ProjectID: "p"}, res.Spec, values))
	assert.Equal(t, "rejouée", values["trace"])
	assert.Empty(t, calls)
}
