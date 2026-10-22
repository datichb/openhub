package workflow

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// N24 (audit-v5 S-16 → S-20): a layer may only harden its parent.

const hubLayered = `apiVersion: oh/v1
kind: Workflow
id: layered
risk: write
entry: { agent: orchestrator-dev }
agents:
  orchestrator-dev: { role: workflow, calls: [developer, reviewer] }
  developer: { role: workflow, after: cp-1 }
  reviewer: { role: workflow, after: cp-2 }
checkpoints:
  cp-1: { label: Start, mode: { manuel: pause, semi-auto: auto, auto: auto } }
  cp-2: { label: Commit, unlocks: [commit], mode: { manuel: pause, semi-auto: pause, auto: auto } }
  cp-3: { label: End, mode: { manuel: pause, semi-auto: pause, auto: auto } }
circuit_breaker: { max_consecutive_subagents: 8 }
plugins: [rtk]
mcp: [gitlab]
`

func layeredDiags(t *testing.T, patch string) (*Resolved, Diagnostics) {
	t.Helper()
	cat := catalogOf(t, map[string]string{
		"hub:layered":  hubLayered,
		"team:layered": "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: hub:layered\n" + patch,
	})
	return ResolveSpec(cat, Ref{LayerTeam, "layered"}, nil)
}

// PoC S-16: the parent has no `beads:` block (the gateway applies the
// read-only default list); the child may not grant writes.
func TestPatch_BeadsWithoutParentBlockComparedToDefault(t *testing.T) {
	t.Parallel()
	r, diags := layeredDiags(t, "beads: { allow: [show, create, close, delete] }\n")
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"loosening"}) || diags[0].Path != "beads.allow" {
		t.Fatalf("diags = %v", diagList(diags))
	}
	if r.Spec.Beads != nil {
		t.Fatalf("beads = %+v, want the parent value (none)", r.Spec.Beads)
	}
	// A subset of the default list hardens it.
	r, diags = layeredDiags(t, "beads: { allow: [show, list] }\n")
	if len(diags) != 0 || !reflect.DeepEqual(r.Spec.Beads.Allow, []string{"show", "list"}) {
		t.Fatalf("diags = %v, beads = %+v", diagList(diags), r.Spec.Beads)
	}
}

// PoC S-17: a checkpoint that unlocks an operation, even not mandatory, may
// not be skipped (the lock would disappear).
func TestPatch_UnlockingCheckpointCannotBeSkipped(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"manuel", "semi-auto", "auto"} {
		r, diags := layeredDiags(t, "checkpoints:\n  cp-2: { mode: { "+mode+": skip } }\n")
		if got := diags.Codes(); !reflect.DeepEqual(got, []string{"loosening"}) || diags[0].Path != "checkpoints.cp-2.mode."+mode {
			t.Fatalf("%s: diags = %v", mode, diagList(diags))
		}
		if cp, _ := r.Spec.Checkpoints.Get("cp-2"); cp.Behavior(mode) == BehaviorSkip {
			t.Fatalf("%s: skip applied", mode)
		}
	}
	_, diags := layeredDiags(t, "checkpoints:\n  cp-2: { mandatory: false }\n")
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"loosening"}) {
		t.Fatalf("mandatory: false: %v", diagList(diags))
	}
	// A checkpoint without unlocks stays relaxable.
	if _, diags := layeredDiags(t, "checkpoints:\n  cp-3: { mode: { auto: skip } }\n"); len(diags) != 0 {
		t.Fatalf("cp-3: %v", diagList(diags))
	}
}

func TestPatch_HardenOnlyTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, patch, path string // path "" = accepted
	}{
		{"plugin added", "plugins: [rtk, context-mode]\n", "plugins"},
		{"plugin removed", "plugins: []\n", ""},
		{"mcp added", "mcp: [gitlab, jira]\n", "mcp"},
		{"mcp narrowed", "mcp: []\n", ""},
		{"after removed", "agents:\n  developer: { after: \"\" }\n", "agents.developer.after"},
		{"after earlier", "agents:\n  reviewer: { after: cp-1 }\n", "agents.reviewer.after"},
		{"after on an agent", "agents:\n  reviewer: { after: developer }\n", "agents.reviewer.after"},
		{"after later", "agents:\n  developer: { after: cp-2 }\n", ""},
		{"after added", "agents:\n  orchestrator-dev: { after: cp-1 }\n", ""},
		{"calls widened", "agents:\n  orchestrator-dev: { calls: [developer, reviewer, designer] }\n", "agents.orchestrator-dev.calls"},
		{"calls narrowed", "agents:\n  orchestrator-dev: { calls: [developer] }\n", ""},
		{"breaker raised", "circuit_breaker: { max_consecutive_subagents: 20 }\n", "circuit_breaker.max_consecutive_subagents"},
		{"breaker disabled", "circuit_breaker: { max_consecutive_subagents: 0 }\n", "circuit_breaker.max_consecutive_subagents"},
		{"breaker lowered", "circuit_breaker: { max_consecutive_subagents: 4 }\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, diags := layeredDiags(t, c.patch)
			if c.path == "" {
				if len(diags) != 0 {
					t.Fatalf("hardening refused: %v", diagList(diags))
				}
				return
			}
			if got := diags.Codes(); !reflect.DeepEqual(got, []string{"loosening"}) || diags[0].Path != c.path {
				t.Fatalf("diags = %v, want loosening at %s", diagList(diags), c.path)
			}
		})
	}
}

// S-20: a team layer that widens a workflow, by adding an agent or skills to
// `skills.extra`, gets a warning: refusing would break the agents of the team
// catalogue (a documented use), so the addition is accepted and reported. A
// project layer (repository content) is refused (see
// TestPatch_ProjectLayerWideningIsRefused). Removing skills stays allowed and
// silent; the hub layer adds freely.
func TestPatch_LayerWideningIsWarned(t *testing.T) {
	t.Parallel()
	hub := hubLayered + "skills: { extra: [a/one, a/two] }\n"
	resolve := func(layer, patch string) (*Resolved, Diagnostics) {
		docs := map[string]string{
			"hub:layered":  hub,
			"team:layered": "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: hub:layered\n" + patch,
		}
		ref := Ref{LayerTeam, "layered"}
		switch layer {
		case "project":
			docs["team:layered"] = "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: hub:layered\n"
			docs["project:layered"] = "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: team:layered\n" + patch
			ref = Ref{LayerProject, "layered"}
		case "hub":
			docs = map[string]string{
				"hub:layered": hub,
				"hub:child":   "apiVersion: oh/v1\nkind: Workflow\nid: child\nextends: hub:layered\n" + patch,
			}
			ref = Ref{LayerHub, "child"}
		}
		return ResolveSpec(catalogOf(t, docs), ref, nil)
	}
	warnings := func(diags Diagnostics, code string) []Diagnostic {
		var out []Diagnostic
		for _, d := range diags {
			if d.Code == code {
				out = append(out, d)
			}
		}
		return out
	}

	t.Run("agent added by a team layer", func(t *testing.T) {
		r, diags := resolve("team", "agents:\n  security-bot: { role: workflow, after: cp-1 }\n")
		w := warnings(diags, "patch_agent_added")
		if len(w) != 1 || w[0].Severity != SeverityWarning || w[0].Path != "agents.security-bot" || !strings.Contains(w[0].Message, "security-bot") {
			t.Fatalf("diags = %v", diagList(diags))
		}
		if diags.HasErrors() {
			t.Fatalf("a warning, not an error: %v", diagList(diags))
		}
		if _, ok := r.Spec.Agents.Get("security-bot"); !ok {
			t.Fatal("the agent is added anyway")
		}
	})
	t.Run("agent added with writes: warning and loosening", func(t *testing.T) {
		r, diags := resolve("team", "agents:\n  security-bot: { role: workflow, writes: [docs/**] }\n")
		if got := diags.Codes(); !reflect.DeepEqual(got, []string{"loosening", "patch_agent_added"}) {
			t.Fatalf("diags = %v", diagList(diags))
		}
		if a, _ := r.Spec.Agents.Get("security-bot"); len(a.Writes) != 0 {
			t.Fatalf("writes kept: %v", a.Writes)
		}
	})
	t.Run("existing agent changed: nothing to report", func(t *testing.T) {
		if _, diags := resolve("team", "agents:\n  developer: { after: cp-2 }\n"); len(diags) != 0 {
			t.Fatalf("diags = %v", diagList(diags))
		}
	})
	t.Run("agent added by the hub layer: silent", func(t *testing.T) {
		if _, diags := resolve("hub", "agents:\n  security-bot: { role: workflow }\n"); len(diags) != 0 {
			t.Fatalf("diags = %v", diagList(diags))
		}
	})

	t.Run("skills added to skills.extra", func(t *testing.T) {
		r, diags := resolve("team", "skills: { extra: [a/one, a/two, shared/x, shared/y] }\n")
		w := warnings(diags, "patch_skills_extra_added")
		if len(w) != 1 || w[0].Severity != SeverityWarning || w[0].Path != "skills.extra" || !strings.Contains(w[0].Message, "shared/x,shared/y") || strings.Contains(w[0].Message, "a/one") {
			t.Fatalf("diags = %v", diagList(diags))
		}
		if diags.HasErrors() || !reflect.DeepEqual(r.Spec.Skills.Extra, []string{"a/one", "a/two", "shared/x", "shared/y"}) {
			t.Fatalf("diags = %v, extra = %v", diagList(diags), r.Spec.Skills.Extra)
		}
	})
	t.Run("skills.extra on a parent without any", func(t *testing.T) {
		cat := catalogOf(t, map[string]string{
			"hub:layered":  hubLayered,
			"team:layered": "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: hub:layered\nskills: { extra: [shared/x] }\n",
		})
		_, diags := ResolveSpec(cat, Ref{LayerTeam, "layered"}, nil)
		if got := diags.Codes(); !reflect.DeepEqual(got, []string{"patch_skills_extra_added"}) {
			t.Fatalf("diags = %v", diagList(diags))
		}
	})
	t.Run("skills removed or kept: silent", func(t *testing.T) {
		for name, patch := range map[string]string{
			"removed":  "skills: { extra: [a/one] }\n",
			"emptied":  "skills: { extra: [] }\n",
			"restated": "skills: { extra: [a/two, a/one] }\n",
			"deny":     "skills: { deny: [a/two] }\n",
		} {
			if _, diags := resolve("team", patch); len(diags) != 0 {
				t.Errorf("%s: diags = %v", name, diagList(diags))
			}
		}
	})
	t.Run("skills added by the hub layer: silent", func(t *testing.T) {
		if _, diags := resolve("hub", "skills: { extra: [a/one, shared/x] }\n"); len(diags) != 0 {
			t.Fatalf("diags = %v", diagList(diags))
		}
	})
}

// S-20: a project layer is repository content, which anyone may edit: it may
// not add an agent nor skills (an error, the parent value is kept), whereas it
// may keep or remove what the parent lists.
func TestPatch_ProjectLayerWideningIsRefused(t *testing.T) {
	t.Parallel()
	hub := hubLayered + "skills: { extra: [a/one, a/two] }\n"
	resolve := func(patch string) (*Resolved, Diagnostics) {
		return ResolveSpec(catalogOf(t, map[string]string{
			"hub:layered":     hub,
			"team:layered":    "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: hub:layered\n",
			"project:layered": "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: team:layered\n" + patch,
		}), Ref{LayerProject, "layered"}, nil)
	}

	t.Run("agent added", func(t *testing.T) {
		r, diags := resolve("agents:\n  security-bot: { role: workflow, after: cp-1 }\n")
		if got := diags.Codes(); !reflect.DeepEqual(got, []string{"patch_agent_refused"}) {
			t.Fatalf("diags = %v", diagList(diags))
		}
		d := diags[0]
		if d.Severity != SeverityError || d.Path != "agents.security-bot" || !strings.Contains(d.Message, "security-bot") || !strings.Contains(d.Message, "team:layered") || d.Hint == "" {
			t.Fatalf("diag = %+v", d)
		}
		if _, ok := r.Spec.Agents.Get("security-bot"); ok {
			t.Fatal("the agent must not be added")
		}
	})
	t.Run("agent added with writes: refused, no extra loosening", func(t *testing.T) {
		_, diags := resolve("agents:\n  security-bot: { role: workflow, writes: [docs/**] }\n")
		if got := diags.Codes(); !reflect.DeepEqual(got, []string{"patch_agent_refused"}) {
			t.Fatalf("diags = %v", diagList(diags))
		}
	})
	t.Run("existing agent changed: silent", func(t *testing.T) {
		if _, diags := resolve("agents:\n  developer: { after: cp-2 }\n"); len(diags) != 0 {
			t.Fatalf("diags = %v", diagList(diags))
		}
	})
	t.Run("skills added", func(t *testing.T) {
		r, diags := resolve("skills: { extra: [a/one, a/two, shared/x] }\n")
		if got := diags.Codes(); !reflect.DeepEqual(got, []string{"patch_skills_extra_refused"}) {
			t.Fatalf("diags = %v", diagList(diags))
		}
		if d := diags[0]; d.Severity != SeverityError || d.Path != "skills.extra" || !strings.Contains(d.Message, "shared/x") || strings.Contains(d.Message, "a/one") {
			t.Fatalf("diag = %+v", d)
		}
		if !reflect.DeepEqual(r.Spec.Skills.Extra, []string{"a/one", "a/two"}) {
			t.Fatalf("extra = %v, want the parent list", r.Spec.Skills.Extra)
		}
	})
	t.Run("skills removed or kept: silent", func(t *testing.T) {
		for name, patch := range map[string]string{
			"removed":  "skills: { extra: [a/one] }\n",
			"emptied":  "skills: { extra: [] }\n",
			"restated": "skills: { extra: [a/two, a/one] }\n",
		} {
			r, diags := resolve(patch)
			if len(diags) != 0 {
				t.Errorf("%s: diags = %v", name, diagList(diags))
			}
			if name == "removed" && !reflect.DeepEqual(r.Spec.Skills.Extra, []string{"a/one"}) {
				t.Errorf("removed: extra = %v", r.Spec.Skills.Extra)
			}
		}
	})
}

// S-20: an agent added by a team layer must still respect `risk:` of the
// resolved workflow: with the agent catalogue, a writing agent added to a
// read-only workflow is an error.
func TestPatch_TeamAddedAgentRespectsRisk(t *testing.T) {
	t.Parallel()
	hub := "apiVersion: oh/v1\nkind: Workflow\nid: ro\nrisk: read\nbeads: { allow: [show] }\nentry: { agent: auditor }\n"
	cat := catalogOf(t, map[string]string{
		"hub:ro":  hub,
		"team:ro": "apiVersion: oh/v1\nkind: Workflow\nid: ro\nextends: hub:ro\nagents:\n  developer: { role: independent }\n  reviewer: { role: independent }\n",
	})
	_, diags := Check(cat, Ref{LayerTeam, "ro"}, nil, Env{Agents: testAgents})
	var codes []string
	for _, d := range diags {
		if d.Code == "read_agent_writes" {
			codes = append(codes, d.Path)
		}
	}
	if !reflect.DeepEqual(codes, []string{"agents.developer"}) {
		t.Fatalf("read_agent_writes at %v, diags = %v", codes, diagList(diags))
	}
}

// Changes a layer may make: a list narrows a parent that has none (every MCP
// server of the project), and the hub layer (oh's own content) may add plugins
// and MCP servers.
func TestPatch_AllowedChanges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		docs    map[string]string
		ref     Ref
		wantMCP []string // nil: not checked
	}{
		{"any mcp list narrows a parent without one",
			map[string]string{
				"hub:quick":  "apiVersion: oh/v1\nkind: Workflow\nid: quick\nrisk: write\n",
				"team:quick": "apiVersion: oh/v1\nkind: Workflow\nid: quick\nextends: hub:quick\nmcp: [gitlab]\n",
			}, Ref{LayerTeam, "quick"}, []string{"gitlab"}},
		{"the hub layer may add plugins and mcp servers",
			map[string]string{
				"hub:base":  strings.Replace(hubLayered, "id: layered", "id: base", 1),
				"hub:child": "apiVersion: oh/v1\nkind: Workflow\nid: child\nextends: hub:base\nplugins: [rtk, context-mode]\nmcp: [gitlab, jira]\n",
			}, Ref{LayerHub, "child"}, []string{"gitlab", "jira"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, diags := ResolveSpec(catalogOf(t, c.docs), c.ref, nil)
			if len(diags) != 0 {
				t.Fatalf("diags = %v", diagList(diags))
			}
			if !reflect.DeepEqual(r.Spec.MCP, c.wantMCP) {
				t.Fatalf("mcp = %v, want %v", r.Spec.MCP, c.wantMCP)
			}
		})
	}
}

// S-19: a read-only workflow may not load a plugin whose tools run commands.
func TestValidate_ReadWorkflowRefusesExecPlugin(t *testing.T) {
	t.Parallel()
	body := "risk: read\nbeads: { allow: [show] }\nplugins: [context-mode@latest]\n"
	assertDiag(t, checkYAML(t, vHeader+body, testEnv()), "read_plugin_exec", "plugins[0]", SeverityError)
	if !strings.Contains(strings.Join(ExecPlugins, ","), "context-mode") {
		t.Fatal("context-mode must be listed")
	}
}

// A disabled agent leaves the explicit delegations of the others (the
// feature orchestrator lists documentarian; a team may disable it).
func TestPatch_DisabledAgentLeavesCalls(t *testing.T) {
	t.Parallel()
	r, diags := layeredDiags(t, "agents:\n  reviewer: { role: disabled }\n")
	if len(diags) != 0 {
		t.Fatalf("diags = %v", diagList(diags))
	}
	od, _ := r.Spec.Agents.Get("orchestrator-dev")
	if !reflect.DeepEqual(od.Calls, []string{"developer"}) {
		t.Fatalf("calls = %v", od.Calls)
	}
	hub, _ := catalogOf(t, map[string]string{"hub:layered": hubLayered}).Lookup(Ref{LayerHub, "layered"})
	if a, _ := hub.Spec.Agents.Get("orchestrator-dev"); len(a.Calls) != 2 {
		t.Fatal("parent mutated")
	}
}

// A refusal of the parent (`skills.deny`) is a security field: outside the
// hub layer the lists are combined, a layer never lifts a refusal.
func TestPatch_SkillsDenyIsUnioned(t *testing.T) {
	t.Parallel()
	hub := hubLayered + "skills: { deny: [a/one, a/two] }\n"
	resolve := func(layer, patch string) (*Resolved, Diagnostics) {
		docs := map[string]string{
			"hub:layered":  hub,
			"team:layered": "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: hub:layered\n" + patch,
		}
		ref := Ref{LayerTeam, "layered"}
		switch layer {
		case "project":
			docs["team:layered"] = "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: hub:layered\n"
			docs["project:layered"] = "apiVersion: oh/v1\nkind: Workflow\nid: layered\nextends: team:layered\n" + patch
			ref = Ref{LayerProject, "layered"}
		case "hub":
			docs = map[string]string{
				"hub:layered": hub,
				"hub:child":   "apiVersion: oh/v1\nkind: Workflow\nid: child\nextends: hub:layered\n" + patch,
			}
			ref = Ref{LayerHub, "child"}
		}
		return ResolveSpec(catalogOf(t, docs), ref, nil)
	}

	for _, layer := range []string{"team", "project"} {
		t.Run(layer+": added refusals are combined", func(t *testing.T) {
			r, diags := resolve(layer, "skills: { deny: [a/one, a/two, b/three] }\n")
			if len(diags) != 0 || !reflect.DeepEqual(r.Spec.Skills.Deny, []string{"a/one", "a/two", "b/three"}) {
				t.Fatalf("diags = %v, deny = %v", diagList(diags), r.Spec.Skills.Deny)
			}
		})
		t.Run(layer+": a list that only adds a new refusal omits the parent's", func(t *testing.T) {
			// Refused, and the parent's refusals are kept with the new one.
			r, diags := resolve(layer, "skills: { deny: [b/three] }\n")
			if got := diags.Codes(); !reflect.DeepEqual(got, []string{"loosening"}) || diags[0].Path != "skills.deny" {
				t.Fatalf("diags = %v", diagList(diags))
			}
			if !reflect.DeepEqual(r.Spec.Skills.Deny, []string{"a/one", "a/two", "b/three"}) {
				t.Fatalf("deny = %v", r.Spec.Skills.Deny)
			}
		})
		t.Run(layer+": lifting a refusal is a loosening", func(t *testing.T) {
			for name, patch := range map[string]string{
				"removed one": "skills: { deny: [a/one] }\n",
				"emptied":     "skills: { deny: [] }\n",
				"swapped":     "skills: { deny: [a/one, a/other] }\n",
			} {
				r, diags := resolve(layer, patch)
				if got := diags.Codes(); !reflect.DeepEqual(got, []string{"loosening"}) || diags[0].Path != "skills.deny" || !diags.HasErrors() {
					t.Errorf("%s: diags = %v", name, diagList(diags))
				}
				for _, kept := range []string{"a/one", "a/two"} {
					if !slices.Contains(r.Spec.Skills.Deny, kept) {
						t.Errorf("%s: parent refusal %s lost: %v", name, kept, r.Spec.Skills.Deny)
					}
				}
			}
		})
		t.Run(layer+": a layer that does not write skills.deny keeps the parent's", func(t *testing.T) {
			r, diags := resolve(layer, "description: another\n")
			if len(diags) != 0 || !reflect.DeepEqual(r.Spec.Skills.Deny, []string{"a/one", "a/two"}) {
				t.Fatalf("diags = %v, deny = %v", diagList(diags), r.Spec.Skills.Deny)
			}
		})
	}

	t.Run("hub layer: replaces, as before", func(t *testing.T) {
		r, diags := resolve("hub", "skills: { deny: [b/three] }\n")
		if len(diags) != 0 || !reflect.DeepEqual(r.Spec.Skills.Deny, []string{"b/three"}) {
			t.Fatalf("diags = %v, deny = %v", diagList(diags), r.Spec.Skills.Deny)
		}
	})
	t.Run("parent without refusals: any list is accepted", func(t *testing.T) {
		r, diags := layeredDiags(t, "skills: { deny: [b/three, b/three] }\n")
		if len(diags) != 0 || !reflect.DeepEqual(r.Spec.Skills.Deny, []string{"b/three"}) {
			t.Fatalf("diags = %v, deny = %v", diagList(diags), r.Spec.Skills.Deny)
		}
	})
}
