package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/adapters"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
)

func TestAliasReplacement(t *testing.T) {
	al := workflowAlias{Workflow: "ticket", Opts: runOptions{Tickets: []string{"bd-1", "bd-2"},
		LooseInputs: map[string]string{"mode": "", "branch": "feat/x", "issue": "it's broken"}, Location: "new"}}
	want := `oh run ticket --tickets bd-1,bd-2 -i branch=feat/x -i issue='it'\''s broken' --location new`
	if got := aliasReplacement(al); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if got := aliasReplacement(workflowAlias{Workflow: "feature"}); got != "oh run feature" {
		t.Fatalf("got %s", got)
	}
}

func startFlagsCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	c := &cobra.Command{Use: "start"}
	addStartFlags(c)
	if err := c.Flags().Parse(args); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestStartAliasFor(t *testing.T) {
	cases := []struct {
		args     []string
		workflow string
		check    func(workflowAlias) bool
	}{
		{nil, "feature", func(a workflowAlias) bool { return a.Opts.Text == "" }},
		{[]string{"-m", "ajoute un export"}, "feature", func(a workflowAlias) bool { return a.Opts.Text == "ajoute un export" }},
		{[]string{"--dev", "-t", "bd-1"}, "ticket", func(a workflowAlias) bool { return true }},
		{[]string{"--onboard", "--refresh"}, "onboarding", func(a workflowAlias) bool { return a.Opts.LooseInputs["refresh"] == "true" }},
		{[]string{"--parallel", "--tickets", "bd-1,bd-2"}, "ticket", func(a workflowAlias) bool { return len(a.Opts.Tickets) == 2 }},
		{[]string{"--sweep", "migrer les logs"}, "sweep", func(a workflowAlias) bool { return a.Opts.Text == "migrer les logs" }},
		{[]string{"-w", "feat/x", "--recap"}, "feature", func(a workflowAlias) bool {
			return a.Recap && a.Opts.Location == "new" && a.Opts.Branch == "feat/x"
		}},
		{[]string{"-a", "debugger", "-m", "regarde le crash"}, "libre", func(a workflowAlias) bool {
			return a.Opts.Agent == "debugger" && a.Opts.Text == "regarde le crash"
		}},
	}
	for _, tc := range cases {
		al := startAliasFor(startFlagsCmd(t, tc.args...))
		if al.Workflow != tc.workflow || !tc.check(al) {
			t.Errorf("%v → %+v", tc.args, al)
		}
	}
	if got := aliasReplacement(startAliasFor(startFlagsCmd(t, "-a", "debugger"))); got != "oh run libre --agent debugger" {
		t.Errorf("replacement = %s", got)
	}
}

// Without a supported tool, `oh start` refuses (no former launch any more);
// the message names the tool of the adapter.
func TestStartRefusesWithoutV2(t *testing.T) {
	withoutV2(t, nil)
	err := requireV2(context.Background())
	if err == nil || !strings.Contains(err.Error(), "faketool") || !strings.Contains(err.Error(), "oh doctor") {
		t.Fatalf("err = %v", err)
	}
	withoutV2(t, &adapters.UnsupportedVersionError{Tool: "faketool", Found: "1.18.29", Min: "2.0.0", Max: "2.99.99"})
	err = requireV2(context.Background())
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
	in := launchInputs(feature.Spec, runOptions{Text: "ajoute un export", LooseInputs: map[string]string{"branch": "feat/x", "refresh": "true"}})
	if len(in) != 1 || in["request"] != "ajoute un export" {
		t.Fatalf("feature inputs = %v", in)
	}
	ticket, err := svc.Resolve(ctx, workflowsvc.Context{}, "ticket", workflowsvc.ResolveOpts{})
	if err != nil {
		t.Fatal(err)
	}
	in = launchInputs(ticket.Spec, runOptions{Inputs: map[string]string{"branch": "explicit"}, Text: "ignored",
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
