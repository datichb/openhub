package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/services/launch"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
)

// Without a supported tool, a launch is refused; the message names the tool
// of the adapter.
func TestLaunchRefusesWithoutTool(t *testing.T) {
	withoutTool(t, nil)
	err := requireSupportedTool(context.Background())
	if err == nil || !strings.Contains(err.Error(), "faketool") || !strings.Contains(err.Error(), "oh doctor") {
		t.Fatalf("err = %v", err)
	}
	withoutTool(t, &adapters.UnsupportedVersionError{Tool: "faketool", Found: "1.18.29", Min: "2.0.0", Max: "2.99.99"})
	err = requireSupportedTool(context.Background())
	if err == nil || !strings.Contains(err.Error(), "1.18.29") || !strings.Contains(err.Error(), "oh doctor") {
		t.Fatalf("err = %v", err)
	}
}

// The alias targets of the test workflows (services/workflow/testdata)
// receive the mapped inputs: declared ones only, free text in the first
// text input.
func TestLaunchInputsOnTestWorkflows(t *testing.T) {
	svc := &workflowsvc.Service{HubDir: repoRoot(t), HubWorkflowsDir: filepath.Join("..", "internal", "services", "workflow", "testdata", "workflows")}
	ctx := context.Background()
	feature, err := svc.Resolve(ctx, workflowsvc.Context{}, "feature", workflowsvc.ResolveOpts{})
	if err != nil {
		t.Fatal(err)
	}
	in := launch.LaunchInputs(feature.Spec, launch.Options{Text: "ajoute un export", LooseInputs: map[string]string{"branch": "feat/x", "refresh": "true"}})
	if len(in) != 1 || in["request"] != "ajoute un export" {
		t.Fatalf("feature inputs = %v", in)
	}
	ticket, err := svc.Resolve(ctx, workflowsvc.Context{}, "ticket", workflowsvc.ResolveOpts{})
	if err != nil {
		t.Fatal(err)
	}
	in = launch.LaunchInputs(ticket.Spec, launch.Options{Inputs: map[string]string{"branch": "explicit"}, Text: "ignored",
		LooseInputs: map[string]string{"branch": "loose"}})
	if len(in) != 1 || in["branch"] != "explicit" {
		t.Fatalf("ticket inputs = %v", in)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "agents")); err != nil {
		t.Skip("hub content not found")
	}
	return root
}

type epicSource map[string][]string

func (e epicSource) IsEpic(id string) (bool, error) { _, ok := e[id]; return ok, nil }
func (e epicSource) OpenChildren(id string) ([]string, error) {
	return e[id], nil
}

// A24: `oh run ticket --tickets <epic>` starts one session per open child
// of the epic (one session with every child with --one-session); the recap
// names the epic.
func TestLaunchTicketsExpandsEpics(t *testing.T) {
	svc := &workflowsvc.Service{HubDir: repoRoot(t), HubWorkflowsDir: filepath.Join("..", "internal", "services", "workflow", "testdata", "workflows")}
	ticket, err := svc.Resolve(context.Background(), workflowsvc.Context{}, "ticket", workflowsvc.ResolveOpts{})
	if err != nil {
		t.Fatal(err)
	}
	src := epicSource{"pt-c3b": {"pt-c3b.1", "pt-c3b.2"}, "pt-empty": nil}

	opts := launch.Options{Workflow: "ticket", Tickets: []string{"pt-c3b"}}
	in := map[string]any{}
	input, per, err := launch.LaunchTickets(ticket.Spec, &opts, in, src)
	if err != nil || input != "ticket" || strings.Join(per, ",") != "pt-c3b.1,pt-c3b.2" || in["ticket"] != "pt-c3b.1" {
		t.Fatalf("per session: %q %v %v %v", input, per, in, err)
	}
	if len(opts.Epics) != 1 || opts.Epics[0].Epic != "pt-c3b" {
		t.Fatalf("epics = %+v", opts.Epics)
	}

	opts = launch.Options{Workflow: "ticket", Tickets: []string{"pt-c3b"}, OneSession: true}
	in = map[string]any{}
	if _, per, err = launch.LaunchTickets(ticket.Spec, &opts, in, src); err != nil || per != nil || in["ticket"] != "pt-c3b.1,pt-c3b.2" {
		t.Fatalf("one session: %v %v %v", per, in, err)
	}

	opts = launch.Options{Workflow: "ticket", Tickets: []string{"pt-empty"}}
	if _, _, err = launch.LaunchTickets(ticket.Spec, &opts, map[string]any{}, src); err == nil || !strings.Contains(err.Error(), "pt-empty") {
		t.Fatalf("empty epic: %v", err)
	}
}
