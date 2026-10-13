package workflow

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// R3: verify: and when: on a checkpoint, and the pause budget (CP-R2).

const hubVerified = `apiVersion: oh/v1
kind: Workflow
id: verified
risk: write
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow, calls: [developer, reviewer] }
  developer: { role: workflow }
  reviewer: { role: workflow }
checkpoints:
  cp-spec:
    label: Spec
    mode: { manuel: pause, semi-auto: conditional, auto: auto }
    when: { paths: ["docs/specs/**"] }
  cp-2:
    label: Commit
    unlocks: [commit]
    mode: { manuel: pause, semi-auto: pause, auto: pause }
    verify: { from: stack, kinds: [build, test], expect: pass, timeout: 5m }
  cp-3:
    label: Recap
    mode: { manuel: pause, semi-auto: auto, auto: auto }
    verify: { cmd: ["make check"] }
`

func verifiedDiags(t *testing.T, patch string) (*Resolved, Diagnostics) {
	t.Helper()
	cat := catalogOf(t, map[string]string{
		"hub:verified":  hubVerified,
		"team:verified": "apiVersion: oh/v1\nkind: Workflow\nid: verified\nextends: hub:verified\n" + patch,
	})
	return ResolveSpec(cat, Ref{LayerTeam, "verified"}, nil)
}

// Removing or weakening an inherited verification, or dropping a condition,
// is a loosening: the parent value is kept.
func TestPatch_VerifyOnlyHardens(t *testing.T) {
	loosen := []struct{ name, patch, path string }{
		{"removed", "checkpoints:\n  cp-2: { verify: null }\n", "checkpoints.cp-2.verify"},
		{"expect", "checkpoints:\n  cp-2: { verify: { from: stack, kinds: [build, test], expect: fail, timeout: 5m } }\n", "checkpoints.cp-2.verify"},
		{"fewer kinds", "checkpoints:\n  cp-2: { verify: { from: stack, kinds: [build], timeout: 5m } }\n", "checkpoints.cp-2.verify"},
		{"shorter timeout", "checkpoints:\n  cp-2: { verify: { from: stack, kinds: [build, test], timeout: 1m } }\n", "checkpoints.cp-2.verify"},
		{"no source", "checkpoints:\n  cp-2: { verify: { cmd: [\"true\"] } }\n", "checkpoints.cp-2.verify"},
		{"cmd dropped", "checkpoints:\n  cp-3: { verify: { cmd: [\"make lint\"] } }\n", "checkpoints.cp-3.verify"},
		{"checkpoint disabled", "checkpoints:\n  cp-3: { disabled: true }\n", "checkpoints.cp-3.disabled"},
		{"when dropped", "checkpoints:\n  cp-spec: { when: null }\n", "checkpoints.cp-spec.when"},
		{"when narrowed", "checkpoints:\n  cp-spec: { when: { labels: [ux] } }\n", "checkpoints.cp-spec.when"},
	}
	for _, c := range loosen {
		t.Run(c.name, func(t *testing.T) {
			r, diags := verifiedDiags(t, c.patch)
			if len(diags) != 1 || diags[0].Path != c.path || diags[0].Severity != SeverityError {
				t.Fatalf("diags = %v", diagList(diags))
			}
			if cp, _ := r.Spec.Checkpoints.Get("cp-2"); cp.Verify == nil || cp.Verify.ExpectOrPass() != VerifyPass || len(cp.Verify.Kinds) != 2 {
				t.Fatalf("cp-2 verify = %+v, want the parent value", cp.Verify)
			}
		})
	}
	harden := []string{
		"checkpoints:\n  cp-2: { verify: { from: stack, kinds: [build, test, lint], expect: pass, timeout: 10m, must_touch: [\"*_test.go\"] } }\n",
		"checkpoints:\n  cp-3: { verify: { cmd: [\"make check\", \"make lint\"] } }\n",
		"checkpoints:\n  cp-spec: { when: { paths: [\"docs/specs/**\"], labels: [ux] } }\n",
		"checkpoints:\n  cp-new: { label: New, mode: { manuel: pause, semi-auto: auto, auto: auto }, verify: { from: stack } }\n",
	}
	for _, patch := range harden {
		if _, diags := verifiedDiags(t, patch); diags.HasErrors() {
			t.Fatalf("%s: diags = %v", patch, diagList(diags))
		}
	}
	r, _ := verifiedDiags(t, harden[0])
	cp, _ := r.Spec.Checkpoints.Get("cp-2")
	if !reflect.DeepEqual(cp.Verify.MustTouch, []string{"*_test.go"}) || cp.Verify.TimeoutOrDefault() != 10*time.Minute {
		t.Fatalf("verify = %+v", cp.Verify)
	}
}

func TestValidate_VerifyAndWhen(t *testing.T) {
	doc := vHeader + `risk: write
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow }
  developer: { role: workflow }
outputs:
  - { id: mr, type: merge_request }
checkpoints:
  cp-a:
    label: A
    mode: { manuel: pause, semi-auto: pause, auto: pause }
    verify: { expect: maybe, kinds: [test], in: ../up, timeout: 2h }
  cp-b:
    label: B
    mode: { manuel: pause, semi-auto: auto, auto: auto }
    verify: { from: git, kinds: [deploy], cmd: ["make {{ .x }}", ""], must_touch: [""] }
  cp-c:
    label: C
    mandatory: true
    mode: { manuel: pause, semi-auto: conditional, auto: auto }
    when: { entry: [ghost], outputs: [nope, mr], paths: [""] }
  cp-d:
    label: D
    mode: { manuel: pause, semi-auto: auto, auto: auto }
    when: {}
`
	diags := checkYAML(t, doc, testEnv())
	want := map[string]string{
		"checkpoints.cp-a.verify":               "verify_empty",
		"checkpoints.cp-a.verify.kinds":         "field_ignored",
		"checkpoints.cp-a.verify.expect":        "enum_invalid",
		"checkpoints.cp-a.verify.in":            "verify_in_invalid",
		"checkpoints.cp-a.verify.timeout":       "verify_timeout_invalid",
		"checkpoints.cp-b.verify.from":          "enum_invalid",
		"checkpoints.cp-b.verify.kinds[0]":      "enum_invalid",
		"checkpoints.cp-b.verify.cmd[0]":        "verify_cmd_template",
		"checkpoints.cp-b.verify.cmd[1]":        "field_required",
		"checkpoints.cp-b.verify.must_touch[0]": "field_required",
		"checkpoints.cp-c.when":                 "when_mandatory",
		"checkpoints.cp-c.when.entry[0]":        "when_unknown_agent",
		"checkpoints.cp-c.when.outputs[0]":      "when_unknown_output",
		"checkpoints.cp-c.when.paths[0]":        "field_required",
		"checkpoints.cp-d.when":                 "when_empty",
	}
	got := map[string]string{}
	for _, d := range diags {
		if strings.Contains(d.Path, ".verify") || strings.Contains(d.Path, ".when") {
			got[d.Path] = d.Code
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v\nall: %v", got, want, diagList(diags))
	}
	for _, d := range diags {
		if d.Code == "checkpoint_condition_missing" {
			t.Fatalf("a conditional checkpoint with when: needs no condition text: %v", d)
		}
	}
}

// CP-R2: 2 human decisions plus 1 per checkpoint that unlocks an
// irreversible action, in semi-auto; warned by validate.
func TestPauseBudget(t *testing.T) {
	doc := func(cps string) string {
		return vHeader + "risk: write\nentry: { agent: orchestrator-dev }\nagents:\n  orchestrator-dev: { role: workflow }\ncheckpoints:\n" + cps
	}
	cp := func(id, semi string, extra string) string {
		return "  " + id + ": { label: X, mode: { manuel: pause, semi-auto: " + semi + ", auto: auto }" + extra + " }\n"
	}
	within := cp("cp-a", "pause", "") + cp("cp-b", "conditional", "") + cp("cp-c", "pause", ", unlocks: [commit]") + cp("cp-d", "auto", "") + cp("cp-e", "skip", "")
	r := resolveYAML(t, doc(within))
	c := r.Spec.PauseBudget()
	if c.Mode != ModeSemiAuto || !reflect.DeepEqual(c.Pauses, []string{"cp-a", "cp-b", "cp-c"}) || c.Budget != 3 || c.Exceeded() {
		t.Fatalf("count = %+v", c)
	}
	if codes := checkYAML(t, doc(within), testEnv()).Codes(); containsStr(codes, "pause_budget_exceeded") {
		t.Fatalf("codes = %v", codes)
	}
	over := within + cp("cp-f", "auto", ", when: { labels: [x] }")
	diags := checkYAML(t, doc(over), testEnv())
	found := false
	for _, d := range diags {
		if d.Code == "pause_budget_exceeded" {
			found = true
			if d.Severity != SeverityWarning || !strings.Contains(d.Message, "cp-f") {
				t.Fatalf("diag = %+v", d)
			}
		}
	}
	if !found {
		t.Fatalf("diags = %v", diagList(diags))
	}
	// Without semi-auto, the default mode is counted.
	manual := vHeader + "risk: write\nentry: { agent: orchestrator-dev }\nagents:\n  orchestrator-dev: { role: workflow }\nmodes: { default: manuel, allowed: [manuel] }\ncheckpoints:\n" +
		"  cp-a: { label: A, mode: { manuel: pause } }\n  cp-b: { label: B, mode: { manuel: pause }, unlocks: [apply] }\n"
	if c := resolveYAML(t, manual).Spec.PauseBudget(); c.Mode != ModeManual || len(c.Pauses) != 2 || c.Exceeded() {
		t.Fatalf("count = %+v", c)
	}
}

func resolveYAML(t *testing.T, doc string) *Resolved {
	t.Helper()
	r, diags := ResolveSpec(catalogOf(t, map[string]string{"hub:x": doc}), Ref{LayerHub, "x"}, nil)
	if r == nil || diags.HasErrors() {
		t.Fatalf("resolution: %v", diagList(diags))
	}
	return r
}

func TestVerifySpecDefaults(t *testing.T) {
	var v *VerifySpec
	if v.ExpectOrPass() != VerifyPass || v.TimeoutOrDefault() != DefaultVerifyTimeout || !reflect.DeepEqual(v.EffectiveKinds(), DefaultVerifyKinds) {
		t.Fatal("defaults")
	}
	if (&VerifySpec{Timeout: "2h"}).TimeoutOrDefault() != MaxVerifyTimeout {
		t.Fatal("timeout capped")
	}
	r := resolveYAML(t, vHeader+"risk: write\ncheckpoints:\n  cp-1: { label: A, mode: { manuel: pause }, verify: { cmd: [a] }, when: { labels: [x] } }\n")
	c := r.Spec.Clone()
	cp, _ := c.Checkpoints.Get("cp-1")
	cp.Verify.Cmd[0], cp.When.Labels[0] = "b", "y"
	orig, _ := r.Spec.Checkpoints.Get("cp-1")
	if orig.Verify.Cmd[0] != "a" || orig.When.Labels[0] != "x" {
		t.Fatal("Clone shares verify/when")
	}
}
