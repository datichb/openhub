package workflow

import (
	"reflect"
	"strings"
	"testing"
)

// SCR: `agents.<id>.scratch: { readers: […] }`, the readers are other
// agents of the workflow.
func TestValidate_Scratch(t *testing.T) {
	base := "risk: write\nentry: { agent: orchestrator-dev }\nagents:\n  orchestrator-dev: { role: workflow, calls: [developer, reviewer] }\n  reviewer: { role: workflow }\n"
	cases := []struct {
		name, agent, code, path string
	}{
		{"readers", "  developer: { role: workflow, scratch: { readers: [reviewer] } }\n", "", ""},
		{"owner only", "  developer: { role: workflow, scratch: {} }\n", "", ""},
		{"reader unknown", "  developer: { role: workflow, scratch: { readers: [ghost] } }\n", "scratch_reader_unknown", "agents.developer.scratch.readers[0]"},
		{"reader not member", "  developer: { role: workflow, scratch: { readers: [documentarian] } }\n", "scratch_reader_unknown", "agents.developer.scratch.readers[0]"},
		{"reader owner", "  developer: { role: workflow, scratch: { readers: [developer] } }\n", "scratch_reader_owner", "agents.developer.scratch.readers[0]"},
		{"reader twice", "  developer: { role: workflow, scratch: { readers: [reviewer, reviewer] } }\n", "duplicate_entry", "agents.developer.scratch.readers[1]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, diags := checkDoc(t, base+c.agent)
			if c.code == "" {
				if diags.HasErrors() {
					t.Fatalf("diags = %v", diagList(diags))
				}
				if got := r.Spec.ScratchOwners(); !reflect.DeepEqual(got, []string{"developer"}) {
					t.Fatalf("owners = %v", got)
				}
				return
			}
			for _, d := range diags {
				if d.Code == c.code && (c.path == "" || d.Path == c.path) {
					return
				}
			}
			t.Fatalf("diags = %v, want %s at %s", diagList(diags), c.code, c.path)
		})
	}
}

// SCR: a layer may narrow the readers or remove the folder, not add a
// reader; a disabled agent leaves the readers.
func TestPatch_Scratch(t *testing.T) {
	hub := strings.Replace(hubLayered, "  developer: { role: workflow, after: cp-1 }\n",
		"  developer: { role: workflow, after: cp-1, scratch: { readers: [reviewer] } }\n", 1)
	diagsOf := func(patch string) (*Resolved, Diagnostics) {
		cat := catalogOf(t, map[string]string{
			"hub:layered":  hub,
			"team:layered": "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: hub:layered\n" + patch,
		})
		return ResolveSpec(cat, Ref{LayerTeam, "layered"}, nil)
	}
	scratchOf := func(r *Resolved, id string) *ScratchSpec {
		a, _ := r.Spec.Agents.Get(id)
		return a.Scratch
	}

	r, diags := diagsOf("agents:\n  developer: { scratch: { readers: [reviewer, orchestrator-dev] } }\n")
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"loosening"}) || diags[0].Path != "agents.developer.scratch.readers" {
		t.Fatalf("widened: %v", diagList(diags))
	}
	if sc := scratchOf(r, "developer"); sc == nil || !reflect.DeepEqual(sc.Readers, []string{"reviewer"}) {
		t.Fatalf("parent readers not kept: %+v", sc)
	}

	r, diags = diagsOf("agents:\n  developer: { scratch: {} }\n")
	if len(diags) != 0 || scratchOf(r, "developer") == nil || len(scratchOf(r, "developer").Readers) != 0 {
		t.Fatalf("narrowed: %v %+v", diagList(diags), scratchOf(r, "developer"))
	}

	r, diags = diagsOf("agents:\n  developer: { scratch: null }\n")
	if len(diags) != 0 || scratchOf(r, "developer") != nil {
		t.Fatalf("removed: %v %+v", diagList(diags), scratchOf(r, "developer"))
	}

	r, diags = diagsOf("agents:\n  reviewer: { scratch: { readers: [developer] } }\n")
	if len(diags) != 0 || scratchOf(r, "reviewer") == nil {
		t.Fatalf("added: %v", diagList(diags))
	}

	r, diags = diagsOf("agents:\n  reviewer: { role: disabled }\n")
	if len(diags) != 0 || scratchOf(r, "developer") == nil || len(scratchOf(r, "developer").Readers) != 0 {
		t.Fatalf("disabled reader: %v %+v", diagList(diags), scratchOf(r, "developer"))
	}
	parent, _ := catalogOf(t, map[string]string{"hub:layered": hub}).Lookup(Ref{LayerHub, "layered"})
	if a, _ := parent.Spec.Agents.Get("developer"); a.Scratch == nil || len(a.Scratch.Readers) != 1 {
		t.Fatal("parent mutated")
	}
}

func checkDoc(t *testing.T, body string) (*Resolved, Diagnostics) {
	t.Helper()
	cat := catalogOf(t, map[string]string{"hub:x": vHeader + body})
	r, diags := Check(cat, Ref{LayerHub, "x"}, nil, testEnv())
	if r == nil && !diags.HasErrors() {
		t.Fatalf("resolution failed: %v", diags)
	}
	return r, diags
}
