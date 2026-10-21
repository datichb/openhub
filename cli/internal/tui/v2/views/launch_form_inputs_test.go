package views

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/testutil/golden"
	"github.com/datichb/openhub/cli/internal/workflow"
)

const implementSpecTestSpec = `apiVersion: oh/v1
kind: Workflow
id: implement-spec
description: { fr: "Implémenter les tickets d'une spec l'un après l'autre" }
risk: write
aliases: [{ id: feature, with: tickets }]
inputs:
  tickets:
    type: beads-ids
    required: true
    from: session.output(spec.tickets)
    label: { fr: Tickets }
    help: { fr: "Tickets Beads à implémenter, dans une seule session" }
  spec:
    type: path
    from: session.output(spec.spec)
    label: { fr: Spec }
    help: { fr: "Spec des tickets dans le wiki (facultatif)" }
  method:
    type: enum
    values: [standard, tdd]
    default: standard
    label: { fr: Méthode }
    help: { fr: "tdd : tests écrits d'abord, vérifiés par oh" }
  fanout:
    type: bool
    default: false
    label: { fr: Fan-out }
  instructions:
    type: text
    label: { fr: Instructions }
    help: { fr: "Consignes données à chaque ticket (texte, facultatif)" }
  rounds: { type: int, label: { fr: Tours } }
modes: { default: semi-auto, allowed: [manuel, semi-auto] }
runtime: { default: local, allowed: [local, container] }
`

const libreTestSpec = `apiVersion: oh/v1
kind: Workflow
id: libre
description: { fr: "Session avec l'agent de ton choix" }
risk: write
entry: { agent: orchestrator, selectable: true }
inputs:
  request:
    type: text
    label: { fr: Demande }
    help: { fr: "Ce qu'il faut faire (facultatif)" }
runtime: { default: local, allowed: [local, container] }
`

func parseSpec(t *testing.T, src string) *workflow.Spec {
	t.Helper()
	doc, diags := workflow.Parse([]byte(src), workflow.Source{Layer: workflow.LayerHub})
	require.False(t, diags.HasErrors(), "%v", diags)
	return doc.Spec
}

// featureForm is the form of `feature` with tickets: implement-spec under
// the alias, the spec computed by oh.
func featureForm(t *testing.T) *LaunchFormView {
	cfg := LaunchFormConfig{WorkflowID: "implement-spec", Origin: "hub · demo", Spec: parseSpec(t, implementSpecTestSpec), Lang: "fr",
		Alias: "feature", AliasWith: "tickets", Tickets: []string{"oh-12", "oh-13", "oh-14"},
		Prefill:    map[string]string{"instructions": "respecter l'API publique", "method": "tdd"},
		InputNotes: map[string]string{"fanout": i18n.T("tui.launch.input.fanout_off")},
		FromPreview: func(context.Context, string, map[string]string) string {
			return "unused: previews are set by the test"
		}}
	v := NewLaunchFormView(cfg)
	v.inputs.previews = map[string]string{"spec": i18n.Tf("tui.launch.input.from_value", "session.output(spec.spec)", "ses_41c0", "docs/wiki/specs/export.md")}
	v.inputs.previewed = true
	return v
}

func mountedText(t *testing.T, v *LaunchFormView, w, h int) string {
	t.Helper()
	content := tview.NewFlex()
	v.SetShell(&recordingShell{})
	v.Mount(content, tview.NewApplication())
	t.Cleanup(v.Unmount)
	return screenText(t, content, w, h)
}

// Sketch §8: « Lancer · feature → implement-spec (avec tickets) », the help
// under each field, the method as a list, the fan-out disabled, the spec
// computed by oh from the last spec session.
func TestLaunchFormFeatureGolden(t *testing.T) {
	frLocale(t)
	v := featureForm(t)
	assert.Equal(t, "Lancer · feature → implement-spec (avec tickets)", v.Title())
	assert.Equal(t, "launch.implement-spec", v.ID())
	golden.Assert(t, "launch_form_feature", mountedText(t, v, 100, 40))
}

// Sketch §8: the entry agent of `libre` is chosen in the form (F-15).
func TestLaunchFormLibreGolden(t *testing.T) {
	frLocale(t)
	v := NewLaunchFormView(LaunchFormConfig{WorkflowID: "libre", Origin: "hub · demo", Spec: parseSpec(t, libreTestSpec), Lang: "fr",
		EntryAgents: []string{"orchestrator", "debugger", "planner", "developer"}})
	golden.Assert(t, "launch_form_libre", mountedText(t, v, 100, 22))
}

// The origin comes from the hub or the team: a bracket in it is text, not a
// colour tag.
func TestLaunchFormOriginIsEscaped(t *testing.T) {
	frLocale(t)
	v := NewLaunchFormView(LaunchFormConfig{WorkflowID: "libre", Origin: "équipe [red]v3", Spec: parseSpec(t, libreTestSpec), Lang: "fr",
		EntryAgents: []string{"orchestrator"}})
	v.SetShell(&recordingShell{})
	v.Mount(tview.NewFlex(), tview.NewApplication())
	t.Cleanup(v.Unmount)
	header := stripTags(v.headerText())
	assert.Contains(t, header, "équipe [red]v3", "the origin is shown as typed")
}

func TestLaunchFormEntryAgent(t *testing.T) {
	t.Parallel()
	cfg := LaunchFormConfig{WorkflowID: "libre", Spec: parseSpec(t, libreTestSpec), Lang: "fr",
		EntryAgents: []string{"orchestrator", "debugger"}}
	m := newLaunchModel(cfg)
	assert.Equal(t, "orchestrator", m.agent)
	assert.Equal(t, "", m.choices().Agent, "the default entry agent is not sent")
	m.agent = "debugger"
	assert.Equal(t, "debugger", m.choices().Agent)

	cfg.Agent = "debugger" // « session libre avec cet agent »
	assert.Equal(t, "debugger", newLaunchModel(cfg).choices().Agent)
	assert.Empty(t, newLaunchModel(launchCfg(t)).choices().Agent, "no choice without selectable entry")
}

// An int input is checked under its field and blocks the launch; an input
// computed by oh keeps no default in the form (oh computes it first) and is
// never required.
func TestLaunchFormInputTypes(t *testing.T) {
	frLocale(t)
	v := featureForm(t)
	m := v.m
	assert.NotContains(t, m.values, "spec", "computed by oh")
	assert.Equal(t, "tdd", m.values["method"])
	assert.Empty(t, m.validate())

	m.values["rounds"] = "trois"
	assert.Equal(t, i18n.T("tui.launch.input.int"), m.typeError("rounds"))
	assert.Equal(t, []string{"Tours : " + i18n.T("tui.launch.input.int")}, m.validate())
	assert.Contains(t, v.noteText("rounds"), "✗ "+i18n.T("tui.launch.input.int"))
	m.values["rounds"] = "3"
	assert.Empty(t, m.validate())

	assert.Contains(t, v.noteText("spec"), "⚙ "+i18n.Tf("tui.launch.input.from", ""), "computed by oh")
	m.values["spec"] = "docs/wiki/specs/autre.md"
	assert.NotContains(t, v.noteText("spec"), "⚙", "a value given wins")
	assert.Contains(t, v.noteText("method"), "standard · tdd", "the choices of the method")
}

// A path or a branch is checked off the event loop once the typing
// pauses: a warning under the field, never a refusal.
func TestLaunchFormChecksBranch(t *testing.T) {
	cfg := launchCfg(t)
	cfg.Tickets = []string{"bd-1"}
	checked := make(chan string, 4)
	cfg.CheckInput = func(_ context.Context, in workflow.Input, value string) string {
		checked <- value
		if in.Type == workflow.InputBranch && value != "main" {
			return i18n.Tf("tui.launch.input.branch_missing", value)
		}
		return ""
	}
	v := NewLaunchFormView(cfg)
	v.SetShell(&recordingShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	onLoop(app, func() {
		v.m.values["branch"] = "feat/y"
		v.inputChanged("branch")
		v.m.values["branch"] = "feat/x"
		v.inputChanged("branch")
	})
	select {
	case got := <-checked:
		assert.Equal(t, "feat/x", got, "only the last value is checked")
	case <-time.After(2 * time.Second):
		t.Fatal("branch not checked")
	}
	require.Eventually(t, func() bool {
		var txt string
		onLoop(app, func() { txt = v.noteText("branch") })
		return strings.Contains(txt, "⚠ "+i18n.Tf("tui.launch.input.branch_missing", "feat/x"))
	}, 2*time.Second, 10*time.Millisecond)
	var problems []string
	onLoop(app, func() { problems = v.m.validate() })
	assert.Empty(t, problems, "a missing branch does not block")
}
