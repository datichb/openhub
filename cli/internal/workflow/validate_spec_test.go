package workflow

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

type fakeAgents map[string]AgentInfo

func (f fakeAgents) Agent(id string) (AgentInfo, bool) {
	a, ok := f[id]
	a.ID = id
	return a, ok
}

type fakeSkills struct {
	known  map[string]bool
	issues []SkillIssue
}

func (f fakeSkills) HasSkill(ref string) bool { return f.known[ref] }
func (f fakeSkills) Closure(roots []string) ([]string, []SkillIssue) {
	return roots, f.issues
}

type fakePrompts map[string]string

func (f fakePrompts) ReadPrompt(_ Origin, path string) ([]byte, error) {
	if s, ok := f[path]; ok {
		return []byte(s), nil
	}
	return nil, errors.New("not found")
}

var testAgents = fakeAgents{
	"conductor":        {Mode: ModePrimary, Tasks: []string{"*"}},
	"orchestrator-dev": {Mode: ModePrimary, Tasks: []string{"developer", "reviewer", "documentarian"}},
	"developer":        {Mode: ModeSubagent, Edits: true, Tasks: []string{"documentarian"}, Skills: []string{"developer/dev-standards"}},
	"reviewer":         {Mode: ModeSubagent, Tasks: []string{"documentarian"}},
	"documentarian":    {Mode: ModeSubagent, Edits: true},
	"auditor":          {Mode: ModePrimary, Tasks: []string{"auditor-subagent"}},
	"auditor-subagent": {Mode: ModeSubagent},
	"pinger":           {Mode: ModeSubagent, Tasks: []string{"ponger"}},
	"ponger":           {Mode: ModeSubagent, Tasks: []string{"pinger"}},
	"shell-user":       {Mode: ModeSubagent, Shell: true},
}

func testEnv() Env {
	return Env{
		Agents: testAgents,
		Skills: fakeSkills{known: map[string]bool{"shared/beads-dev": true, "developer/dev-standards": true}},
		Prompts: fakePrompts{
			"prompts/ticket.md.tmpl": "Ticket {{ .ticket }} on {{ .branch }} ({{ .oh.project }}, {{ .oh.mode }})\n{{ if .publish }}publish{{ end }} {{ .type }}",
			"prompts/bad.md.tmpl":    "{{ .nope }} {{ .oh.secret }}",
		},
		Isolation: sessionspec.IsolationFull,
	}
}

func checkYAML(t *testing.T, doc string, env Env) Diagnostics {
	t.Helper()
	cat := catalogOf(t, map[string]string{"hub:x": doc})
	r, diags := Check(cat, Ref{LayerHub, "x"}, nil, env)
	if r == nil {
		t.Fatalf("resolution failed: %v", diags)
	}
	return diags
}

const vHeader = "apiVersion: oh/v1\nkind: Workflow\nid: x\n"

func TestValidate_FullExampleIsValid(t *testing.T) {
	data, err := os.ReadFile("testdata/spec_v1_full.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cat := NewMemCatalog()
	doc, _ := Parse(data, Source{Layer: LayerHub, Path: "spec_v1_full.yaml"})
	cat.Add(doc)
	_, diags := Check(cat, Ref{LayerHub, "ticket"}, nil, testEnv())
	if len(diags) != 0 {
		t.Fatalf("diagnostics:\n%v", diags.Err())
	}
}

func TestValidate_Rules(t *testing.T) {
	cases := []struct {
		name, body, code, path string
		sev                    Severity
	}{
		// header
		{"id", "risk: write\n", "id_invalid", "id", SeverityError},
		{"risk required", "", "field_required", "risk", SeverityError},
		{"risk enum", "risk: maybe\n", "enum_invalid", "risk", SeverityError},
		{"category enum", "risk: write\ncategory: misc\n", "enum_invalid", "category", SeverityError},
		{"isolation enum", "risk: write\nisolation: loose\n", "enum_invalid", "isolation", SeverityError},
		{"negative breaker", "risk: write\ncircuit_breaker: { max_consecutive_subagents: -1 }\n", "negative_value", "circuit_breaker.max_consecutive_subagents", SeverityError},
		{"negative budget", "risk: write\nlimits: { budget_usd: -2 }\n", "negative_value", "limits.budget_usd", SeverityError},
		// entry
		{"entry unknown", "risk: write\nentry: { agent: ghost }\n", "agent_unknown", "entry.agent", SeverityError},
		{"entry subagent", "risk: write\nentry: { agent: developer }\n", "entry_not_primary", "entry.agent", SeverityError},
		{"entry mode override", "risk: write\nentry: { agent: auditor }\nagents:\n  auditor: { role: workflow, mode: subagent, calls: [] }\n", "entry_not_primary", "agents.auditor.mode", SeverityError},
		{"entry independent", "risk: write\nentry: { agent: auditor }\nagents:\n  auditor: { role: independent }\n", "entry_role", "agents.auditor.role", SeverityError},
		// inputs
		{"input reserved", "risk: write\ninputs:\n  oh: { type: string }\n", "input_id_reserved", "inputs.oh", SeverityError},
		{"input id", "risk: write\ninputs:\n  my-input: { type: string }\n", "input_id_invalid", "inputs.my-input", SeverityError},
		{"input type required", "risk: write\ninputs:\n  a: { required: true }\n", "field_required", "inputs.a.type", SeverityError},
		{"input type enum", "risk: write\ninputs:\n  a: { type: file }\n", "enum_invalid", "inputs.a.type", SeverityError},
		{"enum values", "risk: write\ninputs:\n  a: { type: enum }\n", "enum_values_missing", "inputs.a.values", SeverityError},
		{"values ignored", "risk: write\ninputs:\n  a: { type: string, values: [x] }\n", "field_ignored", "inputs.a.values", SeverityWarning},
		{"picker ignored", "risk: write\ninputs:\n  a: { type: string, picker: { multi: true } }\n", "field_ignored", "inputs.a.picker", SeverityWarning},
		{"enum default", "risk: write\ninputs:\n  a: { type: enum, values: [x, y], default: z }\n", "input_default_invalid", "inputs.a.default", SeverityError},
		{"bool default", "risk: write\ninputs:\n  a: { type: bool, default: maybe }\n", "input_default_invalid", "inputs.a.default", SeverityError},
		{"default self ref", "risk: write\ninputs:\n  a: { type: string, default: \"{{ .a }}\" }\n", "prompt_unknown_variable", "inputs.a.default", SeverityError},
		{"negative max length", "risk: write\ninputs:\n  a: { type: text, max_length: -1 }\n", "negative_value", "inputs.a.max_length", SeverityError},
		// computed inputs (QB2)
		{"from form", "risk: write\ninputs:\n  a: { type: text, from: mr.discussions }\n", "input_from_invalid", "inputs.a.from", SeverityError},
		{"from source", "risk: write\ninputs:\n  mr: { type: string }\n  a: { type: text, from: \"jira.issue(mr)\" }\n", "input_from_unknown_source", "inputs.a.from", SeverityError},
		{"from input", "risk: write\ninputs:\n  a: { type: text, from: \"mr.discussions(mr)\" }\n", "input_from_unknown_input", "inputs.a.from", SeverityError},
		{"from self", "risk: write\ninputs:\n  a: { type: text, from: \"mr.discussions(a)\" }\n", "input_from_unknown_input", "inputs.a.from", SeverityError},
		// prompt
		{"prompt both", "risk: write\nprompt: { text: a, template: b.tmpl }\n", "prompt_both", "prompt", SeverityError},
		{"prompt empty", "risk: write\nprompt: {}\n", "prompt_empty", "prompt", SeverityError},
		{"prompt var", "risk: write\ninputs:\n  a: { type: string }\nprompt: { text: \"{{ .a }} {{ .b }}\" }\n", "prompt_unknown_variable", "prompt.text", SeverityError},
		{"prompt oh var", "risk: write\nprompt: { text: \"{{ .oh.secret }}\" }\n", "prompt_unknown_variable", "prompt.text", SeverityError},
		{"prompt parse", "risk: write\nprompt: { text: \"{{ if }}\" }\n", "prompt_parse", "prompt.text", SeverityError},
		{"prompt path", "risk: write\nprompt: { template: ../etc/passwd }\n", "prompt_template_path", "prompt.template", SeverityError},
		{"prompt missing", "risk: write\nprompt: { template: prompts/none.md.tmpl }\n", "prompt_template_missing", "prompt.template", SeverityError},
		{"prompt file var", "risk: write\nprompt: { template: prompts/bad.md.tmpl }\n", "prompt_unknown_variable", "prompt.template", SeverityError},
		// agents and graph
		{"agent unknown", "risk: write\nagents:\n  ghost: { role: workflow }\n", "agent_unknown", "agents.ghost", SeverityError},
		{"agent id", "risk: write\nagents:\n  Dev_1: { role: independent }\n", "agent_id_invalid", "agents.Dev_1", SeverityError},
		{"role required", "risk: write\nagents:\n  developer: { after: x }\n", "field_required", "agents.developer.role", SeverityError},
		{"role enum", "risk: write\nagents:\n  developer: { role: boss }\n", "enum_invalid", "agents.developer.role", SeverityError},
		{"mode enum", "risk: write\nagents:\n  developer: { role: workflow, mode: main }\n", "enum_invalid", "agents.developer.mode", SeverityError},
		{"after unknown", "risk: write\nagents:\n  developer: { role: workflow, after: cp-9 }\n", "after_unknown", "agents.developer.after", SeverityError},
		{"after self", "risk: write\nagents:\n  developer: { role: workflow, after: developer }\n", "after_unknown", "agents.developer.after", SeverityError},
		{"after cycle", "risk: write\nagents:\n  developer: { role: workflow, after: reviewer }\n  reviewer: { role: workflow, after: developer }\n", "after_cycle", "agents.developer.after", SeverityError},
		{"calls unknown", "risk: write\nagents:\n  developer: { role: workflow, calls: [ghost] }\n", "calls_unknown", "agents.developer.calls[0]", SeverityError},
		{"graph cycle", "risk: write\nagents:\n  developer: { role: workflow, calls: [reviewer] }\n  reviewer: { role: workflow, calls: [developer] }\n", "graph_cycle", "", SeverityError},
		{"derived cycle", "risk: write\nagents:\n  pinger: { role: workflow }\n  ponger: { role: workflow }\n", "graph_cycle", "", SeverityError},
		{"unreachable", "risk: write\nentry: { agent: orchestrator-dev }\nagents:\n  orchestrator-dev: { role: workflow, calls: [developer] }\n  developer: { role: workflow, calls: [] }\n  reviewer: { role: workflow }\n", "agent_unreachable", "agents.reviewer", SeverityError},
		// checkpoints
		{"checkpoint id", "risk: write\ncheckpoints:\n  CP1: { mode: { manuel: pause, semi-auto: pause, auto: pause } }\n", "checkpoint_id_invalid", "checkpoints.CP1", SeverityError},
		{"checkpoint clash", "risk: write\nagents:\n  developer: { role: workflow }\ncheckpoints:\n  developer: { mode: { manuel: pause, semi-auto: pause, auto: pause } }\n", "checkpoint_agent_clash", "checkpoints.developer", SeverityError},
		{"checkpoint mode unknown", "risk: write\ncheckpoints:\n  cp-1: { mode: { manuel: pause, semi-auto: pause, auto: pause, yolo: auto } }\n", "checkpoint_mode_unknown", "checkpoints.cp-1.mode.yolo", SeverityError},
		{"checkpoint behavior", "risk: write\ncheckpoints:\n  cp-1: { mode: { manuel: wait, semi-auto: pause, auto: pause } }\n", "enum_invalid", "checkpoints.cp-1.mode.manuel", SeverityError},
		{"checkpoint condition", "risk: write\ncheckpoints:\n  cp-1: { mode: { manuel: pause, semi-auto: conditional, auto: pause } }\n", "checkpoint_condition_missing", "checkpoints.cp-1.mode.semi-auto", SeverityError},
		{"checkpoint mode missing", "risk: write\ncheckpoints:\n  cp-1: { mode: { manuel: pause } }\n", "checkpoint_mode_missing", "checkpoints.cp-1.mode", SeverityWarning},
		{"checkpoint remote", "risk: write\ncheckpoints:\n  cp-1: { remote: later, mode: { manuel: pause, semi-auto: pause, auto: pause } }\n", "enum_invalid", "checkpoints.cp-1.remote", SeverityError},
		// modes
		{"mode default", "risk: write\nmodes: { default: auto, allowed: [manuel] }\n", "mode_default_not_allowed", "modes.default", SeverityError},
		{"mode duplicate", "risk: write\nmodes: { allowed: [manuel, manuel] }\n", "duplicate_entry", "modes.allowed[1]", SeverityError},
		// resources
		{"model", "risk: write\nmodels: { default: sonnet }\n", "model_invalid", "models.default", SeverityError},
		{"model agent", "risk: write\nmodels: { agents: { ghost: a/b } }\n", "model_agent_unknown", "models.agents.ghost", SeverityError},
		{"skill unknown", "risk: write\nskills: { extra: [shared/nope] }\n", "skill_unknown", "skills.extra[0]", SeverityError},
		{"skill deny unknown", "risk: write\nskills: { deny: [shared/nope] }\n", "skill_unknown_deny", "skills.deny[0]", SeverityWarning},
		{"plugin id", "risk: write\nplugins: [{ id: \"\" }]\n", "field_required", "plugins[0].id", SeverityError},
		{"plugin duplicate", "risk: write\nplugins: [a, a]\n", "duplicate_entry", "plugins[1].id", SeverityError},
		{"mcp duplicate", "risk: write\nmcp: [gitlab, gitlab]\n", "duplicate_entry", "mcp[1]", SeverityError},
		{"runtime enum", "risk: write\nruntime: { allowed: [local, cloud] }\n", "enum_invalid", "runtime.allowed[1]", SeverityError},
		{"runtime default", "risk: write\nruntime: { default: remote, allowed: [local] }\n", "runtime_default_not_allowed", "runtime.default", SeverityError},
		{"output type", "risk: write\noutputs: [{ id: a, type: file }]\n", "enum_invalid", "outputs[0].type", SeverityError},
		{"output type required", "risk: write\noutputs: [{ id: a }]\n", "field_required", "outputs[0].type", SeverityError},
		{"output duplicate", "risk: write\noutputs: [{ id: a, type: path }, { id: a, type: branch }]\n", "duplicate_entry", "outputs[1].id", SeverityError},
		// security
		{"read edits", "risk: read\nbeads: { allow: [show] }\nentry: { agent: orchestrator-dev }\nagents:\n  orchestrator-dev: { role: workflow, calls: [developer] }\n  developer: { role: workflow }\n", "read_agent_writes", "agents.developer", SeverityError},
		{"read shell", "risk: read\nbeads: { allow: [show] }\nagents:\n  shell-user: { role: independent }\n", "read_agent_writes", "agents.shell-user", SeverityError},
		{"read beads unrestricted", "risk: read\nentry: { agent: auditor }\n", "read_beads_unrestricted", "risk", SeverityError},
		{"read beads write", "risk: read\nentry: { agent: auditor }\nbeads: { allow: [show, create] }\n", "read_beads_write", "beads.allow[1]", SeverityError},
		{"remote forbid mandatory", "risk: write\nruntime: { allowed: [local, remote] }\ncheckpoints:\n  cp-1: { mandatory: true, remote: forbid, mode: { manuel: auto, semi-auto: auto, auto: auto } }\n", "remote_forbidden_checkpoint", "checkpoints.cp-1.remote", SeverityError},
		{"remote forbid pause", "risk: write\nruntime: { allowed: [local, remote] }\ncheckpoints:\n  cp-1: { remote: forbid, mode: { manuel: pause, semi-auto: auto, auto: auto } }\n", "remote_forbidden_checkpoint", "checkpoints.cp-1.remote", SeverityError},
		// risk: plan
		{"plan edits", "risk: plan\nbeads: { allow: [show, create] }\nentry: { agent: orchestrator-dev }\nagents:\n  orchestrator-dev: { role: workflow, calls: [developer] }\n  developer: { role: workflow }\n", "plan_agent_writes", "agents.developer", SeverityError},
		{"plan shell", "risk: plan\nbeads: { allow: [show, create] }\nagents:\n  shell-user: { role: independent }\n", "plan_agent_writes", "agents.shell-user", SeverityError},
		{"plan beads unrestricted", "risk: plan\nentry: { agent: auditor }\n", "plan_beads_unrestricted", "risk", SeverityError},
		{"plan beads delete", "risk: plan\nentry: { agent: auditor }\nbeads: { allow: [show, create, delete] }\n", "plan_beads_delete", "beads.allow[2]", SeverityError},
		{"plan without writes", "risk: plan\nentry: { agent: auditor }\nbeads: { allow: [show, list] }\n", "plan_without_beads_write", "risk", SeverityWarning},
		{"read comments", "risk: read\nentry: { agent: auditor }\nbeads: { allow: [show, comments] }\n", "read_beads_write", "beads.allow[1]", SeverityError},
		// preconditions
		{"precondition id", "risk: write\npreconditions:\n  Wiki: { check: { path_exists: [a] } }\n", "precondition_id_invalid", "preconditions.Wiki", SeverityError},
		{"precondition check", "risk: write\npreconditions:\n  wiki: { check: {} }\n", "precondition_check_invalid", "preconditions.wiki.check", SeverityError},
		{"precondition path", "risk: write\npreconditions:\n  wiki: { check: { path_exists: [ok, ../up] } }\n", "precondition_path_invalid", "preconditions.wiki.check.path_exists[1]", SeverityError},
		{"precondition abs path", "risk: write\npreconditions:\n  wiki: { check: { path_exists: [/etc/passwd] } }\n", "precondition_path_invalid", "preconditions.wiki.check.path_exists[0]", SeverityError},
		{"precondition on_fail", "risk: write\npreconditions:\n  wiki: { check: { path_exists: [a] }, on_fail: warn }\n", "enum_invalid", "preconditions.wiki.on_fail", SeverityError},
		{"precondition suggest required", "risk: write\npreconditions:\n  wiki: { check: { path_exists: [a] }, suggest: { resume: true } }\n", "field_required", "preconditions.wiki.suggest.workflow", SeverityError},
		{"precondition self", "risk: write\npreconditions:\n  wiki: { check: { path_exists: [a] }, suggest: { workflow: x } }\n", "precondition_self", "preconditions.wiki.suggest.workflow", SeverityError},
		{"precondition unknown workflow", "risk: write\npreconditions:\n  wiki: { check: { path_exists: [a] }, suggest: { workflow: nope } }\n", "precondition_unknown_workflow", "preconditions.wiki.suggest.workflow", SeverityError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := vHeader + c.body
			if c.name == "id" {
				doc = strings.Replace(doc, "id: x", "id: X_1", 1)
				cat := catalogOf(t, map[string]string{"hub:X_1": doc})
				_, diags := Check(cat, Ref{LayerHub, "X_1"}, nil, testEnv())
				assertDiag(t, diags, c.code, c.path, c.sev)
				return
			}
			assertDiag(t, checkYAML(t, doc, testEnv()), c.code, c.path, c.sev)
		})
	}
}

func assertDiag(t *testing.T, diags Diagnostics, code, path string, sev Severity) {
	t.Helper()
	for _, d := range diags {
		if d.Code == code && (path == "" || d.Path == path) {
			if d.Severity != sev {
				t.Fatalf("%s at %s: severity %s, want %s", code, path, d.Severity, sev)
			}
			if d.Pos.Line == 0 || d.Source == "" {
				t.Fatalf("%s at %s: no location (%+v)", code, path, d)
			}
			return
		}
	}
	t.Fatalf("no %s at %q in:\n%v", code, path, diagList(diags))
}

func diagList(ds Diagnostics) string {
	var b strings.Builder
	for _, d := range ds {
		b.WriteString(d.Code + " " + d.Path + " — " + d.Message + "\n")
	}
	return b.String()
}

func TestValidate_MinimalWorkflowIsValid(t *testing.T) {
	diags := checkYAML(t, vHeader+"risk: write\n", testEnv())
	if len(diags) != 0 {
		t.Fatalf("diags:\n%s", diagList(diags))
	}
	// Read-only audit with restricted Beads access.
	diags = checkYAML(t, vHeader+"risk: read\nentry: { agent: auditor }\nbeads: { allow: [show, list] }\nagents:\n  auditor: { role: workflow }\n  auditor-subagent: { role: workflow }\n", testEnv())
	if len(diags) != 0 {
		t.Fatalf("diags:\n%s", diagList(diags))
	}
}

func TestValidate_WithoutEnvSkipsCatalogueRules(t *testing.T) {
	diags := checkYAML(t, vHeader+"risk: read\nagents:\n  ghost: { role: workflow }\nprompt: { template: prompts/x.tmpl }\n", Env{})
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"read_beads_unrestricted"}) {
		t.Fatalf("codes = %v", got)
	}
}

func TestValidate_SkillIssues(t *testing.T) {
	env := testEnv()
	env.Skills = fakeSkills{
		known:  map[string]bool{"a": true},
		issues: []SkillIssue{{Skill: "b", Kind: "missing", Detail: "a"}, {Skill: "a", Kind: "missing", Detail: "a"}, {Skill: "c", Kind: "duplicate", Detail: "x/c, y/c"}},
	}
	diags := checkYAML(t, vHeader+"risk: write\nskills: { extra: [a], deny: [a] }\n", env)
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"skill_closure", "skill_duplicate"}) {
		t.Fatalf("codes = %v", got)
	}
}

func TestValidate_IsolationWarning(t *testing.T) {
	env := testEnv()
	env.Isolation = sessionspec.IsolationPartial
	diags := checkYAML(t, vHeader+"risk: write\nisolation: strict\n", env)
	assertDiag(t, diags, "isolation_unsupported", "isolation", SeverityWarning)
}

func TestValidate_DiagnosticPointsToPatchLayer(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:x":  vHeader + "risk: write\nprompt: { text: go }\n",
		"team:x": vHeader + "extends: hub:x\nagents:\n  ghost: { role: independent }\n",
	})
	_, diags := Check(cat, Ref{LayerTeam, "x"}, nil, testEnv())
	if len(diags) != 1 || diags[0].Source != "team:x.yaml" || diags[0].Pos != (Pos{6, 3}) {
		t.Fatalf("diags = %+v", diags)
	}
}

func TestTemplateVariables(t *testing.T) {
	inputs := map[string]bool{"a": true, "items": true}
	src := `{{ .a }} {{ .oh.project }} {{ range .items }}{{ .name }}{{ end }} {{ with .a }}{{ .x }}{{ $.b }}{{ end }} {{ upper .c }} {{ define "t" }}{{ .d }}{{ end }}`
	got, err := TemplateVariables(src, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"b", "c", "d"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unknown = %v, want %v", got, want)
	}
}

// A10: a team or project workflow without prompt is reported (the session
// would start without any instruction); a patch inherits the prompt.
func TestValidate_PromptNone(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:x":     vHeader + "risk: write\nprompt: { text: go }\n",
		"team:x":    vHeader + "extends: hub:x\n",
		"team:bare": "apiVersion: oh/v1\nkind: Workflow\nid: bare\nrisk: write\n",
	})
	_, diags := Check(cat, Ref{LayerTeam, "x"}, nil, testEnv())
	if len(diags) != 0 {
		t.Fatalf("patch inherits the prompt: %v", diags.Codes())
	}
	_, diags = Check(cat, Ref{LayerTeam, "bare"}, nil, testEnv())
	assertDiag(t, diags, "prompt_none", "prompt", SeverityWarning)
}
