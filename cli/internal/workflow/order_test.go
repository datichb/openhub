package workflow

import (
	"strings"
	"testing"
)

// r5-fixes: `writes_after` names a checkpoint of every session and needs
// `writes:`; the `after` of a checkpoint names one declared before it.
func TestValidate_Order(t *testing.T) {
	base := "risk: plan\nbeads: { allow: [show] }\nentry: { agent: orchestrator-dev }\ninputs:\n  method: { type: enum, values: [standard, tdd], default: standard }\n" +
		"agents:\n  orchestrator-dev: { role: workflow, calls: [reviewer] }\n"
	cps := "checkpoints:\n" +
		"  cp-a: { mode: { manuel: pause, semi-auto: pause, auto: auto } }\n" +
		"  cp-t: { method: tdd, mode: { manuel: pause, semi-auto: pause, auto: auto } }\n" +
		"  cp-b: { %s mode: { manuel: pause, semi-auto: pause, auto: auto } }\n"
	cases := []struct {
		name, agent, cpB, code, path string
	}{
		{"writes after a checkpoint", "writes: [docs/wiki/specs/**], writes_after: cp-a", "after: cp-a,", "", ""},
		{"unknown checkpoint", "writes: [docs/wiki/specs/**], writes_after: cp-x", "", "writes_after_unknown", "agents.reviewer.writes_after"},
		{"an agent is not a checkpoint", "writes: [docs/wiki/specs/**], writes_after: orchestrator-dev", "", "writes_after_unknown", "agents.reviewer.writes_after"},
		{"without writes", "writes_after: cp-a", "", "writes_after_without_writes", "agents.reviewer.writes_after"},
		{"writes after a method checkpoint", "writes: [docs/wiki/specs/**], writes_after: cp-t", "", "order_method_after", "agents.reviewer.writes_after"},
		{"checkpoint after a later one", "", "", "checkpoint_after_order", "checkpoints.cp-a.after"},
		{"checkpoint after itself", "", "after: cp-b,", "checkpoint_after_order", "checkpoints.cp-b.after"},
		{"checkpoint after an unknown one", "", "after: cp-x,", "checkpoint_after_unknown", "checkpoints.cp-b.after"},
		{"checkpoint after a method checkpoint", "", "after: cp-t,", "order_method_after", "checkpoints.cp-b.after"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := base + "  reviewer: { role: workflow"
			if c.agent != "" {
				doc += ", " + c.agent
			}
			doc += " }\n" + strings.Replace(cps, "%s", c.cpB, 1)
			if c.name == "checkpoint after a later one" {
				doc = strings.Replace(doc, "  cp-a: { mode", "  cp-a: { after: cp-b, mode", 1)
			}
			r, diags := checkDoc(t, doc)
			if c.code == "" {
				if diags.HasErrors() {
					t.Fatalf("diags = %v", diagList(diags))
				}
				a, _ := r.Spec.Agents.Get("reviewer")
				cp, _ := r.Spec.Checkpoints.Get("cp-b")
				if a.WritesAfter != "cp-a" || cp.After != "cp-a" {
					t.Fatalf("writes_after = %q, after = %q", a.WritesAfter, cp.After)
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

// r5-fixes: a layer adds an order or moves it to a later checkpoint, never
// removes it nor moves it earlier (loosening, the parent's kept).
func TestPatch_Order(t *testing.T) {
	hub := strings.Replace(hubLayered, "  developer: { role: workflow, after: cp-1 }\n",
		"  developer: { role: workflow, after: cp-1, writes: [docs/wiki/specs/**], writes_after: cp-2 }\n", 1)
	hub = strings.Replace(hub, "  cp-3: { label: End,", "  cp-3: { label: End, after: cp-2,", 1)
	resolve := func(patch string) (*Resolved, Diagnostics) {
		cat := catalogOf(t, map[string]string{
			"hub:layered":  hub,
			"team:layered": "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: hub:layered\n" + patch,
		})
		return ResolveSpec(cat, Ref{LayerTeam, "layered"}, nil)
	}
	order := func(r *Resolved) (string, string) {
		a, _ := r.Spec.Agents.Get("developer")
		cp, _ := r.Spec.Checkpoints.Get("cp-3")
		return a.WritesAfter, cp.After
	}

	r, diags := resolve("agents:\n  developer: { writes_after: cp-3 }\ncheckpoints:\n  cp-2: { after: cp-1 }\n")
	if len(diags) != 0 {
		t.Fatalf("hardened: %v", diagList(diags))
	}
	if w, _ := order(r); w != "cp-3" {
		t.Fatalf("writes_after moved later: %q", w)
	}
	if cp, _ := r.Spec.Checkpoints.Get("cp-2"); cp.After != "cp-1" {
		t.Fatalf("after added: %q", cp.After)
	}

	for name, patch := range map[string]string{
		"writes_after removed":  "agents:\n  developer: { writes_after: \"\" }\n",
		"writes_after earlier":  "agents:\n  developer: { writes_after: cp-1 }\n",
		"checkpoint after gone": "checkpoints:\n  cp-3: { after: \"\" }\n",
		"checkpoint earlier":    "checkpoints:\n  cp-3: { after: cp-1 }\n",
	} {
		r, diags := resolve(patch)
		if got := diags.Codes(); len(got) != 1 || got[0] != "loosening" {
			t.Errorf("%s: %v", name, diagList(diags))
			continue
		}
		if w, a := order(r); w != "cp-2" || a != "cp-2" {
			t.Errorf("%s: parent order not kept: writes_after %q, after %q", name, w, a)
		}
	}

	// writes: [] leaves nothing to hold.
	r, diags = resolve("agents:\n  developer: { writes: [] }\n")
	if w, _ := order(r); len(diags) != 0 || w != "" {
		t.Fatalf("writes removed: %v %q", diagList(diags), w)
	}
}
