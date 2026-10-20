package workflow

import (
	"reflect"
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

// S-20: a team or project layer that widens a workflow, by adding an agent or
// skills to `skills.extra`, gets a warning: refusing would break the agents of
// the team catalogue (a documented use), so the addition is accepted and
// reported. Removing skills stays allowed and silent; the hub layer adds
// freely.
func TestPatch_LayerWideningIsWarned(t *testing.T) {
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
	t.Run("agent added by a project layer", func(t *testing.T) {
		_, diags := resolve("project", "agents:\n  security-bot: { role: workflow }\n")
		if w := warnings(diags, "patch_agent_added"); len(w) != 1 || diags.HasErrors() {
			t.Fatalf("diags = %v", diagList(diags))
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

// The parent keeps every MCP server of the project when it does not write
// `mcp:`: any list narrows it.
func TestPatch_MCPWithoutParentList(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:quick":  "apiVersion: oh/v1\nkind: Workflow\nid: quick\nrisk: write\n",
		"team:quick": "apiVersion: oh/v1\nkind: Workflow\nid: quick\nextends: hub:quick\nmcp: [gitlab]\n",
	})
	r, diags := ResolveSpec(cat, Ref{LayerTeam, "quick"}, nil)
	if len(diags) != 0 || !reflect.DeepEqual(r.Spec.MCP, []string{"gitlab"}) {
		t.Fatalf("diags = %v, mcp = %v", diagList(diags), r.Spec.MCP)
	}
}

// The hub layer (oh's own content) may add plugins and MCP servers.
func TestPatch_HubLayerMayAddPlugins(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:base":  strings.Replace(hubLayered, "id: layered", "id: base", 1),
		"hub:child": "apiVersion: oh/v1\nkind: Workflow\nid: child\nextends: hub:base\nplugins: [rtk, context-mode]\nmcp: [gitlab, jira]\n",
	})
	if _, diags := ResolveSpec(cat, Ref{LayerHub, "child"}, nil); len(diags) != 0 {
		t.Fatalf("diags = %v", diagList(diags))
	}
}

func TestValidate_UnlockSkipWarned(t *testing.T) {
	body := "risk: write\ncheckpoints:\n  cp-1: { unlocks: [commit], mode: { manuel: pause, semi-auto: pause, auto: skip } }\n"
	assertDiag(t, checkYAML(t, vHeader+body, testEnv()), "unlock_skip", "checkpoints.cp-1.mode.auto", SeverityWarning)
}

// S-19: a read-only workflow may not load a plugin whose tools run commands.
func TestValidate_ReadWorkflowRefusesExecPlugin(t *testing.T) {
	body := "risk: read\nbeads: { allow: [show] }\nplugins: [context-mode@latest]\n"
	assertDiag(t, checkYAML(t, vHeader+body, testEnv()), "read_plugin_exec", "plugins[0]", SeverityError)
	if !strings.Contains(strings.Join(ExecPlugins, ","), "context-mode") {
		t.Fatal("context-mode must be listed")
	}
}

// A disabled agent leaves the explicit delegations of the others (the
// feature orchestrator lists documentarian; a team may disable it).
func TestPatch_DisabledAgentLeavesCalls(t *testing.T) {
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
