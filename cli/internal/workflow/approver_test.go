package workflow

import (
	"reflect"
	"strings"
	"testing"
)

// CP-R9 step 1: `approver:` is declared by a team layer only, and no other
// layer removes it, changes it nor makes its checkpoint pause less.

const approverTeam = `apiVersion: oh/v1
kind: Workflow
id: ticket
extends: hub:ticket
checkpoints:
  cp-1: { approver: Ada Lovelace, mode: { semi-auto: pause } }
  cp-2: { approver: [Ada Lovelace, lead@example.com] }
`

func approverProject(body string) string {
	return "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: team:ticket\n" + body
}

func approverCheck(t *testing.T, docs map[string]string, ref string) (*Resolved, Diagnostics) {
	t.Helper()
	if _, ok := docs["hub:ticket"]; !ok {
		docs["hub:ticket"] = hubTicket
	}
	r, diags := Check(catalogOf(t, docs), mustRef(t, ref), nil, testEnv())
	if r == nil {
		t.Fatalf("resolution failed: %v", diags)
	}
	return r, diags
}

func approverOf(t *testing.T, r *Resolved, id string) []string {
	t.Helper()
	cp, ok := r.Spec.Checkpoints.Get(id)
	if !ok {
		t.Fatalf("checkpoint %s missing", id)
	}
	return cp.Approver
}

func TestApprover_TeamDeclares(t *testing.T) {
	t.Parallel()
	r, diags := approverCheck(t, map[string]string{"team:ticket": approverTeam}, "team:ticket")
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("team approver: %v", diagList(diags))
	}
	if got := approverOf(t, r, "cp-1"); !reflect.DeepEqual(got, []string{"Ada Lovelace"}) {
		t.Fatalf("cp-1 approver = %v (scalar form)", got)
	}
	if got := approverOf(t, r, "cp-2"); !reflect.DeepEqual(got, []string{"Ada Lovelace", "lead@example.com"}) {
		t.Fatalf("cp-2 approver = %v", got)
	}
	if o, ok := r.Origins.Of("checkpoints.cp-2.approver[1]"); !ok || o.Layer != LayerTeam {
		t.Fatalf("origin = %+v", o)
	}
	// The runtime keeps them (sessionspec, bundle) and a project layer that
	// does not touch them keeps them too.
	r, diags = approverCheck(t, map[string]string{"team:ticket": approverTeam,
		"project:ticket": approverProject("checkpoints:\n  cp-2: { label: Commit projet }\n")}, "project:ticket")
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("project without approver: %v", diagList(diags))
	}
	if got := approverOf(t, r, "cp-2"); len(got) != 2 {
		t.Fatalf("approver lost by an unrelated project patch: %v", got)
	}
}

func TestApprover_OtherLayersRefused(t *testing.T) {
	t.Parallel()
	hubWith := strings.Replace(hubTicket, "  cp-2: { label: Commit,", "  cp-2: { label: Commit, approver: Ada,", 1)
	_, diags := approverCheck(t, map[string]string{"hub:ticket": hubWith}, "hub:ticket")
	assertDiag(t, diags, "checkpoint_approver_layer", "checkpoints.cp-2.approver", SeverityError)

	// A project layer over the hub, or its own workflow.
	project := "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\ncheckpoints:\n  cp-2: { approver: Ada }\n"
	_, diags = approverCheck(t, map[string]string{"project:ticket": project}, "project:ticket")
	assertDiag(t, diags, "checkpoint_approver_layer", "checkpoints.cp-2.approver", SeverityError)

	own := strings.Replace(strings.Replace(hubTicket, "id: ticket", "id: mine", 1), "  cp-2: { label: Commit,", "  cp-2: { label: Commit, approver: [Ada],", 1)
	_, diags = approverCheck(t, map[string]string{"project:mine": own}, "project:mine")
	assertDiag(t, diags, "checkpoint_approver_layer", "checkpoints.cp-2.approver", SeverityError)

	// A team layer may declare it in its own workflow too.
	_, diags = approverCheck(t, map[string]string{"team:mine": own}, "team:mine")
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("team workflow: %v", diagList(diags))
	}
}

func TestApprover_ProjectMayNotLoosen(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, body, code, path string }{
		{"changes it", "checkpoints:\n  cp-2: { approver: Bob }\n", "checkpoint_approver_layer", "checkpoints.cp-2.approver"},
		{"removes it", "checkpoints:\n  cp-2: { approver: [] }\n", "checkpoint_approver_layer", "checkpoints.cp-2.approver"},
		{"writes it again", "checkpoints:\n  cp-2: { approver: [Ada Lovelace, lead@example.com] }\n", "checkpoint_approver_layer", "checkpoints.cp-2.approver"},
		{"disables the checkpoint", "checkpoints:\n  cp-1: { disabled: true }\n", "approver_checkpoint_removed", "checkpoints.cp-1.disabled"},
		{"pauses less", "checkpoints:\n  cp-1: { mode: { semi-auto: auto } }\n", "loosening", "checkpoints.cp-1.mode.semi-auto"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, diags := approverCheck(t, map[string]string{"team:ticket": approverTeam, "project:ticket": approverProject(c.body)}, "project:ticket")
			assertDiag(t, diags, c.code, c.path, SeverityError)
			if got := approverOf(t, r, "cp-2"); !reflect.DeepEqual(got, []string{"Ada Lovelace", "lead@example.com"}) {
				t.Fatalf("cp-2 approver = %v: the team value is kept", got)
			}
			if cp, _ := r.Spec.Checkpoints.Get("cp-1"); cp.Behavior("semi-auto") != BehaviorPause || len(cp.Approver) != 1 {
				t.Fatalf("cp-1 = %+v: kept", cp)
			}
		})
	}
	// A project layer may harden: more pauses.
	_, diags := approverCheck(t, map[string]string{"team:ticket": approverTeam,
		"project:ticket": approverProject("checkpoints:\n  cp-1: { mode: { auto: pause } }\n")}, "project:ticket")
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("hardening: %v", diagList(diags))
	}
	// A team layer over another team document may change it.
	r, diags := approverCheck(t, map[string]string{"team:ticket": approverTeam,
		"team:ticket-hotfix": "apiVersion: oh/v1\nkind: Workflow\nid: ticket-hotfix\nextends: team:ticket\ncheckpoints:\n  cp-2: { approver: Bob }\n"}, "team:ticket-hotfix")
	if errs := errorsOf(diags); len(errs) > 0 {
		t.Fatalf("team patch: %v", diagList(diags))
	}
	if got := approverOf(t, r, "cp-2"); !reflect.DeepEqual(got, []string{"Bob"}) {
		t.Fatalf("team patch approver = %v", got)
	}
}

func TestApprover_Values(t *testing.T) {
	t.Parallel()
	team := func(v string) string {
		return "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\ncheckpoints:\n  cp-2: { approver: " + v + " }\n"
	}
	_, diags := approverCheck(t, map[string]string{"team:ticket": team(`["  "]`)}, "team:ticket")
	assertDiag(t, diags, "checkpoint_approver_invalid", "checkpoints.cp-2.approver[0]", SeverityError)
	_, diags = approverCheck(t, map[string]string{"team:ticket": team(`["Ada\tL"]`)}, "team:ticket")
	assertDiag(t, diags, "checkpoint_approver_invalid", "checkpoints.cp-2.approver[0]", SeverityError)
	_, diags = approverCheck(t, map[string]string{"team:ticket": team(`[Ada Lovelace, "ada  lovelace"]`)}, "team:ticket")
	assertDiag(t, diags, "checkpoint_approver_duplicate", "checkpoints.cp-2.approver[1]", SeverityWarning)
	many := make([]string, MaxApprovers+1)
	for i := range many {
		many[i] = "p" + strings.Repeat("x", i+1)
	}
	_, diags = approverCheck(t, map[string]string{"team:ticket": team("[" + strings.Join(many, ", ") + "]")}, "team:ticket")
	assertDiag(t, diags, "checkpoint_approver_max", "checkpoints.cp-2.approver", SeverityError)
	_, err := Parse([]byte(team("{ name: Ada }")), Source{Layer: LayerTeam, Path: "team:ticket.yaml"})
	if !err.HasErrors() {
		t.Fatal("a mapping is not an approver")
	}

	// Nobody is asked: warned.
	noPause := "apiVersion: oh/v1\nkind: Workflow\nid: ticket\nextends: hub:ticket\ncheckpoints:\n  cp-1: { approver: Ada, mode: { manuel: auto } }\n"
	_, diags = approverCheck(t, map[string]string{"team:ticket": noPause}, "team:ticket")
	assertDiag(t, diags, "checkpoint_approver_no_pause", "checkpoints.cp-1.approver", SeverityWarning)
}

func TestApprover_RemoteAutoWarned(t *testing.T) {
	t.Parallel()
	// cp-1 of hubTicket passes on its own in a remote job.
	_, diags := approverCheck(t, map[string]string{"team:ticket": approverTeam}, "team:ticket")
	assertDiag(t, diags, "checkpoint_approver_remote_auto", "checkpoints.cp-1.approver", SeverityWarning)
	for _, d := range diags {
		if d.Code == "checkpoint_approver_remote_auto" && d.Path == "checkpoints.cp-2.approver" {
			t.Fatalf("cp-2 is deferred: %v", d)
		}
	}
}
