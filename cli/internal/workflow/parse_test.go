package workflow

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func parseString(t *testing.T, doc string) (*Document, Diagnostics) {
	t.Helper()
	return Parse([]byte(doc), Source{Layer: LayerHub, Path: "wf.yaml"})
}

const header = "apiVersion: oh/v1\nkind: Workflow\nid: x\n"

func TestParse_FullExampleIsClean(t *testing.T) {
	data, err := os.ReadFile("testdata/spec_v1_full.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, diags := Parse(data, Source{Layer: LayerHub, Path: "spec_v1_full.yaml"})
	if len(diags) != 0 {
		t.Fatalf("diagnostics: %v", diags)
	}
	if doc.Spec.ID != "ticket" || doc.Ref() != (Ref{LayerHub, "ticket"}) {
		t.Fatalf("ref = %v", doc.Ref())
	}
	if got := doc.Pos("agents.developer.after"); got != (Pos{29, 32}) {
		t.Fatalf("pos agents.developer.after = %v", got)
	}
	if got := doc.Pos("outputs[1].type"); got != (Pos{56, 15}) {
		t.Fatalf("pos outputs[1].type = %v", got)
	}
	if !doc.Has("code_mode") || doc.Has("inputs.branch.required") {
		t.Fatal("presence index wrong")
	}
	// Unknown path falls back to the closest written parent.
	if got := doc.Pos("agents.developer.calls"); got != (Pos{29, 3}) {
		t.Fatalf("fallback pos = %v", got)
	}
}

func TestParse_ErrorsCarryLineAndColumn(t *testing.T) {
	cases := []struct {
		name, doc, code, path string
		pos                   Pos
	}{
		{"unknown top field", header + "foo: 1\n", "unknown_field", "foo", Pos{4, 1}},
		{"unknown nested field", header + "inputs:\n  a: { type: string, colour: red }\n", "unknown_field", "inputs.a.colour", Pos{5, 22}},
		{"unknown agent field", header + "agents:\n  dev:\n    role: workflow\n    rol: x\n", "unknown_field", "agents.dev.rol", Pos{7, 5}},
		{"wrong type", header + "version: three\n", "invalid_type", "version", Pos{4, 10}},
		{"wrong type in list", header + "mcp: [a, {b: 1}]\n", "invalid_type", "mcp[1]", Pos{4, 10}},
		{"duplicate key", header + "checkpoints:\n  cp-1: {}\n  cp-1: {}\n", "duplicate_key", "checkpoints.cp-1", Pos{6, 3}},
		{"text as list", header + "description: [a]\n", "expected_text", "description", Pos{4, 14}},
		{"plugin as list", header + "plugins:\n  - [a]\n", "expected_plugin", "plugins[0]", Pos{5, 5}},
		{"map as scalar", header + "agents: nope\n", "expected_mapping", "agents", Pos{4, 9}},
		{"syntax", header + "inputs: {a: [\n", "syntax", "", Pos{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, diags := parseString(t, c.doc)
			var hit *Diagnostic
			for i := range diags {
				if diags[i].Code == c.code {
					hit = &diags[i]
				}
			}
			if hit == nil {
				t.Fatalf("no %s in %v", c.code, diags)
			}
			if hit.Severity != SeverityError || hit.Source != "wf.yaml" {
				t.Fatalf("diag = %+v", hit)
			}
			if c.path != "" && hit.Path != c.path {
				t.Fatalf("path = %q, want %q", hit.Path, c.path)
			}
			if c.pos != (Pos{}) && hit.Pos != c.pos {
				t.Fatalf("pos = %v, want %v", hit.Pos, c.pos)
			}
			if c.code == "syntax" && hit.Pos.Line == 0 {
				t.Fatalf("syntax error without line: %+v", hit)
			}
		})
	}
}

func TestParse_CollectsEveryError(t *testing.T) {
	_, diags := parseString(t, header+"foo: 1\nagents:\n  dev: { role: workflow, bar: 2 }\ndescription: [x]\n")
	got := diags.Codes()
	want := []string{"unknown_field", "unknown_field", "expected_text"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("codes = %v, want %v", got, want)
	}
}

func TestParse_Header(t *testing.T) {
	_, diags := parseString(t, "apiVersion: oh/v2\nkind: Flow\n")
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"field_required", "api_version", "kind"}) {
		t.Fatalf("codes = %v", got)
	}
	if diags[1].Pos != (Pos{1, 1}) || diags[0].Path != "id" {
		t.Fatalf("diags = %+v", diags)
	}
}

func TestParse_DocumentShape(t *testing.T) {
	cases := map[string]string{
		"":                        "empty_document",
		"- a\n":                   "expected_mapping",
		header + "---\n" + header: "multiple_documents",
	}
	for doc, code := range cases {
		_, diags := parseString(t, doc)
		if len(diags) == 0 || diags[0].Code != code {
			t.Errorf("%q: diags = %v, want %s", doc, diags, code)
		}
	}
}

func TestParse_MessagesAreLocalized(t *testing.T) {
	_, diags := parseString(t, header+"foo: 1\n")
	if len(diags) != 1 || strings.HasPrefix(diags[0].Message, "workflow.diag.") || !strings.Contains(diags[0].Message, "foo") {
		t.Fatalf("message not localized: %+v", diags)
	}
}

func TestParseRef(t *testing.T) {
	r, err := ParseRef("team:ticket-hotfix")
	if err != nil || r != (Ref{LayerTeam, "ticket-hotfix"}) || r.String() != "team:ticket-hotfix" {
		t.Fatalf("ref = %v, %v", r, err)
	}
	for _, bad := range []string{"ticket", "session:x", "hub:", "draft:x"} {
		if _, err := ParseRef(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
