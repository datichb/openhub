package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/teamstate"
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
	r, err := runWorkflowValidate(testWorkflowHub(t), "pair", workflow.LayerHub, false, nil)
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
	r, err := runWorkflowValidate(hub, file, workflow.LayerTeam, false, nil)
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

	r, err := runWorkflowValidate(hub, "broken", workflow.LayerHub, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Diagnostics.Codes(); strings.Join(got, ",") != "unknown_workflow,unknown_field" {
		t.Fatalf("codes = %v", got)
	}

	r, err = runWorkflowValidate(hub, "", workflow.LayerHub, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Workflows) != 2 || r.Workflows[0].Ref != "hub:broken" || r.Workflows[0].Valid || !r.Workflows[1].Valid ||
		len(r.Diagnostics) != 1 || r.Diagnostics[0].Code != "unknown_field" {
		t.Fatalf("report = %+v", r)
	}

	if _, err := runWorkflowValidate(hub, "pair", workflow.LayerSession, false, nil); err == nil {
		t.Fatal("session layer accepted")
	}
}

// testTeamLayers builds a team-state with a published team workflow
// extending hub:pair, a project workflow and a hand-edited file.
func testTeamLayers(t *testing.T) *workflowTeamLayers {
	t.Helper()
	repo := teamstate.NewRepo("", t.TempDir())
	publish := func(scope teamstate.WorkflowScope, id, yaml string) {
		rel, err := teamstate.PublishedRel(scope, id)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(repo.Path(), rel), yaml)
		if _, _, err := repo.SealWorkflowLocal(scope, id, teamstate.LockMeta{Version: 1, PublishedBy: "alice", PublishedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(repo.Path(), "workflows", "prompts", "pair.md.tmpl"), "Pair {{ .oh.project }}")
	publish(teamstate.TeamScope(), "pair", "apiVersion: oh/v1\nkind: Workflow\nid: pair\nextends: hub:pair\nenforce: [agents]\ndescription: Team pair\nprompt: { template: prompts/pair.md.tmpl }\n")
	publish(teamstate.ProjectScope("web"), "pair", "apiVersion: oh/v1\nkind: Workflow\nid: pair\nextends: team:pair\nagents:\n  helper: { role: independent }\n")
	publish(teamstate.TeamScope(), "solo", "apiVersion: oh/v1\nkind: Workflow\nid: solo\nrisk: read\nentry: { agent: lead }\n")
	writeTestFile(t, filepath.Join(repo.Path(), "workflows", "published", "solo.yaml"), "apiVersion: oh/v1\nkind: Workflow\nid: solo\nrisk: write\nentry: { agent: lead }\n")
	return &workflowTeamLayers{Repo: repo, Project: "web"}
}

func TestWorkflowValidate_TeamLayers(t *testing.T) {
	hub := testWorkflowHub(t)
	layers := testTeamLayers(t)

	r, err := runWorkflowValidate(hub, "team:pair", workflow.LayerHub, false, layers)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Diagnostics) != 0 || !r.Workflows[0].Valid {
		t.Fatalf("team:pair report = %+v", r)
	}

	// The project patch writes a field locked by the team.
	r, err = runWorkflowValidate(hub, "project:pair", workflow.LayerHub, false, layers)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Diagnostics.Codes(); strings.Join(got, ",") != "enforced_field" || r.Workflows[0].Valid {
		t.Fatalf("project:pair report = %+v", r)
	}

	// The hand-edited team file is skipped with a warning.
	r, err = runWorkflowValidate(hub, "", workflow.LayerHub, true, layers)
	if err != nil {
		t.Fatal(err)
	}
	refs := []string{}
	for _, w := range r.Workflows {
		refs = append(refs, w.Ref)
	}
	if strings.Join(refs, ",") != "hub:pair,team:pair,project:pair" {
		t.Fatalf("refs = %v", refs)
	}
	var mismatch bool
	for _, d := range r.Diagnostics {
		if d.Code == teamstate.DiagWorkflowHashMismatch && d.Severity == workflow.SeverityWarning {
			mismatch = true
		}
	}
	if !mismatch {
		t.Fatalf("hash mismatch not reported: %v", r.Diagnostics)
	}
}

func TestWorkflowIntegrityCheck(t *testing.T) {
	layers := testTeamLayers(t)
	c := workflowIntegrityCheck("acme", layers.Repo)
	if c.OK || !strings.Contains(c.Detail, "solo.yaml") || !strings.Contains(c.Name, "acme") {
		t.Fatalf("check = %+v", c)
	}
	if err := os.Remove(filepath.Join(layers.Repo.Path(), "workflows", "published", "solo.yaml")); err != nil {
		t.Fatal(err)
	}
	l, _ := layers.Repo.ReadWorkflowLock()
	l.Delete(teamstate.TeamScope(), "solo")
	if err := layers.Repo.WriteWorkflowLockLocal(l); err != nil {
		t.Fatal(err)
	}
	if c := workflowIntegrityCheck("acme", layers.Repo); !c.OK {
		t.Fatalf("check = %+v", c)
	}
}
