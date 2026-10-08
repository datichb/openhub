package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func runFlagsCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	c := &cobra.Command{Use: "x"}
	addRunFlags(c)
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
	if got := sessionBranch(doc.Spec, map[string]any{"branch": "feat/bd-1"}, "bd-1", "x"); got != "feat/bd-1" {
		t.Fatalf("branch input: %q", got)
	}
	if got := sessionBranch(doc.Spec, map[string]any{}, "bd-1", ""); got != "oh/ticket-bd-1" {
		t.Fatalf("fallback: %q", got)
	}
	if got := sessionBranch(doc.Spec, map[string]any{}, "bd-1", "feat/w"); got != "feat/w" {
		t.Fatalf("explicit fallback: %q", got)
	}
	if got := sessionBranch(doc.Spec, nil, "", ""); !strings.HasPrefix(got, "oh/ticket-2") {
		t.Fatalf("timestamp fallback: %q", got)
	}
}

func TestPreconditionWarnings(t *testing.T) {
	doc, diags := workflow.Parse([]byte(`apiVersion: oh/v1
kind: Workflow
id: feature
preconditions:
  context:
    label: Contexte projet
    check: { path_exists: [docs/wiki] }
    on_fail: suggest
    suggest: { workflow: onboarding, resume: true }
  config:
    check: { path_exists: [oh.toml] }
    on_fail: block
`), workflow.Source{Layer: workflow.LayerHub})
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	dir := t.TempDir()
	if _, _, err := preconditionWarnings(doc.Spec, dir); err == nil {
		t.Fatal("failed block precondition must refuse the launch")
	}
	if err := os.WriteFile(filepath.Join(dir, "oh.toml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	warns, sugg, err := preconditionWarnings(doc.Spec, dir)
	if err != nil || len(warns) != 1 || warns[0].Code != "precondition" || warns[0].Args[1] != "onboarding" {
		t.Fatalf("warns = %+v (%v)", warns, err)
	}
	if len(sugg) != 1 || sugg[0].Workflow != "onboarding" || !sugg[0].Resume || sugg[0].Label != "Contexte projet" {
		t.Fatalf("suggestions = %+v", sugg)
	}
}

func TestSelectMCP(t *testing.T) {
	avail := []sessionspec.MCPServerDef{{Name: "gitlab"}, {Name: "team"}, {Name: "figma"}}
	kept, missing := selectMCP(avail, []string{"team", "gitlab", "jira", "workflow"})
	if len(kept) != 2 || kept[0].Name != "gitlab" || kept[1].Name != "team" {
		t.Fatalf("kept = %+v (project order)", kept)
	}
	if len(missing) != 1 || missing[0] != "jira" {
		t.Fatalf("missing = %v (workflow is the oh runtime server, never missing)", missing)
	}
	if kept, _ := selectMCP(avail, nil); len(kept) != 0 {
		t.Fatal("an explicit empty selection keeps nothing")
	}
}

func TestRunOptionsDirtyFlags(t *testing.T) {
	opts, err := runOptionsFromFlags(runFlagsCmd(t, "--stash"), "ticket")
	if err != nil || opts.Dirty != runsvc.DirtyStash {
		t.Fatalf("--stash: %+v %v", opts.Dirty, err)
	}
	opts, err = runOptionsFromFlags(runFlagsCmd(t, "--allow-dirty"), "ticket")
	if err != nil || opts.Dirty != runsvc.DirtyAllow {
		t.Fatalf("--allow-dirty: %+v %v", opts.Dirty, err)
	}
	if opts, _ = runOptionsFromFlags(runFlagsCmd(t), "ticket"); opts.Dirty != runsvc.DirtyWorktree {
		t.Fatalf("default: %q", opts.Dirty)
	}
	if _, err := runOptionsFromFlags(runFlagsCmd(t, "--stash", "--allow-dirty"), "ticket"); err == nil {
		t.Fatal("--stash with --allow-dirty accepted")
	}
}

// A35: the recap shows the checkpoints in the session mode, the Beads
// commands and Code Mode; the dirty choice is offered when a writer starts
// in a modified directory.
func TestRunRecapRows(t *testing.T) {
	cps := []sessionspec.CheckpointDef{
		{ID: "cp-1", Behaviors: map[string]string{"semi-auto": "auto"}},
		{ID: "cp-2", Behaviors: map[string]string{"semi-auto": "skip"}, Mandatory: true},
	}
	p := &preparedRun{
		opts:       runOptions{epics: []workflowsvc.EpicExpansion{{Epic: "pt-c3b", Children: []string{"pt-c3b.1", "pt-c3b.2"}}}},
		project:    &domain.Project{Name: "p"},
		resolution: &workflowsvc.Resolution{Resolved: &workflow.Resolved{Spec: &workflow.Spec{ID: "ticket"}, Mode: "semi-auto"}},
		bundle:     &bundle.Bundle{Spec: sessionspec.BundleSpec{CodeMode: true, Workflow: &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: cps}}},
		plan: &runsvc.RunPlan{Request: runsvc.RunRequest{Base: runsvc.StartRequest{BeadsAllow: []string{"show", "close"}}},
			Sessions: []runsvc.PlannedSession{{Location: runsvc.Location{Path: "/p", Kind: runsvc.LocationBase}}},
			Warnings: []runsvc.Warning{{Code: runsvc.WarnDirtyWorktree, Args: []any{"/p", "/p-x"}}}},
	}
	rows, warns := runRecap(p)
	got := map[string]string{}
	for _, r := range rows {
		got[r[0]] = r[1]
	}
	want := map[string]string{
		i18n.T("cmd.run.recap.code_mode"):   "on",
		i18n.T("cmd.run.recap.beads"):       "show · close",
		i18n.T("cmd.run.recap.epic"):        "pt-c3b → pt-c3b.1 · pt-c3b.2",
		i18n.T("cmd.run.recap.checkpoints"): "cp-1 " + i18n.T("cmd.run.recap.cp.auto") + " · cp-2 " + i18n.T("cmd.run.recap.cp.pause") + " (" + i18n.T("cmd.run.recap.cp.mandatory") + ")",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "/p-x") || !planDirty(p.plan) {
		t.Fatalf("warnings = %v", warns)
	}
	if recapBeads(nil) != i18n.T("cmd.run.recap.beads_default") || recapBeads([]string{}) != i18n.T("cmd.run.recap.beads_none") {
		t.Fatal("beads defaults")
	}
	p.bundle.Spec.Workflow.Checkpoints = nil
	rows, _ = runRecap(p)
	for _, r := range rows {
		if r[0] == i18n.T("cmd.run.recap.checkpoints") {
			t.Fatal("no checkpoint row without checkpoints")
		}
	}
}
