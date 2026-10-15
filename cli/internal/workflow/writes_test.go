package workflow

import (
	"reflect"
	"strings"
	"testing"
)

// R4 (E8): `agents.<id>.writes` opens documentation paths only: relative,
// under docs/, a file or a folder ending in /**.
func TestWritesPathProblem(t *testing.T) {
	for p, ok := range map[string]bool{
		"docs/wiki/specs/**":      true,
		"docs/wiki/specs/auth.md": true,
		"docs/adr/**":             true,
		"docs/":                   false,
		"docs":                    false,
		"src/**":                  false,
		"/docs/wiki/**":           false,
		"docs/../src/**":          false,
		"docs/wiki/../../x":       false,
		"docs/wiki/*.md":          false,
		"docs/**/x.md":            false,
		"docs/wiki//specs/**":     false,
		`docs\wiki\**`:            false,
		"":                        false,
		// p6-debts: named root files, never a folder nor code
		"ONBOARDING.md":       true,
		"CONVENTIONS.md":      true,
		"README.md":           true,
		"CHANGELOG.md":        true,
		"ONBOARDING.md/**":    false,
		"README.md/x":         false,
		"main.go":             false,
		"Makefile":            false,
		"readme.md":           false,
		"./README.md":         false,
		"CHANGELOG.md/../src": false,
		"src/README.md":       false,
	} {
		if got := WritesPathProblem(p) == ""; got != ok {
			t.Errorf("WritesPathProblem(%q) = %q, want valid=%v", p, WritesPathProblem(p), ok)
		}
	}
}

func TestWritesCovered(t *testing.T) {
	parent := []string{"docs/wiki/specs/**", "docs/adr/001.md"}
	for p, ok := range map[string]bool{
		"docs/wiki/specs/**":      true,
		"docs/wiki/specs/team/**": true,
		"docs/wiki/specs/auth.md": true,
		"docs/adr/001.md":         true,
		"docs/adr/**":             false,
		"docs/wiki/**":            false,
		"docs/wiki/specs-x/**":    false,
		"docs/adr/002.md":         false,
	} {
		if got := WritesCovered(p, parent); got != ok {
			t.Errorf("WritesCovered(%q) = %v, want %v", p, got, ok)
		}
	}
}

func TestValidate_Writes(t *testing.T) {
	base := "risk: plan\nbeads: { allow: [show] }\nagents:\n  orchestrator-dev: { role: workflow, calls: [reviewer] }\n"
	cases := []struct {
		name, risk, agent, code, path string
	}{
		{"folder", "", "  reviewer: { role: workflow, writes: [docs/wiki/specs/**] }\n", "", ""},
		{"file", "", "  reviewer: { role: workflow, writes: [docs/wiki/specs/a.md] }\n", "", ""},
		{"outside docs", "", "  reviewer: { role: workflow, writes: [src/**] }\n", "writes_path_invalid", "agents.reviewer.writes[0]"},
		{"wildcard", "", "  reviewer: { role: workflow, writes: [docs/*.md] }\n", "writes_path_invalid", "agents.reviewer.writes[0]"},
		{"twice", "", "  reviewer: { role: workflow, writes: [docs/a.md, docs/a.md] }\n", "duplicate_entry", "agents.reviewer.writes[1]"},
		{"read-only workflow", "risk: read\n", "  reviewer: { role: workflow, writes: [docs/a.md] }\n", "writes_read_only", "agents.reviewer.writes"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := base
			if c.risk != "" {
				doc = strings.Replace(doc, "risk: plan\n", c.risk, 1)
			}
			doc = "entry: { agent: orchestrator-dev }\n" + doc + c.agent
			r, diags := checkDoc(t, doc)
			if c.code == "" {
				if diags.HasErrors() {
					t.Fatalf("diags = %v", diagList(diags))
				}
				if got := r.Spec.WritesOf("reviewer"); len(got) != 1 {
					t.Fatalf("writes = %v", got)
				}
				return
			}
			for _, d := range diags {
				if d.Code == c.code && d.Path == c.path {
					return
				}
			}
			t.Fatalf("diags = %v, want %s at %s", diagList(diags), c.code, c.path)
		})
	}
}

// R4 (E8): a layer narrows `writes:`, never widens it: a path the parent
// does not cover, or `writes:` on an agent without (inherited or added),
// is a loosening; the parent's list is kept.
func TestPatch_Writes(t *testing.T) {
	hub := strings.Replace(hubLayered, "  developer: { role: workflow, after: cp-1 }\n",
		"  developer: { role: workflow, after: cp-1, writes: [docs/wiki/specs/**] }\n", 1)
	resolve := func(patch string) (*Resolved, Diagnostics) {
		cat := catalogOf(t, map[string]string{
			"hub:layered":  hub,
			"team:layered": "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: hub:layered\n" + patch,
		})
		return ResolveSpec(cat, Ref{LayerTeam, "layered"}, nil)
	}
	writesOf := func(r *Resolved, id string) []string {
		a, _ := r.Spec.Agents.Get(id)
		return a.Writes
	}

	r, diags := resolve("agents:\n  developer: { writes: [docs/wiki/specs/team/**] }\n")
	if len(diags) != 0 || !reflect.DeepEqual(writesOf(r, "developer"), []string{"docs/wiki/specs/team/**"}) {
		t.Fatalf("narrowed: %v %v", diagList(diags), writesOf(r, "developer"))
	}
	r, diags = resolve("agents:\n  developer: { writes: [] }\n")
	if len(diags) != 0 || len(writesOf(r, "developer")) != 0 {
		t.Fatalf("removed: %v %v", diagList(diags), writesOf(r, "developer"))
	}

	for name, patch := range map[string]string{
		"wider":           "agents:\n  developer: { writes: [docs/wiki/**] }\n",
		"other folder":    "agents:\n  developer: { writes: [docs/wiki/specs/**, docs/adr/**] }\n",
		"agent without":   "agents:\n  reviewer: { writes: [docs/wiki/specs/**] }\n",
		"agent added":     "agents:\n  documentarian: { role: workflow, writes: [docs/wiki/specs/**] }\n",
		"source code too": "agents:\n  developer: { writes: [docs/wiki/specs/**, src/**] }\n",
	} {
		r, diags := resolve(patch)
		if got := diags.Codes(); len(got) == 0 || got[0] != "loosening" || !strings.HasSuffix(diags[0].Path, ".writes") {
			t.Errorf("%s: %v", name, diagList(diags))
			continue
		}
		if got := writesOf(r, "developer"); !reflect.DeepEqual(got, []string{"docs/wiki/specs/**"}) {
			t.Errorf("%s: parent writes not kept: %v", name, got)
		}
		for _, id := range []string{"reviewer", "documentarian"} {
			if got := writesOf(r, id); len(got) != 0 {
				t.Errorf("%s: %s got writes %v", name, id, got)
			}
		}
	}
	parent, _ := catalogOf(t, map[string]string{"hub:layered": hub}).Lookup(Ref{LayerHub, "layered"})
	if a, _ := parent.Spec.Agents.Get("developer"); len(a.Writes) != 1 {
		t.Fatal("parent mutated")
	}
}

// R4: `verify.spec` is a verification on its own (no from nor cmd needed);
// a layer keeps its paths and breakdown.
func TestVerifySpecCheck(t *testing.T) {
	doc := "risk: plan\nbeads: { allow: [show] }\nentry: { agent: orchestrator-dev }\nagents:\n  orchestrator-dev: { role: workflow }\ncheckpoints:\n"
	_, diags := checkDoc(t, doc+"  cp-spec: { mode: { manuel: pause, semi-auto: pause, auto: pause }, verify: { spec: { paths: [docs/wiki/specs/**] } } }\n")
	if diags.HasErrors() {
		t.Fatalf("spec alone: %v", diagList(diags))
	}
	_, diags = checkDoc(t, doc+"  cp-spec: { mode: { manuel: pause, semi-auto: pause, auto: pause }, verify: { spec: { paths: [] } } }\n")
	if !hasCode(diags, "field_required") {
		t.Fatalf("no paths: %v", diagList(diags))
	}
	_, diags = checkDoc(t, doc+"  cp-spec: { mode: { manuel: pause, semi-auto: pause, auto: pause }, verify: { spec: { paths: [../x/**] } } }\n")
	if !hasCode(diags, "verify_spec_path_invalid") {
		t.Fatalf("outside: %v", diagList(diags))
	}

	parent := &VerifySpec{Spec: &SpecCheck{Paths: []string{"docs/wiki/specs/**"}, Breakdown: true}}
	for name, c := range map[string]struct {
		v    *VerifySpec
		want bool
	}{
		"same":         {&VerifySpec{Spec: &SpecCheck{Paths: []string{"docs/wiki/specs/**"}, Breakdown: true}}, true},
		"more paths":   {&VerifySpec{Spec: &SpecCheck{Paths: []string{"docs/wiki/specs/**", "docs/x/**"}, Breakdown: true}}, true},
		"no breakdown": {&VerifySpec{Spec: &SpecCheck{Paths: []string{"docs/wiki/specs/**"}}}, false},
		"other paths":  {&VerifySpec{Spec: &SpecCheck{Paths: []string{"docs/x/**"}, Breakdown: true}}, false},
		"removed":      {&VerifySpec{From: VerifyFromStack}, false},
	} {
		if got := c.v.Covers(parent); got != c.want {
			t.Errorf("%s: Covers = %v, want %v", name, got, c.want)
		}
	}
	if cl := parent.clone(); cl.Spec == parent.Spec || !reflect.DeepEqual(cl.Spec, parent.Spec) {
		t.Fatal("clone shares the spec check")
	}
}

func hasCode(diags Diagnostics, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}
