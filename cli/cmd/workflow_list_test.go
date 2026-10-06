package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func TestWorkflowListAndShow(t *testing.T) {
	svc := &workflowsvc.Service{HubDir: testWorkflowHub(t), Lang: "fr"}
	ctx := context.Background()
	ws, err := svc.Workspace(ctx, workflowsvc.Context{})
	if err != nil || len(ws.Workflows) != 1 || !ws.Workflows[0].Valid || ws.Editable {
		t.Fatalf("workspace = %+v (%v)", ws, err)
	}

	var out bytes.Buffer
	if err := printWorkflowList(&out, ws, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "pair") || !strings.Contains(out.String(), "hub") {
		t.Fatalf("list = %q", out.String())
	}
	out.Reset()
	if err := printWorkflowList(&out, &workflowsvc.Workspace{}, true); err != nil || strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("empty json = %q (%v)", out.String(), err)
	}

	res, err := svc.Resolve(ctx, workflowsvc.Context{}, "pair", workflowsvc.ResolveOpts{})
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	printWorkflowShow(&out, res, true)
	for _, want := range []string{"pair", "hub:pair", "lead", "helper", "← hub:pair"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("show lacks %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	if err := printWorkflowShowJSON(&out, res); err != nil {
		t.Fatal(err)
	}
	var decoded workflowShowJSON
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil || decoded.Ref != "hub:pair" || decoded.EntryAgent != "lead" ||
		decoded.Spec["risk"] != "write" || decoded.Origins["risk"].ID != "pair" {
		t.Fatalf("json = %s (%v)", out.String(), err)
	}
}

func TestWorkflowListDraftsAndIntegrity(t *testing.T) {
	ws := &workflowsvc.Workspace{
		Member:    "alice",
		Workflows: []workflowsvc.Summary{{ID: "ticket", Layer: "team", Valid: true, Queued: true}},
		Drafts: []workflowsvc.Summary{{ID: "release", Layer: "team", Draft: true, Errors: 2},
			{ID: "hotfix", Layer: "team", Draft: true, Valid: true, NewBricks: []string{"agent:hotfixer"}}},
		Integrity: workflow.Diagnostics{{Code: "workflow_unlocked", Message: "fichier non publié", Source: "/ts/workflows/published/x.yaml"}},
		Queue:     make([]teamstate.QueuedOp, 1),
	}
	var out bytes.Buffer
	if err := printWorkflowList(&out, ws, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alice", "✎", "release", "2", "agent:hotfixer", "fichier non publié", "x.yaml", "--retry", "⏳"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("list lacks %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	if err := printWorkflowList(&out, ws, true); err != nil {
		t.Fatal(err)
	}
	var decoded []workflowsvc.Summary
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil || len(decoded) != 3 || !decoded[1].Draft {
		t.Fatalf("json = %s (%v)", out.String(), err)
	}
}
