package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
)

func TestWorkflowListAndShow(t *testing.T) {
	svc := &workflowsvc.Service{HubDir: testWorkflowHub(t), Lang: "fr"}
	ctx := context.Background()
	list, err := svc.Catalog(ctx, workflowsvc.Context{})
	if err != nil || len(list) != 1 || !list[0].Valid {
		t.Fatalf("catalog = %+v (%v)", list, err)
	}

	var out bytes.Buffer
	if err := printWorkflowList(&out, list, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "pair") || !strings.Contains(out.String(), "hub") {
		t.Fatalf("list = %q", out.String())
	}
	out.Reset()
	if err := printWorkflowList(&out, nil, true); err != nil || strings.TrimSpace(out.String()) != "[]" {
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
