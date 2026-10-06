package workflow

import (
	"reflect"
	"testing"
)

const teamTicketEnforced = `apiVersion: oh/v1
kind: Workflow
id: ticket
extends: hub:ticket
enforce: [checkpoints, modes]
checkpoints:
  cp-1: { label: Team start }
`

func TestEnforce_BlocksLockedFields(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:ticket":  hubTicket,
		"team:ticket": teamTicketEnforced,
		"project:ticket": `apiVersion: oh/v1
kind: Workflow
id: ticket
extends: team:ticket
description: Project ticket
checkpoints:
  cp-1: { label: Project start }
modes: { default: manuel }
`,
	})
	r, diags := ResolveSpec(cat, mustRef(t, "project:ticket"), nil)
	if r == nil {
		t.Fatalf("no result: %v", diags)
	}
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"enforced_field", "enforced_field"}) {
		t.Fatalf("codes = %v", got)
	}
	if diags[0].Path != "checkpoints" || diags[1].Path != "modes" {
		t.Fatalf("paths = %s, %s", diags[0].Path, diags[1].Path)
	}
	if diags[0].Source != "project:ticket.yaml" || diags[0].Pos.IsZero() {
		t.Fatalf("diagnostic not located: %+v", diags[0])
	}
	cp, _ := r.Spec.Checkpoints.Get("cp-1")
	if cp.Label.Text("en") != "Team start" {
		t.Fatalf("locked checkpoint changed: %q", cp.Label.Text("en"))
	}
	if r.Spec.Modes.Default != "semi-auto" {
		t.Fatalf("locked modes changed: %q", r.Spec.Modes.Default)
	}
	if r.Spec.Description.Text("en") != "Project ticket" {
		t.Fatalf("unlocked field not applied: %q", r.Spec.Description.Text("en"))
	}
}

func TestEnforce_AllLocksEverythingButIdentity(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:ticket": hubTicket,
		"team:ticket": `apiVersion: oh/v1
kind: Workflow
id: ticket
extends: hub:ticket
enforce: ["*"]
`,
		"project:ticket": `apiVersion: oh/v1
kind: Workflow
id: ticket
version: 4
extends: team:ticket
enforce: [models]
risk: read
`,
	})
	r, diags := ResolveSpec(cat, mustRef(t, "project:ticket"), nil)
	if got := diags.Codes(); !reflect.DeepEqual(got, []string{"enforced_field"}) || diags[0].Path != "risk" {
		t.Fatalf("diags = %v", diags)
	}
	if r.Spec.Risk != RiskWrite {
		t.Fatalf("risk = %s", r.Spec.Risk)
	}
	if !reflect.DeepEqual(r.Spec.Enforce, []string{"*", "models"}) {
		t.Fatalf("enforce = %v", r.Spec.Enforce)
	}
}

func TestEnforce_AccumulatesAndKeepsParentFree(t *testing.T) {
	cat := catalogOf(t, map[string]string{
		"hub:ticket":  hubTicket,
		"team:ticket": teamTicketEnforced,
	})
	r := resolveOK(t, cat, "team:ticket")
	if !reflect.DeepEqual(r.Spec.Enforce, []string{"checkpoints", "modes"}) {
		t.Fatalf("enforce = %v", r.Spec.Enforce)
	}
	// The document that sets a lock may still write the locked field.
	cp, _ := r.Spec.Checkpoints.Get("cp-1")
	if cp.Label.Text("en") != "Team start" {
		t.Fatalf("label = %q", cp.Label.Text("en"))
	}
}

func TestEnforce_UnknownFieldIsAnError(t *testing.T) {
	cat := catalogOf(t, map[string]string{"hub:ticket": hubTicket + "enforce: [checkpoints, nope]\n"})
	_, diags := Check(cat, mustRef(t, "hub:ticket"), nil, Env{})
	var found bool
	for _, d := range diags {
		if d.Code == "enforce_unknown_field" {
			found = d.Path == "enforce[1]" && d.Severity == SeverityError
		}
	}
	if !found {
		t.Fatalf("enforce_unknown_field not reported: %v", diags)
	}
}

func TestEnforceableFields(t *testing.T) {
	fields := EnforceableFields()
	for _, want := range []string{"checkpoints", "risk", "limits", "inputs"} {
		if !IsEnforceable(want) {
			t.Errorf("%s not enforceable (%v)", want, fields)
		}
	}
	for _, no := range []string{"id", "extends", "enforce", "version"} {
		if IsEnforceable(no) {
			t.Errorf("%s must not be enforceable", no)
		}
	}
	if !IsEnforceable(EnforceAll) {
		t.Error("* not enforceable")
	}
}
