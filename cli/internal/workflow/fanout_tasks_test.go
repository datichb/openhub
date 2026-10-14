package workflow

import (
	"strings"
	"testing"
)

// sweep-fanout: a fan-out whose children are the tasks oh recorded at the
// tasks checkpoint (source: tasks), and the breakdown check (tasks:). Each
// guard has its case, the attempts to get around it from a layer included.

const tasksChild = `apiVersion: oh/v1
kind: Workflow
id: task-child
risk: write
inputs:
  task: { type: text, required: true }
  files: { type: text, required: true }
  domain: { type: enum, values: [backend, tests] }
  goal: { type: text }
checkpoints:
  cp-commit: { mandatory: true, unlocks: [commit], verify: { from: stack, expect: pass }, mode: { manuel: pause, semi-auto: pause, auto: pause } }
`

const tasksParent = vHeader + `risk: write
inputs:
  goal: { type: text, required: true }
  fanout: { type: bool, default: false }
agents:
  developer: { role: workflow, after: cp-plan, fanout: never }
fanout:
  input: fanout
  source: tasks
  child: task-child
  merge: cp-merge
  delegates: [cp-commit]
  inputs: [goal]
checkpoints:
  cp-plan:
    mandatory: true
    mode: { manuel: pause, semi-auto: pause, auto: pause }
    tasks: { max: 5, domains: [backend, tests] }
  cp-commit: { fanout: never, mandatory: true, unlocks: [commit], verify: { from: stack, expect: pass }, mode: { manuel: pause, semi-auto: pause, auto: pause } }
  cp-merge:
    fanout: only
    mandatory: true
    unlocks: [merge]
    verify: { from: stack, expect: pass, on: integration }
    mode: { manuel: pause, semi-auto: pause, auto: pause }
outputs:
  - { id: tasks, type: text, record: tasks }
`

func tasksCheck(t *testing.T, docs map[string]string, ref Ref, opts *SessionOptions) (*Resolved, Diagnostics) {
	t.Helper()
	if _, ok := docs["hub:task-child"]; !ok {
		docs["hub:task-child"] = tasksChild
	}
	return fanoutCheck(t, docs, ref, opts)
}

func TestFanoutTasks_HubDeclarationIsValid(t *testing.T) {
	r, diags := tasksCheck(t, map[string]string{"hub:x": tasksParent}, Ref{LayerHub, "x"}, nil)
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("valid: %v", diagList(diags))
	}
	if !r.Spec.Fanout.Tasks() || !r.Spec.SubstitutesControl() || r.Spec.TasksCheckpoint() != "cp-plan" {
		t.Fatalf("tasks fan-out: %s", r.Spec.Fanout)
	}
	cp, _ := r.Spec.Checkpoints.Get("cp-plan")
	if cp.Tasks.EffectiveMax() != 5 || strings.Join(cp.Tasks.Domains, ",") != "backend,tests" {
		t.Fatalf("tasks check: %+v", cp.Tasks)
	}
}

// The session without the fan-out input keeps its developer and its
// cp-commit; the fan-out session loses both (the children do the work).
func TestFanoutTasks_SessionSeesItsAgentsAndCheckpoints(t *testing.T) {
	docs := map[string]string{"hub:x": tasksParent}
	r, _ := tasksCheck(t, docs, Ref{LayerHub, "x"}, &SessionOptions{Inputs: map[string]any{"goal": "g"}})
	if got := strings.Join(r.Spec.Checkpoints.Keys(), ","); got != "cp-plan,cp-commit" {
		t.Fatalf("standard session: %s", got)
	}
	if _, ok := r.Spec.Agents.Get("developer"); !ok {
		t.Fatal("the developer works in the standard session")
	}
	r, _ = tasksCheck(t, docs, Ref{LayerHub, "x"}, &SessionOptions{Inputs: map[string]any{"goal": "g", "fanout": true}})
	if got := strings.Join(r.Spec.Checkpoints.Keys(), ","); got != "cp-plan,cp-merge" {
		t.Fatalf("fan-out session: %s", got)
	}
	if _, ok := r.Spec.Agents.Get("developer"); ok {
		t.Fatal("fanout: never removes the developer from the fan-out session")
	}
}

func TestFanoutTasks_Guarantees(t *testing.T) {
	cases := []struct{ name, from, to, code, path string }{
		{"tickets as well", "  source: tasks\n", "  source: tasks\n  tickets: goal\n", "fanout_tasks_tickets", "fanout.tickets"},
		{"unknown source", "  source: tasks\n", "  source: files\n", "enum_invalid", "fanout.source"},
		{"no tasks checkpoint", "    tasks: { max: 5, domains: [backend, tests] }\n", "", "fanout_tasks_checkpoint", "fanout.source"},
		{"tasks checkpoint not mandatory", "  cp-plan:\n    mandatory: true\n", "  cp-plan:\n", "fanout_tasks_mandatory", "checkpoints.cp-plan.mandatory"},
		{"tasks checkpoint of the fan-out only", "  cp-plan:\n    mandatory: true\n", "  cp-plan:\n    fanout: only\n    mandatory: true\n", "fanout_tasks_scope", "checkpoints.cp-plan.fanout"},
		{"reserved input given", "  inputs: [goal]\n", "  inputs: [goal, task]\n", "fanout_inputs_reserved", "fanout.inputs[1]"},
		{"too many tasks", "max: 5", "max: 21", "tasks_max", "checkpoints.cp-plan.tasks.max"},
		{"domain twice", "domains: [backend, tests]", "domains: [backend, backend]", "duplicate_entry", "checkpoints.cp-plan.tasks.domains[1]"},
		{"domain unknown to the child", "domains: [backend, tests]", "domains: [backend, frontend]", "fanout_task_domain", "fanout.child"},
		{"agent scope on the entry", "agents:\n", "entry: { agent: developer }\nagents:\n", "agent_fanout_entry", "agents.developer.fanout"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := strings.Replace(tasksParent, c.from, c.to, 1)
			if doc == tasksParent {
				t.Fatalf("replacement not applied: %q", c.from)
			}
			_, diags := tasksCheck(t, map[string]string{"hub:x": doc}, Ref{LayerHub, "x"}, nil)
			assertDiag(t, diags, c.code, c.path, SeverityError)
		})
	}
	// The tasks checkpoint after the merge checkpoint: the children would
	// never start.
	moved := strings.Replace(tasksParent, "  cp-plan:\n    mandatory: true\n    mode: { manuel: pause, semi-auto: pause, auto: pause }\n    tasks: { max: 5, domains: [backend, tests] }\n", "", 1)
	moved = strings.Replace(moved, "outputs:\n", "  cp-plan:\n    mandatory: true\n    mode: { manuel: pause, semi-auto: pause, auto: pause }\n    tasks: { max: 5 }\noutputs:\n", 1)
	_, diags := tasksCheck(t, map[string]string{"hub:x": moved}, Ref{LayerHub, "x"}, nil)
	assertDiag(t, diags, "fanout_tasks_after_merge", "checkpoints.cp-plan", SeverityError)

	// The child must take the task and its scope.
	for _, in := range []string{"  task: { type: text, required: true }\n", "  files: { type: text, required: true }\n"} {
		child := strings.Replace(tasksChild, in, "", 1)
		_, diags := tasksCheck(t, map[string]string{"hub:x": tasksParent, "hub:task-child": child}, Ref{LayerHub, "x"}, nil)
		assertDiag(t, diags, "fanout_task_input", "fanout.child", SeverityError)
	}
	// Two breakdowns in one workflow; a recorded breakdown without check.
	two := strings.Replace(tasksParent, "  cp-commit: { fanout: never,", "  cp-commit: { tasks: {}, fanout: never,", 1)
	_, diags = tasksCheck(t, map[string]string{"hub:x": two}, Ref{LayerHub, "x"}, nil)
	assertDiag(t, diags, "tasks_duplicate", "checkpoints.cp-commit.tasks", SeverityError)
	diags = checkYAML(t, vHeader+"risk: read\nbeads: { allow: [] }\noutputs:\n  - { id: tasks, type: text, record: tasks }\n", testEnv())
	assertDiag(t, diags, "output_record_tasks_unchecked", "outputs[0].record", SeverityError)
	// An agent scope without fan-out.
	diags = checkYAML(t, vHeader+"risk: write\nagents:\n  developer: { role: workflow, fanout: never }\n", testEnv())
	assertDiag(t, diags, "agent_fanout_without_fanout", "agents.developer.fanout", SeverityError)
}

// A team or project layer may only harden the breakdown and the tasks
// fan-out: fewer tasks, fewer domains, fewer children at once.
func TestFanoutTasks_PatchMayOnlyHarden(t *testing.T) {
	patch := func(body string) string { return "apiVersion: oh/v1\nkind: Workflow\nid: x\nextends: hub:x\n" + body }
	cases := []struct{ name, body, path string }{
		{"back to tickets", "fanout: { source: tickets }\n", "fanout.source"},
		{"tasks check removed", "checkpoints:\n  cp-plan: { tasks: null }\n", "checkpoints.cp-plan.tasks"},
		{"more tasks", "checkpoints:\n  cp-plan: { tasks: { max: 12 } }\n", "checkpoints.cp-plan.tasks.max"},
		{"another domain", "checkpoints:\n  cp-plan: { tasks: { domains: [backend, tests, frontend] } }\n", "checkpoints.cp-plan.tasks.domains"},
		{"developer kept in the fan-out", "agents:\n  developer: { fanout: \"\" }\n", "agents.developer.fanout"},
		{"commit delegated elsewhere", "fanout: { delegates: [cp-plan] }\n", "fanout.delegates"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, diags := tasksCheck(t, map[string]string{"hub:x": tasksParent, "team:x": patch(c.body)}, Ref{LayerTeam, "x"}, nil)
			assertDiag(t, diags, "loosening", c.path, SeverityError)
		})
	}
	r, diags := tasksCheck(t, map[string]string{"hub:x": tasksParent, "team:x": patch("checkpoints:\n  cp-plan: { tasks: { max: 3, domains: [tests] } }\n")}, Ref{LayerTeam, "x"}, nil)
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("narrower breakdown: %v", diagList(diags))
	}
	cp, _ := r.Spec.Checkpoints.Get("cp-plan")
	if cp.Tasks.EffectiveMax() != 3 || strings.Join(cp.Tasks.Domains, ",") != "tests" || !r.Spec.Fanout.Tasks() {
		t.Fatalf("hardened: %+v %s", cp.Tasks, r.Spec.Fanout)
	}
}

// A team layer that hardens the checkpoint the children pass (cp-commit,
// fanout: never) keeps its hardening outside a fan-out, and is warned that
// a fan-out session does not have that checkpoint.
func TestFanoutTasks_NeverPatchedWarned(t *testing.T) {
	patch := "apiVersion: oh/v1\nkind: Workflow\nid: x\nextends: hub:x\ncheckpoints:\n  cp-commit: { verify: { from: stack, expect: pass, kinds: [build, test, typecheck, lint] } }\n"
	r, diags := tasksCheck(t, map[string]string{"hub:x": tasksParent, "team:x": patch}, Ref{LayerTeam, "x"}, nil)
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("hardening: %v", diagList(diags))
	}
	assertDiag(t, diags, "fanout_never_patched", "checkpoints.cp-commit", SeverityWarning)
	cp, _ := r.Spec.Checkpoints.Get("cp-commit")
	if len(cp.Verify.Kinds) != 4 {
		t.Fatalf("the hardening stays outside a fan-out: %+v", cp.Verify)
	}
	_, diags = tasksCheck(t, map[string]string{"hub:x": tasksParent}, Ref{LayerHub, "x"}, nil)
	for _, d := range diags {
		if d.Code == "fanout_never_patched" {
			t.Fatalf("the hub layer itself is not warned: %v", d)
		}
	}
}
