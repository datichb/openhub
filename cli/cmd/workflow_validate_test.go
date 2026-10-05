package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/workflow"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// testWorkflowHub builds a hub directory with two agents and one workflow.
func testWorkflowHub(t *testing.T) string {
	t.Helper()
	hub := t.TempDir()
	writeTestFile(t, filepath.Join(hub, "agents", "core", "lead.md"),
		"---\nid: lead\nmode: primary\npermission:\n  edit: deny\n  bash: deny\n  task:\n    \"*\": deny\n    helper: allow\n---\nLead\n")
	writeTestFile(t, filepath.Join(hub, "agents", "core", "helper.md"),
		"---\nid: helper\nmode: subagent\npermission:\n  edit: allow\n  bash: deny\n---\nHelper\n")
	writeTestFile(t, filepath.Join(hub, "workflows", "pair.yaml"),
		"apiVersion: oh/v1\nkind: Workflow\nid: pair\nrisk: write\nentry: { agent: lead }\nagents:\n  lead: { role: workflow }\n  helper: { role: workflow }\n")
	return hub
}

func TestWorkflowValidate_HubID(t *testing.T) {
	r, err := runWorkflowValidate(testWorkflowHub(t), "pair", workflow.LayerHub, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Diagnostics) != 0 || len(r.Workflows) != 1 || !r.Workflows[0].Valid || r.Workflows[0].Ref != "hub:pair" {
		t.Fatalf("report = %+v", r)
	}
	var out bytes.Buffer
	if err := printWorkflowValidate(&out, r, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "hub:pair") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestWorkflowValidate_FileWithErrors(t *testing.T) {
	hub := testWorkflowHub(t)
	file := filepath.Join(t.TempDir(), "pair.yaml")
	writeTestFile(t, file, "apiVersion: oh/v1\nkind: Workflow\nid: pair\nextends: hub:pair\nrisk: publish\nagents:\n  ghost: { role: independent }\n")
	r, err := runWorkflowValidate(hub, file, workflow.LayerTeam, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Diagnostics.Codes(); strings.Join(got, ",") != "loosening,agent_unknown" {
		t.Fatalf("codes = %v", got)
	}
	if r.Workflows[0].Valid || r.Diagnostics[0].Source != file || r.Diagnostics[0].Pos.Line != 5 {
		t.Fatalf("report = %+v", r)
	}

	var out bytes.Buffer
	if err := printWorkflowValidate(&out, r, true); err == nil {
		t.Fatal("errors must make the command fail")
	}
	var decoded workflowValidateReport
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil || len(decoded.Diagnostics) != 2 || decoded.Diagnostics[1].Path != "agents.ghost" {
		t.Fatalf("json = %s (%v)", out.String(), err)
	}
}

func TestWorkflowValidate_SyntaxErrorAndAll(t *testing.T) {
	hub := testWorkflowHub(t)
	writeTestFile(t, filepath.Join(hub, "workflows", "broken.yaml"), "apiVersion: oh/v1\nkind: Workflow\nid: broken\nrisk: write\nfoo: 1\n")

	r, err := runWorkflowValidate(hub, "broken", workflow.LayerHub, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Diagnostics.Codes(); strings.Join(got, ",") != "unknown_workflow,unknown_field" {
		t.Fatalf("codes = %v", got)
	}

	r, err = runWorkflowValidate(hub, "", workflow.LayerHub, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Workflows) != 2 || r.Workflows[0].Ref != "hub:broken" || r.Workflows[0].Valid || !r.Workflows[1].Valid ||
		len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unknown_field" {
		t.Fatalf("report = %+v", r)
	}

	if _, err := runWorkflowValidate(hub, "pair", workflow.LayerSession, false); err == nil {
		t.Fatal("session layer accepted")
	}
}
