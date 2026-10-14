package workflow

import (
	"strings"
	"testing"
)

// R6: fan-out and control substitution (CP-R3). Each guard of CP-R3 that
// lives in the schema has its own case, attempts to get around it included.

const fanoutChild = `apiVersion: oh/v1
kind: Workflow
id: child
risk: write
inputs:
  ticket: { type: beads-id, required: true }
checkpoints:
  cp-1: { mode: { manuel: pause, semi-auto: auto, auto: auto } }
  cp-2: { mandatory: true, unlocks: [commit, close], verify: { from: stack, expect: pass }, mode: { manuel: pause, semi-auto: pause, auto: pause } }
`

const fanoutParent = vHeader + `risk: write
inputs:
  tickets: { type: beads-ids, required: true }
  fanout: { type: bool, default: false }
agents:
  developer: { role: workflow, after: cp-1 }
fanout:
  input: fanout
  tickets: tickets
  child: child
  parallel: 3
  merge: cp-merge
  delegates: [cp-2]
checkpoints:
  cp-plan: { mandatory: true, mode: { manuel: pause, semi-auto: pause, auto: pause } }
  cp-1: { fanout: never, mode: { manuel: pause, semi-auto: auto, auto: auto } }
  cp-2: { fanout: never, mandatory: true, unlocks: [commit, close], verify: { from: stack, expect: pass }, mode: { manuel: pause, semi-auto: pause, auto: pause } }
  cp-merge:
    fanout: only
    mandatory: true
    unlocks: [merge]
    verify: { from: stack, expect: pass, on: integration }
    mode: { manuel: pause, semi-auto: pause, auto: pause }
`

func fanoutCheck(t *testing.T, docs map[string]string, ref Ref, opts *SessionOptions) (*Resolved, Diagnostics) {
	t.Helper()
	if _, ok := docs["hub:child"]; !ok {
		docs["hub:child"] = fanoutChild
	}
	r, diags := Check(catalogOf(t, docs), ref, opts, testEnv())
	if r == nil {
		t.Fatalf("resolution failed: %v", diags)
	}
	return r, diags
}

func errorsOf(diags Diagnostics) []Diagnostic {
	var out []Diagnostic
	for _, d := range diags {
		if d.Severity == SeverityError {
			out = append(out, d)
		}
	}
	return out
}

func TestFanout_HubDeclarationIsValid(t *testing.T) {
	r, diags := fanoutCheck(t, map[string]string{"hub:x": fanoutParent}, Ref{LayerHub, "x"}, nil)
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("valid: %v", diagList(diags))
	}
	if !r.Spec.SubstitutesControl() {
		t.Fatal("cp-merge substitutes the control")
	}
}

func TestFanout_SessionSeesItsCheckpoints(t *testing.T) {
	docs := map[string]string{"hub:x": fanoutParent}
	r, _ := fanoutCheck(t, docs, Ref{LayerHub, "x"}, &SessionOptions{Inputs: map[string]any{"tickets": []string{"a"}}})
	if r.Fanout || r.Spec.Fanout != nil {
		t.Fatal("not a fan-out by default")
	}
	if got := strings.Join(r.Spec.Checkpoints.Keys(), ","); got != "cp-plan,cp-1,cp-2" {
		t.Fatalf("standard session: %s", got)
	}
	r, _ = fanoutCheck(t, docs, Ref{LayerHub, "x"}, &SessionOptions{Inputs: map[string]any{"tickets": []string{"a"}, "fanout": true}})
	if !r.Fanout || r.Spec.Fanout == nil {
		t.Fatal("fan-out session")
	}
	if got := strings.Join(r.Spec.Checkpoints.Keys(), ","); got != "cp-plan,cp-merge" {
		t.Fatalf("fan-out session: %s", got)
	}
	if _, ok := r.Spec.Agents.Get("developer"); ok {
		t.Fatal("the developer gated on a dropped checkpoint is removed")
	}
}

func TestFanout_MergeCheckpointGuarantees(t *testing.T) {
	cases := []struct{ name, from, to, code, path string }{
		{"not mandatory", "    mandatory: true\n    unlocks: [merge]\n", "", "fanout_merge_mandatory", "checkpoints.cp-merge.mandatory"},
		{"no merge unlocked", "    unlocks: [merge]\n", "    unlocks: [commit]\n", "fanout_merge_unlocks", "checkpoints.cp-merge.unlocks"},
		{"no verification", "    verify: { from: stack, expect: pass, on: integration }\n", "", "fanout_merge_verify", "checkpoints.cp-merge.verify"},
		{"verified in the session", "verify: { from: stack, expect: pass, on: integration }", "verify: { from: stack, expect: pass }", "fanout_merge_verify_on", "checkpoints.cp-merge.verify.on"},
		{"expects a failure", "verify: { from: stack, expect: pass, on: integration }", "verify: { from: stack, expect: fail, on: integration }", "fanout_merge_verify_expect", "checkpoints.cp-merge.verify.expect"},
		{"in every session", "    fanout: only\n", "", "fanout_merge_scope", "checkpoints.cp-merge.fanout"},
		{"unknown", "  merge: cp-merge", "  merge: cp-nope", "fanout_merge_unknown", "fanout.merge"},
		{"unknown delegate", "delegates: [cp-2]", "delegates: [cp-9]", "fanout_delegate_unknown", "fanout.delegates[0]"},
		{"unknown child", "  child: child", "  child: nope", "fanout_unknown_workflow", "fanout.child"},
		{"self", "  child: child", "  child: x", "fanout_child_self", "fanout.child"},
		{"too parallel", "parallel: 3", "parallel: 4", "fanout_parallel", "fanout.parallel"},
		{"input not bool", "fanout: { type: bool, default: false }", "fanout: { type: string }", "fanout_input", "fanout.input"},
		{"tickets not ids", "tickets: { type: beads-ids, required: true }", "tickets: { type: string, required: true }", "fanout_tickets", "fanout.tickets"},
		{"merge elsewhere", "  cp-plan: { mandatory: true,", "  cp-plan: { mandatory: true, unlocks: [merge],", "unlock_merge_fanout_only", "checkpoints.cp-plan.unlocks[0]"},
		{"integration elsewhere", "  cp-2: { fanout: never, mandatory: true, unlocks: [commit, close], verify: { from: stack, expect: pass }", "  cp-2: { fanout: never, mandatory: true, unlocks: [commit, close], verify: { from: stack, expect: pass, on: integration }", "verify_on_integration_merge_only", "checkpoints.cp-2.verify.on"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := strings.Replace(fanoutParent, c.from, c.to, 1)
			if doc == fanoutParent {
				t.Fatalf("replacement not applied: %q", c.from)
			}
			_, diags := fanoutCheck(t, map[string]string{"hub:x": doc}, Ref{LayerHub, "x"}, nil)
			assertDiag(t, diags, c.code, c.path, SeverityError)
		})
	}
}

func TestFanout_NestedChildRefused(t *testing.T) {
	nested := strings.Replace(fanoutParent, "id: x\n", "id: child\n", 1)
	nested = strings.Replace(nested, "  child: child", "  child: ticket", 1)
	_, diags := fanoutCheck(t, map[string]string{"hub:x": fanoutParent, "hub:child": nested, "hub:ticket": strings.Replace(fanoutChild, "id: child", "id: ticket", 1)}, Ref{LayerHub, "x"}, nil)
	assertDiag(t, diags, "fanout_child_nested", "fanout.child", SeverityError)
}

func TestFanout_CheckpointScopeWithoutFanout(t *testing.T) {
	diags := checkYAML(t, vHeader+"risk: write\ncheckpoints:\n  cp-1: { fanout: never, mode: { manuel: pause, semi-auto: pause, auto: pause } }\n", testEnv())
	assertDiag(t, diags, "checkpoint_fanout_without_fanout", "checkpoints.cp-1.fanout", SeverityError)
}

// CP-R3: the substitution is declared by the hub layer only. A team or
// project layer that writes the delegates, in a full definition or in a
// patch, is refused; and a patch may only harden the fan-out.
func TestFanout_DelegatesHubOnly(t *testing.T) {
	teamFull := strings.Replace(fanoutParent, "id: x\n", "id: y\n", 1)
	_, diags := fanoutCheck(t, map[string]string{"team:y": teamFull}, Ref{LayerTeam, "y"}, nil)
	assertDiag(t, diags, "fanout_delegates_layer", "fanout.delegates", SeverityError)

	projectFull := strings.Replace(fanoutParent, "id: x\n", "id: z\n", 1)
	_, diags = fanoutCheck(t, map[string]string{"project:z": projectFull}, Ref{LayerProject, "z"}, nil)
	assertDiag(t, diags, "fanout_delegates_layer", "fanout.delegates", SeverityError)

	// Without delegates, a team may declare its own fan-out (the children
	// keep their pauses).
	noDelegates := strings.Replace(teamFull, "  delegates: [cp-2]\n", "", 1)
	_, diags = fanoutCheck(t, map[string]string{"team:y": noDelegates}, Ref{LayerTeam, "y"}, nil)
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("team fan-out without delegation: %v", diagList(diags))
	}
}

func TestFanout_PatchMayOnlyHarden(t *testing.T) {
	hubNoDelegates := strings.Replace(fanoutParent, "  delegates: [cp-2]\n", "", 1)
	patch := func(body string) string { return "apiVersion: oh/v1\nkind: Workflow\nid: x\nextends: hub:x\n" + body }
	cases := []struct{ name, hub, body, path string }{
		{"adds delegates", hubNoDelegates, "fanout: { delegates: [cp-2] }\n", "fanout.delegates"},
		{"changes delegates", fanoutParent, "fanout: { delegates: [cp-1] }\n", "fanout.delegates"},
		{"changes the merge checkpoint", fanoutParent, "fanout: { merge: cp-plan }\n", "fanout.merge"},
		{"changes the child", fanoutParent, "fanout: { child: other }\n", "fanout.child"},
		{"changes the input", fanoutParent, "fanout: { input: tickets }\n", "fanout.input"},
		{"adds a merger", fanoutParent, "fanout: { merger: child }\n", "fanout.merger"},
		{"more children", strings.Replace(fanoutParent, "parallel: 3", "parallel: 2", 1), "fanout: { parallel: 3 }\n", "fanout.parallel"},
		{"removes the fan-out", fanoutParent, "fanout: null\n", "fanout"},
		{"merge not mandatory", fanoutParent, "checkpoints:\n  cp-merge: { mandatory: false }\n", "checkpoints.cp-merge.mandatory"},
		{"merge verified in the session", fanoutParent, "checkpoints:\n  cp-merge: { verify: { from: stack, expect: pass } }\n", "checkpoints.cp-merge.verify"},
		{"merge verification removed", fanoutParent, "checkpoints:\n  cp-merge: { verify: null }\n", "checkpoints.cp-merge.verify"},
		{"merge in every session", fanoutParent, "checkpoints:\n  cp-merge: { fanout: never }\n", "checkpoints.cp-merge.fanout"},
		{"cp-2 dropped from the standard session", fanoutParent, "checkpoints:\n  cp-2: { fanout: only }\n", "checkpoints.cp-2.fanout"},
		{"merge paused by the mode", fanoutParent, "checkpoints:\n  cp-merge: { mode: { auto: auto } }\n", "checkpoints.cp-merge.mode.auto"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, diags := fanoutCheck(t, map[string]string{"hub:x": c.hub, "team:x": patch(c.body)}, Ref{LayerTeam, "x"}, nil)
			assertDiag(t, diags, "loosening", c.path, SeverityError)
		})
	}
	// The merge checkpoint cannot be removed.
	_, diags := fanoutCheck(t, map[string]string{"hub:x": fanoutParent, "team:x": patch("checkpoints:\n  cp-merge: { disabled: true }\n")}, Ref{LayerTeam, "x"}, nil)
	assertDiag(t, diags, "unlocking_checkpoint_removed", "checkpoints.cp-merge.disabled", SeverityError)

	// Fewer children at the same time is a hardening; the delegation of
	// the hub stays.
	r, diags := fanoutCheck(t, map[string]string{"hub:x": fanoutParent, "team:x": patch("fanout: { parallel: 1 }\n")}, Ref{LayerTeam, "x"}, nil)
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("lower parallel: %v", diagList(diags))
	}
	if r.Spec.Fanout.EffectiveParallel() != 1 || strings.Join(r.Spec.Fanout.Delegates, ",") != "cp-2" || !r.Spec.SubstitutesControl() {
		t.Fatalf("hardened fan-out: %s", r.Spec.Fanout)
	}
}

// A draft of the hub workflow never declares the substitution either.
func TestFanout_DelegatesNotFromDraft(t *testing.T) {
	r := &Resolved{Ref: Ref{LayerHub, "x"}, Spec: &Spec{Fanout: &Fanout{Delegates: []string{"cp-2"}}},
		Origins: Origins{"fanout.delegates": {Layer: LayerTeam, ID: "x", Draft: true}}}
	refused := func() bool {
		v := &validator{r: r, s: r.Spec}
		v.fanoutDelegatesLayer(r.Spec.Fanout)
		return len(v.diags) == 1 && v.diags[0].Code == "fanout_delegates_layer"
	}
	if !refused() {
		t.Fatal("team draft")
	}
	r.Origins = Origins{"fanout.delegates": {Layer: LayerHub, ID: "x", Draft: true}}
	if !refused() {
		t.Fatal("hub draft")
	}
	r.Origins = Origins{"fanout.delegates": {Layer: LayerHub, ID: "x"}}
	if refused() {
		t.Fatal("published hub document")
	}
}

func TestFanoutOf(t *testing.T) {
	r, _ := fanoutCheck(t, map[string]string{"hub:x": fanoutParent}, Ref{LayerHub, "x"}, nil)
	for _, c := range []struct {
		in   any
		want bool
	}{{nil, false}, {true, true}, {false, false}, {"true", true}, {"false", false}} {
		in := map[string]any{}
		if c.in != nil {
			in["fanout"] = c.in
		}
		if got := r.Spec.FanoutOf(in); got != c.want {
			t.Errorf("%v: %v", c.in, got)
		}
	}
	if (&Spec{}).FanoutOf(map[string]any{"fanout": true}) {
		t.Fatal("no fan-out declared")
	}
}

// fanout.inputs: only declared value inputs of the parent reach the
// children, under a name and a type the child workflow declares.
func TestFanout_ChildInputs(t *testing.T) {
	child := strings.Replace(fanoutChild, "  ticket: { type: beads-id, required: true }\n",
		"  ticket: { type: beads-id, required: true }\n  method: { type: enum, values: [standard, tdd], default: standard }\n  instructions: { type: text }\n  count: { type: int }\n", 1)
	parent := strings.Replace(fanoutParent, "  fanout: { type: bool, default: false }\n",
		"  fanout: { type: bool, default: false }\n  method: { type: enum, values: [standard, tdd], default: standard }\n  instructions: { type: text }\n  count: { type: string }\n  other: { type: text }\n  big: { type: enum, values: [standard, tdd, bdd] }\n", 1)
	ok := strings.Replace(parent, "  delegates: [cp-2]\n", "  delegates: [cp-2]\n  inputs: [method, instructions]\n", 1)
	r, diags := fanoutCheck(t, map[string]string{"hub:x": ok, "hub:child": child}, Ref{LayerHub, "x"},
		&SessionOptions{Inputs: map[string]any{"tickets": []string{"a"}, "fanout": true, "method": "tdd", "other": "x"}})
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("valid inputs: %v", diagList(diags))
	}
	got := r.Spec.ChildInputs(map[string]any{"method": "tdd", "other": "x", "tickets": []string{"a"}})
	if len(got) != 1 || got["method"] != "tdd" {
		t.Fatalf("only the declared inputs given, not the defaults: %v", got)
	}

	cases := []struct{ name, list, code string }{
		{"unknown in the parent", "[nope]", "fanout_inputs_parent"},
		{"the tickets", "[tickets]", "fanout_inputs_parent"},
		{"the fan-out input", "[fanout]", "fanout_inputs_parent"},
		{"absent from the child", "[other]", "fanout_inputs_child"},
		{"another type", "[count]", "fanout_inputs_type"},
		{"an enum absent from the child", "[big]", "fanout_inputs_child"},
		{"twice", "[method, method]", "duplicate_entry"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := strings.Replace(parent, "  delegates: [cp-2]\n", "  delegates: [cp-2]\n  inputs: "+c.list+"\n", 1)
			_, diags := fanoutCheck(t, map[string]string{"hub:x": doc, "hub:child": child}, Ref{LayerHub, "x"}, nil)
			found := false
			for _, d := range errorsOf(diags) {
				found = found || d.Code == c.code
			}
			if !found {
				t.Fatalf("want %s: %v", c.code, diagList(diags))
			}
		})
	}
	// An enum value the child does not know.
	bigChild := strings.Replace(child, "  instructions: { type: text }\n", "  instructions: { type: text }\n  big: { type: enum, values: [standard, tdd] }\n", 1)
	doc := strings.Replace(parent, "  delegates: [cp-2]\n", "  delegates: [cp-2]\n  inputs: [big]\n", 1)
	_, diags = fanoutCheck(t, map[string]string{"hub:x": doc, "hub:child": bigChild}, Ref{LayerHub, "x"}, nil)
	assertDiag(t, diags, "fanout_inputs_value", "fanout.inputs[0]", SeverityError)

	// A team layer may give fewer inputs, never more.
	patch := func(body string) string { return "apiVersion: oh/v1\nkind: Workflow\nid: x\nextends: hub:x\n" + body }
	r, diags = fanoutCheck(t, map[string]string{"hub:x": ok, "hub:child": child, "team:x": patch("fanout: { inputs: [method] }\n")}, Ref{LayerTeam, "x"}, nil)
	if errs := errorsOf(diags); len(errs) > 0 || strings.Join(r.Spec.Fanout.Inputs, ",") != "method" {
		t.Fatalf("fewer inputs: %v %v", r.Spec.Fanout, diagList(diags))
	}
	_, diags = fanoutCheck(t, map[string]string{"hub:x": ok, "hub:child": child, "team:x": patch("fanout: { inputs: [method, instructions, other] }\n")}, Ref{LayerTeam, "x"}, nil)
	assertDiag(t, diags, "loosening", "fanout.inputs", SeverityError)
}
