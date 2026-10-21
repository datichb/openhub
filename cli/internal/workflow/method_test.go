package workflow

import (
	"reflect"
	"strings"
	"testing"
)

// R7: the method of a session (input `method`) and the checkpoints
// declared for it.

const hubTDD = `apiVersion: oh/v1
kind: Workflow
id: tdd
risk: write
entry: { agent: orchestrator-dev }
inputs:
  method: { type: enum, values: [standard, tdd], default: standard }
agents:
  orchestrator-dev: { role: workflow, calls: [developer] }
  developer: { role: workflow }
checkpoints:
  cp-1:
    label: Start
    mode: { manuel: pause, semi-auto: auto, auto: auto }
  cp-seams:
    label: Seams
    method: tdd
    mode: { manuel: pause, semi-auto: pause, auto: auto }
  cp-red:
    label: Red
    method: tdd
    mode: { manuel: auto, semi-auto: auto, auto: auto }
    verify: { from: stack, expect: fail, must_touch_tests: true, outside_tests: warn, since: cp-seams }
  cp-2:
    label: Commit
    mandatory: true
    unlocks: [commit]
    mode: { manuel: pause, semi-auto: pause, auto: pause }
    verify: { from: stack }
`

func tddCatalog(t *testing.T, patch string) *MemCatalog {
	t.Helper()
	docs := map[string]string{"hub:tdd": hubTDD}
	if patch != "" {
		docs["team:tdd"] = "apiVersion: oh/v1\nkind: Workflow\nid: tdd\nextends: hub:tdd\n" + patch
	}
	return catalogOf(t, docs)
}

func checkpointKeys(s *Spec) []string { return s.Checkpoints.Keys() }

func TestMethod_Resolve(t *testing.T) {
	t.Parallel()
	cat := tddCatalog(t, "")
	all := []string{"cp-1", "cp-seams", "cp-red", "cp-2"}
	std := []string{"cp-1", "cp-2"}
	cases := []struct {
		name   string
		opts   *SessionOptions
		method Method
		want   []string
	}{
		{"catalogue view: every checkpoint", nil, "", all},
		{"default input", &SessionOptions{}, MethodStandard, std},
		{"standard", &SessionOptions{Inputs: map[string]any{"method": "standard"}}, MethodStandard, std},
		{"tdd", &SessionOptions{Inputs: map[string]any{"method": "tdd"}}, MethodTDD, all},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, diags := Check(cat, Ref{LayerHub, "tdd"}, c.opts, Env{})
			if diags.HasErrors() {
				t.Fatalf("diags = %v", diagList(diags))
			}
			if r.Method != c.method || !reflect.DeepEqual(checkpointKeys(r.Spec), c.want) {
				t.Fatalf("method %q, checkpoints %v", r.Method, checkpointKeys(r.Spec))
			}
		})
	}
	// An unknown method is refused as any enum value.
	_, diags := Check(cat, Ref{LayerHub, "tdd"}, &SessionOptions{Inputs: map[string]any{"method": "bdd"}}, Env{})
	if !containsStr(diags.Codes(), "session_input_invalid") {
		t.Fatalf("diags = %v", diagList(diags))
	}
	// The default of the input chooses the method.
	r := resolveYAML(t, strings.NewReplacer("id: tdd", "id: x", "default: standard", "default: tdd").Replace(hubTDD))
	if m := r.Spec.MethodOf(nil); m != MethodTDD {
		t.Fatalf("method = %q", m)
	}
	if m := (&Spec{}).MethodOf(map[string]any{"method": "tdd"}); m != MethodStandard {
		t.Fatalf("without the input: %q", m)
	}
	if s := r.Spec.ForMethod(MethodTDD); s != r.Spec {
		t.Fatal("nothing dropped: the spec itself")
	}
	if r.Spec.ForMethod(""); r.Spec.Checkpoints.Len() != 4 {
		t.Fatal("ForMethod changed the spec")
	}
}

func TestMethod_Validate(t *testing.T) {
	t.Parallel()
	doc := vHeader + `risk: write
entry: { agent: orchestrator-dev }
inputs:
  method: { type: enum, values: [standard, tdd, bdd] }
agents:
  orchestrator-dev: { role: workflow }
  developer: { role: workflow, after: cp-b }
checkpoints:
  cp-a:
    label: A
    method: bdd
    mode: { manuel: pause, semi-auto: pause, auto: auto }
  cp-b:
    label: B
    method: tdd
    mandatory: true
    mode: { manuel: pause, semi-auto: pause, auto: pause }
  cp-c:
    label: C
    mode: { manuel: pause, semi-auto: auto, auto: auto }
    verify: { from: stack, outside_tests: maybe, since: cp-b }
  cp-d:
    label: D
    mode: { manuel: pause, semi-auto: auto, auto: auto }
    verify: { from: stack, must_touch_tests: true, outside_tests: warn, since: cp-e }
  cp-e:
    label: E
    mode: { manuel: pause, semi-auto: auto, auto: auto }
`
	diags := checkYAML(t, doc, testEnv())
	want := map[string]string{
		"inputs.method.values":                  "enum_invalid",
		"checkpoints.cp-a.method":               "enum_invalid",
		"checkpoints.cp-b.method":               "checkpoint_method_mandatory",
		"agents.developer.after":                "checkpoint_method_after",
		"checkpoints.cp-c.verify.outside_tests": "enum_invalid|verify_outside_tests_ignored",
		"checkpoints.cp-c.verify.since":         "verify_since_method",
		"checkpoints.cp-d.verify.since":         "verify_since_unknown",
	}
	got := map[string]string{}
	for _, d := range diags {
		if _, ok := want[d.Path]; ok {
			if got[d.Path] != "" {
				got[d.Path] += "|"
			}
			got[d.Path] += d.Code
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v\nall: %v", got, want, diagList(diags))
	}

	noInput := vHeader + "risk: write\ninputs:\n  method: { type: string }\ncheckpoints:\n  cp-a: { label: A, method: tdd, mode: { manuel: pause } }\n"
	codes := checkYAML(t, noInput, testEnv()).Codes()
	if !containsStr(codes, "method_input_type") || !containsStr(codes, "checkpoint_method_no_input") {
		t.Fatalf("codes = %v", codes)
	}
	if diags := checkYAML(t, strings.Replace(hubTDD, "id: tdd", "id: x", 1), testEnv()); diags.HasErrors() {
		t.Fatalf("diags = %v", diagList(diags))
	}
}

// A method drops a checkpoint outside of it: giving one to an inherited
// checkpoint is a loosening; the R7 verification fields only harden.
func TestMethod_PatchOnlyHardens(t *testing.T) {
	t.Parallel()
	loosen := []struct{ name, patch, path string }{
		{"method on an inherited checkpoint", "checkpoints:\n  cp-1: { method: tdd }\n", "checkpoints.cp-1.method"},
		{"must_touch_tests dropped", "checkpoints:\n  cp-red: { verify: { from: stack, expect: fail, outside_tests: warn, since: cp-seams } }\n", "checkpoints.cp-red.verify"},
		{"outside_tests weakened", "checkpoints:\n  cp-red: { verify: { from: stack, expect: fail, must_touch_tests: true, since: cp-seams } }\n", "checkpoints.cp-red.verify"},
		{"since changed", "checkpoints:\n  cp-red: { verify: { from: stack, expect: fail, must_touch_tests: true, outside_tests: warn, since: cp-1 } }\n", "checkpoints.cp-red.verify"},
	}
	for _, c := range loosen {
		t.Run(c.name, func(t *testing.T) {
			_, diags := ResolveSpec(tddCatalog(t, c.patch), Ref{LayerTeam, "tdd"}, nil)
			if len(diags) != 1 || diags[0].Path != c.path || diags[0].Code != "loosening" {
				t.Fatalf("diags = %v", diagList(diags))
			}
		})
	}
	harden := []string{
		"checkpoints:\n  cp-red: { verify: { from: stack, expect: fail, must_touch_tests: true, outside_tests: refuse, since: cp-seams } }\n",
		"checkpoints:\n  cp-seams: { method: tdd, mode: { auto: pause } }\n",
		"checkpoints:\n  cp-extra: { label: X, method: tdd, mode: { manuel: pause, semi-auto: auto, auto: auto } }\n",
	}
	for _, patch := range harden {
		if _, diags := ResolveSpec(tddCatalog(t, patch), Ref{LayerTeam, "tdd"}, nil); diags.HasErrors() {
			t.Fatalf("%s: diags = %v", patch, diagList(diags))
		}
	}
}

// CP-R2 and R7: a method adds at most MethodPauseAllowance pause to the
// budget, for its sessions only.
func TestPauseBudget_Method(t *testing.T) {
	t.Parallel()
	r := resolveYAML(t, strings.Replace(hubTDD, "id: tdd", "id: x", 1))
	c := r.Spec.PauseBudget()
	if !reflect.DeepEqual(c.Pauses, []string{"cp-seams", "cp-2"}) || c.Budget != 3 || c.Exceeded() ||
		!reflect.DeepEqual(c.MethodPauses, map[Method][]string{MethodTDD: {"cp-seams"}}) {
		t.Fatalf("count = %+v", c)
	}
	full := PauseCount{Pauses: []string{"a", "b", "c", "m"}, Budget: 3, MethodPauses: map[Method][]string{MethodTDD: {"m"}}}
	if full.Exceeded() {
		t.Fatal("3 + 1 for the method: within")
	}
	full.Pauses = append(full.Pauses, "n")
	full.MethodPauses[MethodTDD] = []string{"m", "n"}
	if !full.Exceeded() {
		t.Fatal("the method adds 2 pauses: over")
	}
	base := PauseCount{Pauses: []string{"a", "b", "c", "d", "m"}, Budget: 3, MethodPauses: map[Method][]string{MethodTDD: {"m"}}}
	if !base.Exceeded() {
		t.Fatal("over without the method")
	}
}
