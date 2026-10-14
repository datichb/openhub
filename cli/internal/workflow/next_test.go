package workflow

import (
	"reflect"
	"strings"
	"testing"
)

// R5 (schema_next.go): next:, aliases:, outputs[].record and the session
// sources of from:.

func TestParseSessionFrom(t *testing.T) {
	for from, want := range map[string]*SessionFrom{
		"session.output(spec.tickets)": {Source: SourceSessionOutput, Workflow: "spec", Output: "tickets"},
		"session.output(tickets)":      {Source: SourceSessionOutput, Output: "tickets"},
		"session.output( tickets )":    {Source: SourceSessionOutput, Output: "tickets"},
		"session.trace()":              {Source: SourceSessionTrace},
		"session.trace(ticket)":        {Source: SourceSessionTrace, Workflow: "ticket"},
		"session.trace(ticket.x)":      nil,
		"session.output()":             nil,
		"session.output(Spec.tickets)": nil,
		"session.input(method)":        nil,
		"mr.discussions(mr)":           nil,
	} {
		got, ok := ParseSessionFrom(from)
		if want == nil {
			if ok {
				t.Errorf("ParseSessionFrom(%q) = %+v, want invalid", from, got)
			}
			continue
		}
		if !ok || got != *want {
			t.Errorf("ParseSessionFrom(%q) = %+v %v, want %+v", from, got, ok, *want)
		}
	}
	if !IsSessionSource("session.input(x)") || IsSessionSource("mr.discussions(mr)") {
		t.Fatal("IsSessionSource")
	}
}

func TestOutputFits(t *testing.T) {
	for _, c := range []struct {
		out OutputType
		in  InputType
		ok  bool
	}{
		{OutputBeadsIDs, InputBeadsIDs, true}, {OutputBeadsIDs, InputBeadsID, true}, {OutputBeadsIDs, InputText, false},
		{OutputPath, InputPath, true}, {OutputPath, InputText, true}, {OutputPath, InputBeadsIDs, false},
		{OutputBranch, InputBranch, true}, {OutputReport, InputString, true}, {OutputReport, InputPath, false},
	} {
		if OutputFits(c.out, c.in) != c.ok {
			t.Errorf("OutputFits(%s, %s) != %v", c.out, c.in, c.ok)
		}
	}
}

const composeFrom = `apiVersion: oh/v1
kind: Workflow
id: from
risk: plan
beads: { allow: [show, create] }
outputs:
  - { id: tickets, type: beads-ids, record: created }
  - { id: notes, type: path }
  - { id: report, type: report, record: report }
`

const composeTo = `apiVersion: oh/v1
kind: Workflow
id: to
risk: write
inputs:
  tickets: { type: beads-ids, from: session.output(from.tickets) }
  notes: { type: text }
  count: { type: int }
  findings: { type: text, from: session.output(from.report) }
`

// composeCheck validates hub:<id> of a catalog of the documents.
func composeCheck(t *testing.T, id string, docs map[string]string) Diagnostics {
	t.Helper()
	_, diags := Check(catalogOf(t, docs), Ref{LayerHub, id}, nil, Env{})
	return diags
}

func hasDiag(ds Diagnostics, code, path string) bool {
	for _, d := range ds {
		if d.Code == code && (path == "" || d.Path == path) {
			return true
		}
	}
	return false
}

func TestValidate_Next(t *testing.T) {
	cases := []struct {
		name, next, code, path string
	}{
		{"valid", "next:\n  - { workflow: to, when: { outputs: [tickets] }, inputs: { tickets: session.output(tickets), notes: session.output(notes) } }\n", "", ""},
		{"unknown workflow", "next:\n  - { workflow: nope }\n", "next_unknown_workflow", "next[0].workflow"},
		{"self", "next:\n  - { workflow: from }\n", "next_self", "next[0].workflow"},
		{"twice", "next:\n  - { workflow: to }\n  - { workflow: to }\n", "duplicate_entry", "next[1].workflow"},
		{"unknown when output", "next:\n  - { workflow: to, when: { outputs: [branch] } }\n", "next_unknown_output", "next[0].when.outputs[0]"},
		{"not a session output", "next:\n  - { workflow: to, inputs: { notes: \"mr.discussions(mr)\" } }\n", "next_input_invalid", "next[0].inputs.notes"},
		{"qualified", "next:\n  - { workflow: to, inputs: { tickets: session.output(from.tickets) } }\n", "next_input_invalid", "next[0].inputs.tickets"},
		{"unknown output", "next:\n  - { workflow: to, inputs: { notes: session.output(branch) } }\n", "next_unknown_output", "next[0].inputs.notes"},
		{"unknown input", "next:\n  - { workflow: to, inputs: { ticket: session.output(tickets) } }\n", "next_unknown_input", "next[0].inputs.ticket"},
		{"type", "next:\n  - { workflow: to, inputs: { count: session.output(tickets) } }\n", "next_input_type", "next[0].inputs.count"},
		{"equals", "next:\n  - { workflow: to, when: { equals: { notes: x } } }\n", "", ""},
		{"unknown equals output", "next:\n  - { workflow: to, when: { equals: { verdict: go } } }\n", "next_unknown_output", "next[0].when.equals.verdict"},
		{"report from", "next:\n  - { workflow: to, inputs: { findings: session.output(report) } }\n", "", ""},
		{"report by id", "next:\n  - { workflow: to, inputs: { notes: session.output(report) } }\n", "next_input_report_from", "next[0].inputs.notes"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := composeCheck(t, "from", map[string]string{"hub:from": composeFrom + c.next, "hub:to": composeTo})
			if c.code == "" {
				if diags.HasErrors() {
					t.Fatalf("diags = %v", diagList(diags))
				}
				return
			}
			if !hasDiag(diags, c.code, c.path) {
				t.Fatalf("diags = %v, want %s at %s", diagList(diags), c.code, c.path)
			}
		})
	}
}

func TestValidate_SessionFrom(t *testing.T) {
	cases := []struct {
		name, input, code string
	}{
		{"valid", "  tickets: { type: beads-ids, from: session.output(from.tickets) }\n", ""},
		{"trace", "  digest: { type: text, from: session.trace() }\n", ""},
		{"trace of a workflow", "  digest: { type: text, from: session.trace(from) }\n", ""},
		{"trace not text", "  digest: { type: string, from: session.trace() }\n", "input_from_type"},
		{"no workflow", "  tickets: { type: beads-ids, from: session.output(tickets) }\n", "input_from_session_workflow"},
		{"unknown workflow", "  tickets: { type: beads-ids, from: session.output(nope.tickets) }\n", "input_from_unknown_workflow"},
		{"unknown output", "  tickets: { type: beads-ids, from: session.output(from.branch) }\n", "input_from_unknown_output"},
		{"type", "  tickets: { type: int, from: session.output(from.tickets) }\n", "input_from_type"},
		{"invalid", "  tickets: { type: beads-ids, from: session.input(tickets) }\n", "input_from_invalid"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := "apiVersion: oh/v1\nkind: Workflow\nid: to\nrisk: write\ninputs:\n" + c.input
			diags := composeCheck(t, "to", map[string]string{"hub:from": composeFrom, "hub:to": doc})
			if c.code == "" {
				if diags.HasErrors() {
					t.Fatalf("diags = %v", diagList(diags))
				}
				return
			}
			if !hasDiag(diags, c.code, "") {
				t.Fatalf("diags = %v, want %s", diagList(diags), c.code)
			}
		})
	}
}

func TestValidate_OutputRecord(t *testing.T) {
	cases := []struct {
		name, outputs, code string
	}{
		{"created", "  - { id: tickets, type: beads-ids, record: created }\n", ""},
		{"report", "  - { id: report, type: report, record: report }\n", ""},
		{"unknown kind", "  - { id: tickets, type: beads-ids, record: seen }\n", "enum_invalid"},
		{"type", "  - { id: tickets, type: path, record: created }\n", "output_record_type"},
		{"spec without check", "  - { id: spec, type: path, record: spec }\n", "output_record_spec_unchecked"},
		{"input", "  - { id: method, type: text, record: input }\ninputs:\n  method: { type: enum, values: [standard, tdd] }\n", ""},
		{"input unknown", "  - { id: method, type: text, record: input }\n", "output_record_input_unknown"},
		{"input type", "  - { id: method, type: path, record: input }\ninputs:\n  method: { type: enum, values: [standard, tdd] }\n", "output_record_type"},
		// B1: files written under paths, or a value read in them.
		{"written", "  - { id: decision, type: path, record: written, paths: [\"docs/wiki/decisions/**\"] }\n", ""},
		{"written match", "  - { id: verdict, type: text, record: written, paths: [docs/a.md], match: 'Décision : (go|kill)' }\n", ""},
		{"written type", "  - { id: verdict, type: path, record: written, paths: [docs/a.md], match: '(go)' }\n", "output_record_type"},
		{"written text without match", "  - { id: verdict, type: text, record: written, paths: [docs/a.md] }\n", "output_record_type"},
		{"written without paths", "  - { id: decision, type: path, record: written }\n", "output_written_paths"},
		{"written absolute", "  - { id: decision, type: path, record: written, paths: [/etc/passwd] }\n", "output_written_path"},
		{"written outside", "  - { id: decision, type: path, record: written, paths: [../x.md] }\n", "output_written_path"},
		{"match without group", "  - { id: verdict, type: text, record: written, paths: [docs/a.md], match: 'go' }\n", "output_match_invalid"},
		{"match invalid", "  - { id: verdict, type: text, record: written, paths: [docs/a.md], match: '(go' }\n", "output_match_invalid"},
		{"paths without written", "  - { id: spec, type: path, paths: [docs/a.md] }\n", "output_written_only"},
		{"match without written", "  - { id: tickets, type: beads-ids, record: created, match: '(x)' }\n", "output_written_only"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			diags := checkYAML(t, vHeader+"risk: write\noutputs:\n"+c.outputs, testEnv())
			if c.code == "" {
				if len(diags) != 0 {
					t.Fatalf("diags = %v", diagList(diags))
				}
				return
			}
			if !hasDiag(diags, c.code, "") {
				t.Fatalf("diags = %v, want %s", diagList(diags), c.code)
			}
		})
	}
}

func TestValidate_Aliases(t *testing.T) {
	cases := []struct {
		name, aliases, code string
	}{
		{"valid", "aliases: [old, { id: older, with: tickets }]\n", ""},
		{"self", "aliases: [from]\n", "alias_self"},
		{"id", "aliases: [Old]\n", "id_invalid"},
		{"with", "aliases: [{ id: old, with: nope }]\n", "alias_with_unknown_input"},
		{"shadowed", "aliases: [to]\n", "alias_shadowed"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := strings.Replace(composeFrom, "risk: plan\n", "risk: plan\ninputs:\n  tickets: { type: beads-ids }\n"+c.aliases, 1)
			diags := composeCheck(t, "from", map[string]string{"hub:from": doc, "hub:to": composeTo})
			if c.code == "" {
				if diags.HasErrors() || len(diags) != 0 {
					t.Fatalf("diags = %v", diagList(diags))
				}
				return
			}
			if !hasDiag(diags, c.code, "") {
				t.Fatalf("diags = %v, want %s", diagList(diags), c.code)
			}
		})
	}
}

// An alias launches its default target, or the target whose `with` input
// is given; a document with the alias id wins.
func TestResolveAlias(t *testing.T) {
	plan := "apiVersion: oh/v1\nkind: Workflow\nid: plan\nrisk: write\naliases: [feat, old]\n"
	impl := "apiVersion: oh/v1\nkind: Workflow\nid: impl\nrisk: write\ninputs:\n  tickets: { type: beads-ids }\naliases: [{ id: feat, with: tickets }]\n"
	cat := catalogOf(t, map[string]string{"hub:plan": plan, "hub:impl": impl})
	given := func(in string) bool { return in == "tickets" }
	if ref, ok := cat.ResolveAlias("feat", nil); !ok || ref != (Ref{LayerHub, "plan"}) {
		t.Fatalf("feat = %v %v", ref, ok)
	}
	if ref, ok := cat.ResolveAlias("feat", given); !ok || ref != (Ref{LayerHub, "impl"}) {
		t.Fatalf("feat with tickets = %v %v", ref, ok)
	}
	if ref, ok := cat.ResolveAlias("old", given); !ok || ref.ID != "plan" {
		t.Fatalf("old = %v %v", ref, ok)
	}
	if _, ok := cat.ResolveAlias("plan", nil); ok {
		t.Fatal("a workflow id is not an alias")
	}
	if _, ok := cat.ResolveAlias("nope", nil); ok {
		t.Fatal("unknown alias")
	}
	if sp, ok := cat.WorkflowSpec("old"); !ok || sp.ID != "plan" {
		t.Fatalf("WorkflowSpec(old) = %v", sp)
	}
	if ds := CheckAliases(cat); len(ds) != 0 {
		t.Fatalf("CheckAliases = %v", diagList(ds))
	}

	// A team document named after the alias wins.
	cat = catalogOf(t, map[string]string{"hub:plan": plan, "hub:impl": impl, "team:old": "apiVersion: oh/v1\nkind: Workflow\nid: old\nrisk: read\n"})
	if _, ok := cat.ResolveAlias("old", nil); ok {
		t.Fatal("document shadowed by the alias")
	}

	// Two default targets, or none, cannot be resolved.
	other := "apiVersion: oh/v1\nkind: Workflow\nid: other\nrisk: write\naliases: [feat]\n"
	if ds := CheckAliases(catalogOf(t, map[string]string{"hub:plan": plan, "hub:other": other})); !hasDiag(ds, "alias_ambiguous", "") {
		t.Fatalf("ambiguous = %v", diagList(ds))
	}
	if ds := CheckAliases(catalogOf(t, map[string]string{"hub:impl": impl})); !hasDiag(ds, "alias_without_default", "") {
		t.Fatalf("without default = %v", diagList(ds))
	}
}

// A patch replaces next: and aliases: as a whole; the parent is untouched.
func TestPatch_Next(t *testing.T) {
	hub := composeFrom + "aliases: [old]\nnext:\n  - { workflow: to, inputs: { tickets: session.output(tickets) } }\n"
	cat := catalogOf(t, map[string]string{
		"hub:from":  hub,
		"hub:to":    composeTo,
		"team:from": "apiVersion: oh/v1\nkind: Workflow\nid: from\nextends: hub:from\nnext: []\naliases: [older]\n",
	})
	r, diags := ResolveSpec(cat, Ref{LayerTeam, "from"}, nil)
	if diags.HasErrors() || len(r.Spec.Next) != 0 || !reflect.DeepEqual(r.Spec.Aliases, []Alias{{ID: "older"}}) {
		t.Fatalf("patched = %+v %+v %v", r.Spec.Next, r.Spec.Aliases, diagList(diags))
	}
	parent, _ := ResolveSpec(cat, Ref{LayerHub, "from"}, nil)
	if len(parent.Spec.Next) != 1 || parent.Spec.Next[0].Inputs["tickets"] != "session.output(tickets)" {
		t.Fatalf("parent = %+v", parent.Spec.Next)
	}
	c := parent.Spec.Clone()
	c.Next[0].Inputs["tickets"] = "x"
	if parent.Spec.Next[0].Inputs["tickets"] == "x" {
		t.Fatal("Clone shares next inputs")
	}
}

// feature-migration: `record: input` records the value of an input (given,
// else its default) so that `next:` passes it on (method of spec).
func TestRecordedInputs(t *testing.T) {
	doc, diags := Parse([]byte(vHeader+"risk: write\ninputs:\n  method: { type: enum, values: [standard, tdd], default: standard }\n  request: { type: text }\noutputs:\n  - { id: method, type: text, record: input }\n  - { id: tickets, type: beads-ids, record: created }\n"), Source{Layer: LayerHub})
	if diags.HasErrors() {
		t.Fatal(diagList(diags))
	}
	s := doc.Spec
	if got := s.RecordedInputs(map[string]any{"method": "tdd", "request": "x"}); !reflect.DeepEqual(got, map[string]any{"method": "tdd"}) {
		t.Fatalf("given = %v", got)
	}
	if got := s.RecordedInputs(nil); !reflect.DeepEqual(got, map[string]any{"method": "standard"}) {
		t.Fatalf("default = %v", got)
	}
	if got := (&Spec{}).RecordedInputs(map[string]any{"method": "tdd"}); got != nil {
		t.Fatalf("none = %v", got)
	}
	if !OutputFits(OutputText, InputEnum) || OutputFits(OutputText, InputBeadsIDs) {
		t.Fatal("OutputFits(text)")
	}
}

// B1: a step is proposed when its outputs were recorded, with the values of
// equals (case ignored).
func TestNextWhenHolds(t *testing.T) {
	var none *NextWhen
	if !none.Holds(nil) {
		t.Fatal("no condition: always")
	}
	w := &NextWhen{Outputs: []string{"decision"}, Equals: map[string]string{"verdict": "go"}}
	for _, c := range []struct {
		outputs map[string]any
		want    bool
	}{
		{map[string]any{"decision": "docs/wiki/decisions/a.md", "verdict": "go"}, true},
		{map[string]any{"decision": "docs/wiki/decisions/a.md", "verdict": " Go "}, true},
		{map[string]any{"decision": "docs/wiki/decisions/a.md", "verdict": "kill"}, false},
		{map[string]any{"decision": "docs/wiki/decisions/a.md"}, false},
		{map[string]any{"verdict": "go"}, false},
		{map[string]any{"decision": []any{}, "verdict": "go"}, false},
	} {
		if got := w.Holds(c.outputs); got != c.want {
			t.Errorf("Holds(%v) = %v, want %v", c.outputs, got, c.want)
		}
	}
}
