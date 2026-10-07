package workflow

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// catalogOf parses docs ("layer" → YAML) into a catalog.
func catalogOf(t *testing.T, docs map[string]string) *MemCatalog {
	t.Helper()
	cat := NewMemCatalog()
	for name, src := range docs {
		layer, _, _ := strings.Cut(name, ":")
		doc, diags := Parse([]byte(src), Source{Layer: Layer(layer), Path: name + ".yaml"})
		if diags.HasErrors() {
			t.Fatalf("%s: %v", name, diags)
		}
		if ds := cat.Add(doc); ds != nil {
			t.Fatalf("%s: %v", name, ds)
		}
	}
	return cat
}

const hubTicket = `apiVersion: oh/v1
kind: Workflow
id: ticket
version: 2
category: develop
description: Ticket
risk: write
isolation: strict
entry: { agent: orchestrator-dev }
inputs:
  ticket: { type: beads-id, required: true, picker: { filter: ai-delegated, multi: true } }
  branch: { type: string, default: "feat/{{ .ticket }}" }
prompt: { text: "Do {{ .ticket }}" }
agents:
  orchestrator-dev: { role: workflow, calls: [developer, reviewer] }
  developer: { role: workflow, after: cp-1 }
  reviewer: { role: workflow, after: developer }
  documentarian: { role: independent }
checkpoints:
  cp-1: { label: Start, mode: { manuel: pause, semi-auto: auto, auto: auto }, remote: auto }
  cp-2: { label: Commit, mandatory: true, mode: { manuel: pause, semi-auto: pause, auto: pause }, remote: defer }
modes: { default: semi-auto, allowed: [manuel, semi-auto, auto] }
beads: { allow: [show, update, close, list] }
runtime: { default: local, allowed: [local, container, remote] }
outputs:
  - { id: branch, type: branch }
limits: { budget_usd: 5, models: ["eu.anthropic.*"] }
`

func resolveOK(t *testing.T, cat Catalog, ref string) *Resolved {
	t.Helper()
	r, diags := ResolveSpec(cat, mustRef(t, ref), nil)
	if r == nil || diags.HasErrors() {
		t.Fatalf("resolve %s: %v", ref, diags)
	}
	return r
}

func mustRef(t *testing.T, s string) Ref {
	t.Helper()
	r, err := ParseRef(s)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestResolve_RootOnly(t *testing.T) {
	cat := catalogOf(t, map[string]string{"hub:ticket": hubTicket})
	r := resolveOK(t, cat, "hub:ticket")
	if r.Spec.ID != "ticket" || r.Mode != ModeSemiAuto || r.Runtime != RuntimeLocal {
		t.Fatalf("resolved = %+v", r)
	}
	if !reflect.DeepEqual(r.Chain, []Ref{{LayerHub, "ticket"}}) {
		t.Fatalf("chain = %v", r.Chain)
	}
	o, ok := r.Origins.Of("checkpoints.cp-2.remote")
	if !ok || o.Layer != LayerHub || o.Version != 2 || o.Source != "hub:ticket.yaml" || o.Pos.Line != 21 {
		t.Fatalf("origin = %+v", o)
	}
}

func TestResolve_PatchChainAndOrigins(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:ticket": hubTicket,
		"team:ticket": `apiVersion: oh/v1
kind: Workflow
id: ticket
version: 7
extends: hub:ticket
description: { fr: Ticket équipe, en: Team ticket }
inputs:
  branch: { default: "fix/{{ .ticket }}" }
  publish: { type: bool, default: false }
agents:
  documentarian: { role: disabled }
  designer: { role: independent }
checkpoints:
  cp-1: { mode: { auto: pause } }
  cp-3: { label: Deploy, mode: { manuel: pause } }
runtime: { allowed: [local, container] }
`,
		"project:ticket": `apiVersion: oh/v1
kind: Workflow
id: ticket
extends: team:ticket
checkpoints:
  cp-1: { disabled: true }
inputs:
  ticket: { required: false }
`,
	})
	r := resolveOK(t, cat, "project:ticket")
	s := r.Spec
	if got := r.Chain; !reflect.DeepEqual(got, []Ref{{LayerHub, "ticket"}, {LayerTeam, "ticket"}, {LayerProject, "ticket"}}) {
		t.Fatalf("chain = %v", got)
	}
	if got := s.Inputs.Keys(); !reflect.DeepEqual(got, []string{"ticket", "branch", "publish"}) {
		t.Fatalf("inputs = %v", got)
	}
	ticket, _ := s.Inputs.Get("ticket")
	if ticket.Required || ticket.Picker == nil || !ticket.Picker.Multi {
		t.Fatalf("ticket = %+v (required: false must apply, picker kept)", ticket)
	}
	branch, _ := s.Inputs.Get("branch")
	if branch.Default != "fix/{{ .ticket }}" || branch.Type != InputString {
		t.Fatalf("branch = %+v", branch)
	}
	if got := s.Agents.Keys(); !reflect.DeepEqual(got, []string{"orchestrator-dev", "developer", "reviewer", "designer"}) {
		t.Fatalf("agents = %v", got)
	}
	if got := s.Checkpoints.Keys(); !reflect.DeepEqual(got, []string{"cp-2", "cp-3"}) {
		t.Fatalf("checkpoints = %v", got)
	}
	if got := s.AllowedRuntimes(); !reflect.DeepEqual(got, []Runtime{RuntimeLocal, RuntimeContainer}) {
		t.Fatalf("runtimes = %v", got)
	}
	if s.Description.Text("en") != "Team ticket" || s.Version != 0 || s.Extends != "" {
		t.Fatalf("header = %+v", s)
	}

	cases := map[string]Layer{
		"inputs.branch.default":   LayerTeam,
		"inputs.branch.type":      LayerHub,
		"inputs.ticket.required":  LayerProject,
		"inputs.publish":          LayerTeam,
		"runtime.allowed":         LayerTeam,
		"runtime.allowed[1]":      LayerTeam,
		"runtime.default":         LayerHub,
		"checkpoints.cp-2.remote": LayerHub,
		"description":             LayerTeam,
	}
	for path, want := range cases {
		if o, ok := r.Origins.Of(path); !ok || o.Layer != want {
			t.Errorf("origin %s = %+v, want %s", path, o, want)
		}
	}
	// A list replaced as a whole leaves no stale item origins.
	if _, ok := r.Origins["runtime.allowed[2]"]; ok {
		t.Error("stale origin for runtime.allowed[2]")
	}
	// Removed entries leave no origins.
	for _, p := range []string{"checkpoints.cp-1", "checkpoints.cp-1.remote", "agents.documentarian"} {
		if _, ok := r.Origins[p]; ok {
			t.Errorf("origin left for removed %s", p)
		}
	}
	// The parent documents are untouched.
	hub, _ := cat.Lookup(Ref{LayerHub, "ticket"})
	if hub.Spec.Checkpoints.Len() != 2 || hub.Spec.Agents.Len() != 4 {
		t.Fatal("parent document mutated")
	}
}

func TestResolve_HardeningOnly(t *testing.T) {
	cases := []struct {
		name, patch, path string
	}{
		{"risk", "risk: publish\n", "risk"},
		{"isolation", "isolation: standard\n", "isolation"},
		{"beads widened", "beads: { allow: [show, create] }\n", "beads.allow"},
		{"beads unrestricted", "beads: null\n", "beads"},
		{"mandatory dropped", "checkpoints:\n  cp-2: { mandatory: false }\n", "checkpoints.cp-2.mandatory"},
		{"mandatory relaxed", "checkpoints:\n  cp-2: { mode: { auto: auto } }\n", "checkpoints.cp-2.mode.auto"},
		{"remote relaxed", "checkpoints:\n  cp-2: { remote: auto }\n", "checkpoints.cp-2.remote"},
		{"budget raised", "limits: { budget_usd: 50 }\n", "limits.budget_usd"},
		{"budget removed", "limits: { budget_usd: null }\n", "limits.budget_usd"},
		{"models widened", "limits: { models: [\"us.*\"] }\n", "limits.models"},
		{"models removed", "limits: { models: [] }\n", "limits.models"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cat := catalogOf(t, map[string]string{
				"hub:ticket":  hubTicket,
				"team:ticket": "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\n" + c.patch,
			})
			_, diags := ResolveSpec(cat, Ref{LayerTeam, "ticket"}, nil)
			if len(diags) != 1 || diags[0].Code != "loosening" {
				t.Fatalf("diags = %v", diags)
			}
			if c.path != "" && diags[0].Path != c.path {
				t.Fatalf("path = %q, want %q", diags[0].Path, c.path)
			}
			if diags[0].Source != "team:ticket.yaml" || diags[0].Pos.Line < 5 {
				t.Fatalf("location = %s %v", diags[0].Source, diags[0].Pos)
			}
		})
	}
}

func TestResolve_RuntimeCannotWiden(t *testing.T) {
	for _, patch := range []string{
		"runtime: { allowed: [local, remote] }\n",
		"runtime: { default: remote }\n", // a default outside the parent list is caught by validation
	} {
		cat := catalogOf(t, map[string]string{
			"hub:quick":  "apiVersion: oh/v1\nkind: Workflow\nid: quick\n",
			"team:quick": "apiVersion: oh/v1\nkind: Workflow\nid: quick\nextends: hub:quick\n" + patch,
		})
		r, diags := ResolveSpec(cat, Ref{LayerTeam, "quick"}, nil)
		if strings.Contains(patch, "allowed") {
			if got := diags.Codes(); !reflect.DeepEqual(got, []string{"loosening"}) {
				t.Fatalf("%s: codes = %v", patch, got)
			}
			continue
		}
		if got := r.Spec.AllowedRuntimes(); !reflect.DeepEqual(got, []Runtime{RuntimeLocal}) {
			t.Fatalf("allowed = %v", got)
		}
	}
}

func TestResolve_HardeningAccepted(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:ticket": hubTicket,
		"team:ticket": `apiVersion: oh/v1
kind: Workflow
id: ticket
extends: hub:ticket
risk: read
beads: { allow: [show, list] }
runtime: { allowed: [local] }
limits: { budget_usd: 2, models: ["eu.anthropic.claude-haiku-*"] }
checkpoints:
  cp-1: { remote: forbid, mandatory: true }
  cp-2: { remote: forbid }
`,
	})
	r := resolveOK(t, cat, "team:ticket")
	cp1, _ := r.Spec.Checkpoints.Get("cp-1")
	if r.Spec.Risk != RiskRead || !cp1.IsMandatory() || cp1.Remote != RemoteForbid || *r.Spec.Limits.BudgetUSD != 2 ||
		!reflect.DeepEqual(r.Spec.Limits.Models, []string{"eu.anthropic.claude-haiku-*"}) {
		t.Fatalf("spec = %+v", r.Spec)
	}
}

func TestResolve_MandatoryCheckpointCannotBeRemoved(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:ticket":  hubTicket,
		"team:ticket": "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\ncheckpoints:\n  cp-2: { disabled: true }\n",
	})
	_, diags := ResolveSpec(cat, Ref{LayerTeam, "ticket"}, nil)
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"mandatory_checkpoint_removed"}) {
		t.Fatalf("codes = %v", got)
	}
}

func TestResolve_EntryCannotBeDisabled(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:ticket":  hubTicket,
		"team:ticket": "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\nagents:\n  orchestrator-dev: { role: disabled }\n",
	})
	_, diags := ResolveSpec(cat, Ref{LayerTeam, "ticket"}, nil)
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"entry_disabled"}) {
		t.Fatalf("codes = %v", got)
	}
}

func TestResolve_ChainErrors(t *testing.T) {
	hdr := func(id, extends string) string {
		s := "apiVersion: oh/v1\nkind: Workflow\nid: " + id + "\n"
		if extends != "" {
			s += "extends: " + extends + "\n"
		}
		return s
	}
	cases := []struct {
		name string
		docs map[string]string
		ref  string
		code string
	}{
		{"unknown", map[string]string{"hub:a": hdr("a", "")}, "hub:b", "unknown_workflow"},
		{"parent missing", map[string]string{"team:a": hdr("a", "hub:zz")}, "team:a", "extends_not_found"},
		{"bad ref", map[string]string{"team:a": hdr("a", "ticket")}, "team:a", "invalid_extends"},
		{"more specific", map[string]string{"team:a": hdr("a", "project:b"), "project:b": hdr("b", "")}, "team:a", "extends_more_specific"},
		{"cycle", map[string]string{"team:a": hdr("a", "team:b"), "team:b": hdr("b", "team:a")}, "team:a", "extends_cycle"},
		{"shadow", map[string]string{"hub:a": hdr("a", ""), "team:a": hdr("a", "")}, "team:a", "shadow_without_extends"},
		{"skips layer", map[string]string{"hub:a": hdr("a", ""), "team:a": hdr("a", "hub:a"), "project:a": hdr("a", "hub:a")}, "project:a", "must_extend_nearest"},
		{"shadow via other id", map[string]string{"hub:a": hdr("a", ""), "hub:base": hdr("base", ""), "team:a": hdr("a", "hub:base")}, "team:a", "must_extend_nearest"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, diags := ResolveSpec(catalogOf(t, c.docs), mustRef(t, c.ref), nil)
			if r != nil || len(diags) != 1 || diags[0].Code != c.code {
				t.Fatalf("r=%v diags=%v, want %s", r != nil, diags, c.code)
			}
		})
	}
}

func TestResolve_SameLayerExtendsOtherID(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:ticket":         hubTicket,
		"team:ticket-hotfix": "apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: hub:ticket\nrisk: write\n",
	})
	r := resolveOK(t, cat, "team:ticket-hotfix")
	if r.Spec.ID != "ticket-hotfix" || r.Spec.Agents.Len() != 4 {
		t.Fatalf("spec = %+v", r.Spec)
	}
}

func TestResolve_SessionOptions(t *testing.T) {
	cat := catalogOf(t, map[string]string{"hub:ticket": hubTicket})
	r, diags := ResolveSpec(cat, Ref{LayerHub, "ticket"}, &SessionOptions{
		Mode: ModeAuto, Runtime: RuntimeContainer,
		Inputs: map[string]any{"ticket": "bd-1", "branch": "x"},
	})
	if len(diags) != 0 || r.Mode != ModeAuto || r.Runtime != RuntimeContainer || r.Inputs["ticket"] != "bd-1" {
		t.Fatalf("r = %+v, diags = %v", r, diags)
	}

	_, diags = ResolveSpec(cat, Ref{LayerHub, "ticket"}, &SessionOptions{
		Mode: "yolo", Runtime: "cloud",
		Inputs: map[string]any{"nope": 1},
	})
	want := []string{"session_mode_not_allowed", "session_runtime_not_allowed", "session_input_unknown", "session_input_missing"}
	if got := diags.Codes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("codes = %v", got)
	}
	if diags[0].Source != "session" || diags[2].Path != "session.inputs.nope" {
		t.Fatalf("diags = %+v", diags)
	}
}

func TestCheckInputValue(t *testing.T) {
	cases := []struct {
		in   Input
		v    any
		want bool
	}{
		{Input{Type: InputBool}, "true", true},
		{Input{Type: InputBool}, "yes", false},
		{Input{Type: InputInt}, "12", true},
		{Input{Type: InputInt}, 3, true},
		{Input{Type: InputInt}, "x", false},
		{Input{Type: InputEnum, Values: []string{"a", "b"}}, "b", true},
		{Input{Type: InputEnum, Values: []string{"a", "b"}}, "c", false},
		{Input{Type: InputBeadsID}, []any{"a"}, false},
		{Input{Type: InputBeadsID, Picker: &Picker{Multi: true}}, []any{"a", "b"}, true},
		{Input{Type: InputBeadsIDs}, []string{"a"}, true},
		{Input{Type: InputString}, 3, false},
	}
	for i, c := range cases {
		if _, ok := checkInputValue(c.in, c.v); ok != c.want {
			t.Errorf("#%d %v %v: got %v", i, c.in.Type, c.v, ok)
		}
	}
}

func TestApplyPatch_LeavesParentIntact(t *testing.T) {
	parentDoc, _ := Parse([]byte(hubTicket), Source{Layer: LayerHub})
	patch, _ := Parse([]byte("apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\nagents:\n  developer: { calls: [reviewer] }\nmodes: { allowed: [manuel] , default: manuel }\n"), Source{Layer: LayerTeam})
	out, diags := ApplyPatch(parentDoc.Spec, patch, "hub:ticket")
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	dev, _ := out.Agents.Get("developer")
	if !reflect.DeepEqual(dev.Calls, []string{"reviewer"}) || dev.After != "cp-1" {
		t.Fatalf("developer = %+v", dev)
	}
	if orig, _ := parentDoc.Spec.Agents.Get("developer"); orig.Calls != nil {
		t.Fatal("parent mutated")
	}
	if out.DefaultMode() != ModeManual || len(parentDoc.Spec.Modes.Allowed) != 3 {
		t.Fatal("modes not patched or parent mutated")
	}
}

func TestSpecClone_Independent(t *testing.T) {
	s := loadFullSpec(t)
	c := s.Clone()
	if !reflect.DeepEqual(s, c) {
		t.Fatal("clone differs")
	}
	cp, _ := c.Checkpoints.Get("cp-1")
	cp.Mode[ModeAuto] = BehaviorSkip
	c.Plugins[1].Options["verbose"] = false
	*c.Limits.BudgetUSD = 1
	c.Inputs.Set("new", Input{Type: InputString})
	if orig, _ := s.Checkpoints.Get("cp-1"); orig.Mode[ModeAuto] != BehaviorAuto {
		t.Fatal("checkpoint mode shared")
	}
	if s.Plugins[1].Options["verbose"] != true || *s.Limits.BudgetUSD != 5 || s.Inputs.Len() != 4 {
		t.Fatal("clone shares state")
	}
}

func TestSubagentGraphAndMaxDepth(t *testing.T) {
	cat := catalogOf(t, map[string]string{"hub:ticket": hubTicket})
	r := resolveOK(t, cat, "hub:ticket")
	members, graph := SubagentGraph(r.Spec, testAgents)
	if !reflect.DeepEqual(members, []string{"orchestrator-dev", "developer", "reviewer", "documentarian"}) {
		t.Fatalf("members = %v", members)
	}
	want := map[string][]string{
		"orchestrator-dev": {"developer", "reviewer"}, // explicit calls
		"developer":        {"documentarian"},         // derived from task permission
		"reviewer":         {"documentarian"},
	}
	if !reflect.DeepEqual(graph, want) {
		t.Fatalf("graph = %v", graph)
	}
	if d := MaxDepth("orchestrator-dev", graph); d != 2 {
		t.Fatalf("depth = %d", d)
	}
	if d := MaxDepth("documentarian", graph); d != 1 {
		t.Fatalf("leaf depth = %d, want the minimum 1", d)
	}
	if d := MaxDepth("a", map[string][]string{"a": {"b"}, "b": {"c", "a"}, "c": {"d"}}); d != 3 {
		t.Fatalf("cyclic depth = %d", d)
	}

	// Implicit conductor entry: derived from its task permission ("*").
	quick := catalogOf(t, map[string]string{"hub:quick": vHeader + "risk: write\nagents:\n  developer: { role: workflow }\n"})
	rq, _ := ResolveSpec(quick, Ref{LayerHub, "x"}, nil)
	members, graph = SubagentGraph(rq.Spec, testAgents)
	if members[0] != "conductor" || !reflect.DeepEqual(graph["conductor"], []string{"developer"}) {
		t.Fatalf("members = %v graph = %v", members, graph)
	}
}

// QB1: code_mode and modes.allowed are security fields (hardening only).
func TestResolve_CodeModeAndModesHardeningOnly(t *testing.T) {
	const hubQuick = "apiVersion: oh/v1\nkind: Workflow\nid: quick\nmodes: { default: manuel, allowed: [manuel, semi-auto] }\n"
	refused := []struct{ name, hub, patch, path string }{
		{"code mode enabled", hubQuick, "code_mode: true\n", "code_mode"},
		{"code mode enabled from unset", "apiVersion: oh/v1\nkind: Workflow\nid: quick\n", "code_mode: true\n", "code_mode"},
		{"mode added", hubQuick, "modes: { allowed: [manuel, semi-auto, auto] }\n", "modes.allowed"},
		{"modes reset to all", hubQuick, "modes: { allowed: [] }\n", "modes.allowed"},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			cat := catalogOf(t, map[string]string{
				"hub:quick":  c.hub,
				"team:quick": "apiVersion: oh/v1\nkind: Workflow\nid: quick\nextends: hub:quick\n" + c.patch,
			})
			r, diags := ResolveSpec(cat, Ref{LayerTeam, "quick"}, nil)
			if len(diags) != 1 || diags[0].Code != "loosening" || diags[0].Path != c.path {
				t.Fatalf("diags = %v", diags)
			}
			if r != nil && r.Spec != nil && (codeMode(r.Spec) || (c.hub == hubQuick && slices.Contains(r.Spec.AllowedModes(), ModeAuto))) {
				t.Fatalf("parent value not kept: %+v", r.Spec)
			}
		})
	}

	cat := catalogOf(t, map[string]string{
		"hub:quick":  "apiVersion: oh/v1\nkind: Workflow\nid: quick\ncode_mode: true\n",
		"team:quick": "apiVersion: oh/v1\nkind: Workflow\nid: quick\nextends: hub:quick\ncode_mode: false\nmodes: { default: manuel, allowed: [manuel] }\n",
	})
	r := resolveOK(t, cat, "team:quick")
	if codeMode(r.Spec) || !reflect.DeepEqual(r.Spec.AllowedModes(), []string{ModeManual}) {
		t.Fatalf("hardening refused: %+v", r.Spec)
	}
}

func codeMode(s *Spec) bool { return s.CodeMode != nil && *s.CodeMode }
