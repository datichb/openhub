package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// tddWorkflow is a ticket workflow with the TDD checkpoints (hub agents).
const tddWorkflow = `apiVersion: oh/v1
kind: Workflow
id: tdd-test
risk: write
entry: { agent: orchestrator-dev }
inputs:
  ticket: { type: beads-id, required: true }
  method: { type: enum, values: [standard, tdd], default: standard }
prompt: { text: "Ticket {{ .ticket }}" }
agents:
  orchestrator-dev: { role: workflow }
  developer: { role: workflow, after: cp-1 }
  reviewer: { role: workflow, mode: subagent }
checkpoints:
  cp-1:
    by: [orchestrator-dev]
    label: Démarrer le ticket
    mode: { manuel: pause, semi-auto: auto, auto: auto }
  cp-seams:
    by: [developer]
    method: tdd
    label: Points d'injection et tests prévus
    description: Présenter les points d'injection et les tests prévus.
    mode: { manuel: pause, semi-auto: pause, auto: auto }
  cp-red:
    by: [developer]
    method: tdd
    label: Tests en échec
    description: Méthode TDD, tests écrits et avant le code.
    mode: { manuel: auto, semi-auto: auto, auto: auto }
    verify: { from: stack, expect: fail, must_touch_tests: true, outside_tests: warn, since: cp-seams }
  cp-2:
    by: [orchestrator-dev]
    label: Commit ou correction
    mandatory: true
    unlocks: [commit, close]
    mode: { manuel: pause, semi-auto: pause, auto: pause }
    verify: { from: stack, expect: pass }
  cp-3:
    by: [orchestrator-dev]
    label: Ticket suivant
    mode: { manuel: pause, semi-auto: auto, auto: auto }
`

// R7: the bundle of a TDD session holds cp-seams and cp-red, passed by the
// developer, with what oh checks; the bundle of another session is the one
// of a workflow without them.
func TestMethodBundle(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	doc, pd := workflow.Parse([]byte(tddWorkflow), workflow.Source{Layer: workflow.LayerHub, Path: "tdd-test.yaml"})
	require.False(t, pd.HasErrors(), "%v", pd)
	require.Empty(t, cat.Add(doc))
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "tdd-test"}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)

	std, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
	require.NoError(t, err)
	same, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, Method: workflow.MethodStandard})
	require.NoError(t, err)
	assert.Equal(t, std.Spec.Hash, same.Spec.Hash, "standard = no method")
	_, ok := std.Spec.Workflow.Checkpoint("cp-seams")
	assert.False(t, ok, "no method checkpoint outside the method")
	assert.Empty(t, std.Spec.Workflow.Method)
	assert.NotContains(t, findAgent(std.Spec.Agents, "developer").Body, "cp-red")

	tdd, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, Method: workflow.MethodTDD})
	require.NoError(t, err)
	assert.NotEqual(t, std.Spec.Hash, tdd.Spec.Hash)
	wf := tdd.Spec.Workflow
	assert.Equal(t, "tdd", wf.Method)
	var ids []string
	for _, c := range wf.Checkpoints {
		ids = append(ids, c.ID)
	}
	assert.Equal(t, []string{"cp-1", "cp-seams", "cp-red", "cp-2", "cp-3"}, ids)
	seams, _ := wf.Checkpoint("cp-seams")
	assert.True(t, seams.Base, "the commit is recorded at cp-seams")
	assert.Equal(t, []string{"developer"}, seams.By)
	red, _ := wf.Checkpoint("cp-red")
	require.NotNil(t, red.Verify)
	assert.Equal(t, sessionspec.VerifyDef{From: "stack", Kinds: []string{"build", "test", "typecheck"}, Expect: "fail", TimeoutSec: 600,
		MustTouchTests: true, OutsideTests: "warn", Since: "cp-seams"}, *red.Verify)
	assert.Equal(t, "auto", red.Behavior("semi-auto"))
	assert.Equal(t, "pause", seams.Behavior("semi-auto"))

	dev := findAgent(tdd.Spec.Agents, "developer").Body
	assert.Contains(t, dev, "**Méthode de la session** : `tdd`")
	assert.Contains(t, dev, "`cp-seams` (`developer`) → `cp-red` (`developer`)")
	assert.Contains(t, dev, "- **Checkpoints** : tu passes `cp-seams`")
	assert.Contains(t, dev, "  - `cp-red` : Méthode TDD")
	assert.Contains(t, dev, "oh lance lui-même la vérification avant de le laisser passer : les tests doivent échouer")
	lead := findAgent(tdd.Spec.Agents, "orchestrator-dev").Body
	assert.Contains(t, lead, "**Méthode de la session** : `tdd`", "the coordinator knows the method")
	assert.NotContains(t, lead, "  - `cp-red` :", "the details go to the agent that passes it")
	assert.NotContains(t, lead, "tu passes `cp-1` (Démarrer le ticket), `cp-seams`", "its own checkpoints only")

	// A method without checkpoint in the workflow changes nothing.
	q, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "quick"}, nil, env)
	require.False(t, diags.HasErrors())
	a, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: q.Spec})
	require.NoError(t, err)
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: q.Spec, Method: workflow.MethodTDD})
	require.NoError(t, err)
	assert.Equal(t, a.Spec.Hash, b.Spec.Hash)
}

// R7: the text of the workflow map on a checkpoint verified for a test
// written first; nothing for the other verifications.
func TestTestCheckText(t *testing.T) {
	assert.Empty(t, testCheckText(nil))
	assert.Empty(t, testCheckText(&workflow.VerifySpec{From: workflow.VerifyFromStack}))
	txt := testCheckText(&workflow.VerifySpec{From: workflow.VerifyFromStack, Expect: workflow.VerifyFail, MustTouchTests: true,
		OutsideTests: workflow.OutsideTestsRefuse, Since: "cp-seams"})
	assert.Contains(t, txt, "les tests doivent échouer")
	assert.Contains(t, txt, "depuis **cp-seams**")
	assert.Contains(t, txt, "et aucun autre fichier")
	txt = testCheckText(&workflow.VerifySpec{Cmd: []string{"make t"}, MustTouchTests: true})
	assert.NotContains(t, txt, "échouer")
	assert.Contains(t, txt, "au moins un fichier de test")
}
