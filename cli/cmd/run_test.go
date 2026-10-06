package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/workflow"
)

func runFlagsCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	c := &cobra.Command{Use: "x"}
	c.Flags().AddFlagSet(runCmd.Flags())
	if err := c.Flags().Parse(args); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRunOptionsFromFlags(t *testing.T) {
	c := runFlagsCmd(t, "-i", "branch=feat/x", "--input", "request=a=b", "--tickets", "bd-1, bd-2,", "--mode", "manuel",
		"--runtime", "container", "--location", "new", "--attach", "none", "--parent", "ses_1")
	opts, err := runOptionsFromFlags(c, "ticket")
	if err != nil {
		t.Fatal(err)
	}
	if opts.Workflow != "ticket" || opts.Inputs["branch"] != "feat/x" || opts.Inputs["request"] != "a=b" ||
		strings.Join(opts.Tickets, ",") != "bd-1,bd-2" || opts.Mode != "manuel" || opts.Runtime != "container" ||
		opts.Location != "new" || opts.Attach != "none" || opts.ParentSessionID != "ses_1" {
		t.Fatalf("opts = %+v", opts)
	}
	if _, err := runOptionsFromFlags(runFlagsCmd(t, "-i", "novalue"), "ticket"); err == nil {
		t.Fatal("input without '=' accepted")
	}
}

func TestSessionBranch(t *testing.T) {
	doc, diags := workflow.Parse([]byte("apiVersion: oh/v1\nkind: Workflow\nid: ticket\ninputs:\n  branch: { type: branch }\n"),
		workflow.Source{Layer: workflow.LayerHub})
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	if got := sessionBranch(doc.Spec, map[string]any{"branch": "feat/bd-1"}, "bd-1"); got != "feat/bd-1" {
		t.Fatalf("branch input: %q", got)
	}
	if got := sessionBranch(doc.Spec, map[string]any{}, "bd-1"); got != "oh/ticket-bd-1" {
		t.Fatalf("fallback: %q", got)
	}
	if got := sessionBranch(doc.Spec, nil, ""); !strings.HasPrefix(got, "oh/ticket-2") {
		t.Fatalf("timestamp fallback: %q", got)
	}
}
