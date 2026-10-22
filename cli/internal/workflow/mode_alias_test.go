package workflow

import (
	"strings"
	"testing"
)

const manualWorkflow = `apiVersion: oh/v1
kind: Workflow
id: demo
risk: write
entry: { agent: orchestrator }
agents:
  orchestrator: { role: workflow }
checkpoints:
  cp-1: { label: Start, mode: { manual: pause, semi-auto: auto, auto: auto } }
modes: { default: manual, allowed: [manual, semi-auto, auto] }
`

// U-04: « manual » is read as « manuel » wherever a mode is written.
func TestParseReadsManualAsManuel(t *testing.T) {
	doc, diags := Parse([]byte(manualWorkflow), Source{Layer: LayerHub})
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	s := doc.Spec
	if s.Modes.Default != ModeManual || strings.Join(s.Modes.Allowed, ",") != "manuel,semi-auto,auto" {
		t.Fatalf("modes = %+v", s.Modes)
	}
	cp, _ := s.Checkpoints.Get("cp-1")
	if _, old := cp.Mode["manual"]; old || cp.Mode["manuel"] != BehaviorPause || len(cp.Mode) != 3 {
		t.Fatalf("checkpoint modes = %v", cp.Mode)
	}
	if !doc.Has("checkpoints.cp-1.mode.manuel") || doc.Has("checkpoints.cp-1.mode.manual") {
		t.Fatalf("index paths = %v", doc.Paths())
	}
	if doc.Pos("checkpoints.cp-1.mode.manuel").IsZero() {
		t.Fatal("the position of the renamed key is kept")
	}
	if got := s.DefaultMode(); got != "manuel" {
		t.Fatalf("DefaultMode = %q", got)
	}
	cat := catalogOf(t, map[string]string{"hub:demo": manualWorkflow})
	if _, d := Check(cat, Ref{LayerHub, "demo"}, nil, Env{Agents: fakeAgents{"orchestrator": {Mode: ModePrimary}}}); d.HasErrors() {
		t.Fatalf("validate: %v", d)
	}
}

func TestParseRefusesBothSpellingsOfManual(t *testing.T) {
	src := strings.Replace(manualWorkflow, "mode: { manual: pause,", "mode: { manuel: auto, manual: pause,", 1)
	_, diags := Parse([]byte(src), Source{Layer: LayerHub})
	if !diags.HasErrors() {
		t.Fatal("a checkpoint written with manuel and manual must be reported")
	}
}

// --mode manual launches in manuel (and « manual » alone is not a mode name
// of a workflow that does not allow manuel).
func TestSessionModeManualIsManuel(t *testing.T) {
	cat := catalogOf(t, map[string]string{"hub:ticket": hubTicket})
	r, diags := ResolveSpec(cat, mustRef(t, "hub:ticket"), &SessionOptions{Mode: "manual", Inputs: map[string]any{"ticket": "bd-1"}})
	if r == nil || diags.HasErrors() {
		t.Fatal(diags)
	}
	if r.Mode != "manuel" {
		t.Fatalf("mode = %q", r.Mode)
	}
	if _, diags := ResolveSpec(cat, mustRef(t, "hub:ticket"), &SessionOptions{Mode: "turbo", Inputs: map[string]any{"ticket": "bd-1"}}); !diags.HasErrors() {
		t.Fatal("unknown mode accepted")
	}
}

func TestCanonicalMode(t *testing.T) {
	for in, want := range map[string]string{"manual": "manuel", "manuel": "manuel", "auto": "auto", "custom": "custom", "": ""} {
		if got := CanonicalMode(in); got != want {
			t.Errorf("CanonicalMode(%q) = %q, want %q", in, got, want)
		}
	}
}

// U-04: libre gets the alias « free » but stays a workflow: a team layer
// extending hub:libre still resolves (the extends_alias rule is for ids that
// are only aliases).
func TestExtendsLibreWithAlias(t *testing.T) {
	hub := "apiVersion: oh/v1\nkind: Workflow\nid: libre\nrisk: write\naliases: [free]\nentry: { agent: orchestrator }\nagents:\n  orchestrator: { role: workflow }\n"
	team := "apiVersion: oh/v1\nkind: Workflow\nid: libre\nextends: hub:libre\n"
	cat := catalogOf(t, map[string]string{"hub:libre": hub, "team:libre": team})
	if r, diags := ResolveSpec(cat, mustRef(t, "team:libre"), nil); r == nil || diags.HasErrors() {
		t.Fatalf("extends hub:libre: %v", diags)
	}
}
