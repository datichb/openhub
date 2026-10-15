package hubcat

import (
	"reflect"
	"testing"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// CP-R9: the example of the team guide (docs/guides/team-workflows) is a
// valid team layer over the shipped infra-change.
func TestTeamApproverOverShippedWorkflow(t *testing.T) {
	cat, diags := LoadWorkflows(repoHub)
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	doc, diags := workflow.Parse([]byte(`apiVersion: oh/v1
kind: Workflow
id: infra-change
extends: hub:infra-change
checkpoints:
  cp-apply:
    approver: [Grace Hopper, lead@example.com]
`), workflow.Source{Layer: workflow.LayerTeam, Path: "infra-change.yaml"})
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	if ds := cat.Add(doc); ds != nil {
		t.Fatal(ds)
	}
	c, err := New(repoHub)
	if err != nil {
		t.Fatal(err)
	}
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerTeam, ID: "infra-change"}, nil, c.Env())
	if r == nil || diags.HasErrors() {
		t.Fatalf("team approver: %v", diags)
	}
	for _, d := range diags {
		if d.Path == "checkpoints.cp-apply.approver" {
			t.Fatalf("unexpected: %+v", d)
		}
	}
	cp, _ := r.Spec.Checkpoints.Get("cp-apply")
	if !reflect.DeepEqual([]string(cp.Approver), []string{"Grace Hopper", "lead@example.com"}) {
		t.Fatalf("approver = %v", cp.Approver)
	}
}
