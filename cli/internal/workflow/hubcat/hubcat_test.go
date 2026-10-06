package hubcat

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// repoHub is the repository root, laid out like an extracted hub.
const repoHub = "../../../.."

func TestAgentInfo_FromRepositoryAgents(t *testing.T) {
	c, err := New(repoHub)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id           string
		mode         workflow.AgentMode
		edits, shell bool
		task         string
	}{
		{"developer", workflow.ModeSubagent, true, false, "documentarian"},
		{"orchestrator-dev", workflow.ModePrimary, false, false, "developer"},
		{"auditor-subagent", workflow.ModeSubagent, false, false, ""},
		{"reviewer", workflow.ModePrimary, false, false, "documentarian"},
		{"debugger", workflow.ModePrimary, false, true, "documentarian"},
	}
	for _, tc := range cases {
		a, ok := c.Agent(tc.id)
		if !ok {
			t.Fatalf("%s not found", tc.id)
		}
		if a.Mode != tc.mode || a.Edits != tc.edits || a.Shell != tc.shell {
			t.Errorf("%s = %+v", tc.id, a)
		}
		if tc.task != "" && !contains(a.Tasks, tc.task) {
			t.Errorf("%s tasks = %v, want %s", tc.id, a.Tasks, tc.task)
		}
		if len(a.Skills) == 0 {
			t.Errorf("%s has no skills", tc.id)
		}
	}
	if _, ok := c.Agent("ghost"); ok {
		t.Fatal("unknown agent found")
	}
	if !c.HasSkill("shared/universal-guardrails") || c.HasSkill("shared/nope") {
		t.Fatal("skill lookup")
	}
}

func TestAgentInfoFrom_Permissions(t *testing.T) {
	fm := &deploy.AgentFrontmatter{Mode: "primary"}
	cases := []struct {
		name         string
		perms        map[string]any
		edits, shell bool
		tasks        []string
	}{
		{"nothing written: tool defaults", map[string]any{}, true, true, nil},
		{"denied", map[string]any{"edit": "deny", "write": false, "bash": "deny", "task": "deny"}, false, false, nil},
		{"one edit key enough", map[string]any{"edit": "deny", "write": "allow"}, true, true, nil},
		{"restricted shell", map[string]any{"bash": map[string]any{"*": "deny", "git diff*": "allow"}}, true, false, nil},
		{"shell without wildcard", map[string]any{"bash": map[string]any{"git diff*": "allow"}}, true, true, nil},
		{"task map", map[string]any{"task": map[string]any{"*": "deny", "b": "allow", "a": "ask", "self": "allow"}}, true, true, []string{"a", "b"}},
		{"task allow", map[string]any{"task": "allow"}, true, true, []string{"*"}},
	}
	for _, tc := range cases {
		a := AgentInfoFrom("self", fm, tc.perms)
		if a.Edits != tc.edits || a.Shell != tc.shell || !reflect.DeepEqual(a.Tasks, tc.tasks) {
			t.Errorf("%s: %+v", tc.name, a)
		}
	}
}

func TestCheck_TicketLikeWorkflowAgainstRepositoryHub(t *testing.T) {
	hub := t.TempDir()
	for _, d := range []string{"agents", "skills", "permissions"} {
		if err := os.CopyFS(filepath.Join(hub, d), os.DirFS(filepath.Join(repoHub, d))); err != nil {
			t.Fatal(err)
		}
	}
	wfDir := filepath.Join(hub, WorkflowsDir)
	write(t, filepath.Join(wfDir, "ticket.yaml"), `apiVersion: oh/v1
kind: Workflow
id: ticket
risk: write
entry: { agent: orchestrator-dev }
inputs:
  ticket: { type: beads-id, required: true }
prompt: { template: prompts/ticket.md.tmpl }
agents:
  orchestrator-dev: { role: workflow }
  developer: { role: workflow, after: cp-1 }
  reviewer: { role: workflow, after: developer }
  documentarian: { role: independent }
checkpoints:
  cp-1: { mode: { manuel: pause, semi-auto: auto, auto: auto } }
`)
	write(t, filepath.Join(wfDir, "prompts", "ticket.md.tmpl"), "Implement {{ .ticket }} in {{ .oh.project }}.\n")
	write(t, filepath.Join(wfDir, "audit.yaml"), `apiVersion: oh/v1
kind: Workflow
id: audit
risk: read
entry: { agent: auditor }
beads: { allow: [show, list] }
agents:
  auditor: { role: workflow }
  auditor-subagent: { role: workflow }
  developer: { role: independent }
`)

	cat, diags := LoadWorkflows(hub)
	if len(diags) != 0 {
		t.Fatal(diags)
	}
	c, err := New(hub)
	if err != nil {
		t.Fatal(err)
	}
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "ticket"}, nil, c.Env())
	if r == nil || len(diags) != 0 {
		t.Fatalf("ticket: %v", diags)
	}
	_, diags = workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "audit"}, nil, c.Env())
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"read_agent_writes"}) {
		t.Fatalf("audit codes = %v", got)
	}
	if diags[0].Path != "agents.developer" || diags[0].Source != filepath.Join(wfDir, "audit.yaml") || diags[0].Pos.Line != 10 {
		t.Fatalf("audit diag = %+v", diags[0])
	}
}

func TestLoadWorkflows_MissingDirAndNameMismatch(t *testing.T) {
	cat, diags := LoadWorkflows(t.TempDir())
	if len(diags) != 0 || len(cat.Refs()) != 0 {
		t.Fatal("missing dir must give an empty catalog")
	}
	hub := t.TempDir()
	write(t, filepath.Join(hub, WorkflowsDir, "a.yaml"), "apiVersion: oh/v1\nkind: Workflow\nid: b\nrisk: read\n")
	_, diags = LoadWorkflows(hub)
	if len(diags) != 1 || diags[0].Code != "id_filename_mismatch" {
		t.Fatalf("diags = %v", diags)
	}
}

func TestReadPrompt_RefusesEscapes(t *testing.T) {
	c := &Catalog{hubDir: t.TempDir()}
	if _, err := c.ReadPrompt(workflow.Origin{Layer: workflow.LayerHub}, "../../etc/passwd"); err == nil {
		t.Fatal("escape accepted")
	}
	dir := t.TempDir()
	write(t, filepath.Join(dir, "prompts", "x.tmpl"), "next to the file")
	data, err := c.ReadPrompt(workflow.Origin{Layer: workflow.LayerTeam, Source: filepath.Join(dir, "x.yaml")}, "prompts/x.tmpl")
	if err != nil || string(data) != "next to the file" {
		t.Fatalf("relative to the document: %q, %v", data, err)
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConductorIsAReadOnlyCoordinator(t *testing.T) {
	c, err := New(repoHub)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := c.Agent(workflow.DefaultEntryAgent)
	if !ok {
		t.Fatal("conductor agent missing from the hub")
	}
	if a.Mode != workflow.ModePrimary || a.Edits || a.Shell || !reflect.DeepEqual(a.Tasks, []string{"*"}) {
		t.Fatalf("conductor = %+v", a)
	}
	if !contains(a.Skills, "workflow/workflow-map") || !c.HasSkill("workflow/workflow-map") {
		t.Fatal("conductor must load the workflow map")
	}
}

// The enchaînement comes from the workflows (P1-T13): the orchestrator keeps
// only its posture and contracts.
func TestOrchestratorHasNoHardCodedModes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoHub, "agents", "planning", "orchestrator.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, banned := range []string{"Mode A", "Mode B", "Mode C", "Mode D", "Mode E", "CP-0", "CP-onboard", "CP-spec", "CP-audit", "CP-feature", "orchestrator-modes", "orchestrator-ticket-routing"} {
		if strings.Contains(body, banned) {
			t.Errorf("orchestrator.md still contains %q", banned)
		}
	}
}

// The former entry modes A–E are workflows now (P1-T14): no agent or skill
// refers to them, except the legacy static skills only used by `oh deploy`.
func TestNoLegacyEntryModes(t *testing.T) {
	legacy := regexp.MustCompile(`\b[Mm]odes? [A-E]\b`)
	allowed := map[string]bool{
		filepath.Join("skills", "orchestrator", "orchestrator-modes.md"):          true,
		filepath.Join("skills", "orchestrator", "orchestrator-ticket-routing.md"): true,
	}
	for _, dir := range []string{"agents", "skills"} {
		err := filepath.WalkDir(filepath.Join(repoHub, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return err
			}
			rel, _ := filepath.Rel(repoHub, path)
			if allowed[rel] {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if m := legacy.FindString(string(data)); m != "" {
				t.Errorf("%s still refers to %q", rel, m)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
