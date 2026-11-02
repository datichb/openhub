package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/services/launch"
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
	p := &launch.Prepared{
		Opts:       launch.Options{Epics: []workflowsvc.EpicExpansion{{Epic: "pt-c3b", Children: []string{"pt-c3b.1", "pt-c3b.2"}}}},
		Project:    &domain.Project{Name: "p"},
		Resolution: &workflowsvc.Resolution{Resolved: &workflow.Resolved{Spec: &workflow.Spec{ID: "ticket"}, Mode: "semi-auto"}},
		Bundle:     &bundle.Bundle{Spec: sessionspec.BundleSpec{CodeMode: true, Workflow: &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: cps}}},
		Plan: &runsvc.RunPlan{Request: runsvc.RunRequest{Base: runsvc.StartRequest{BeadsAllow: []string{"show", "close"}}},
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
	if len(warns) != 1 || !strings.Contains(warns[0], "/p-x") || !planDirty(p.Plan) {
		t.Fatalf("warnings = %v", warns)
	}
	if recapBeads(nil) != i18n.T("cmd.run.recap.beads_default") || recapBeads([]string{}) != i18n.T("cmd.run.recap.beads_none") {
		t.Fatal("beads defaults")
	}
	p.Bundle.Spec.Workflow.Checkpoints = nil
	rows, _ = runRecap(p)
	for _, r := range rows {
		if r[0] == i18n.T("cmd.run.recap.checkpoints") {
			t.Fatal("no checkpoint row without checkpoints")
		}
	}
}

// Bug 16: the recap of a review of another branch says it runs in a
// temporary worktree of that branch, and where.
func TestRunRecapTemporaryWorktreeOfAReview(t *testing.T) {
	i18ntest.Set(t, "en")
	wt := "/hub/worktrees/p1/feat-x-ab12cd34"
	p := &launch.Prepared{
		Project:    &domain.Project{Name: "p"},
		Resolution: &workflowsvc.Resolution{Resolved: &workflow.Resolved{Spec: &workflow.Spec{ID: "review"}, Mode: "semi-auto"}},
		Bundle:     &bundle.Bundle{},
		Plan: &runsvc.RunPlan{
			Sessions: []runsvc.PlannedSession{{Location: runsvc.Location{Path: wt, Kind: runsvc.LocationWorktree, Branch: "feat/x", Create: true, Temp: true, Ref: "refs/heads/feat/x"}}},
			Warnings: []runsvc.Warning{{Code: runsvc.WarnReviewWorktree, Args: []any{"feat/x", wt}}}},
	}
	rows, warns := runRecap(p)
	found := false
	for _, r := range rows {
		if r[1] == wt+" (temporary worktree of feat/x)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("rows = %v", rows)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "temporary worktree of feat/x: "+wt) {
		t.Fatalf("warnings = %v", warns)
	}
	if planDirty(p.Plan) {
		t.Fatal("a review worktree is not a dirty directory")
	}
}

// -m is short for -i request=…; both together are refused.
func TestRunOptionsMessage(t *testing.T) {
	opts, err := runOptionsFromFlags(runFlagsCmd(t, "-m", "Ajouter l'export CSV", "-i", "method=tdd"), "feature")
	if err != nil {
		t.Fatal(err)
	}
	if opts.Inputs["request"] != "Ajouter l'export CSV" || !opts.Message || opts.Inputs["method"] != "tdd" {
		t.Fatalf("opts = %+v", opts)
	}
	if _, err := runOptionsFromFlags(runFlagsCmd(t, "-m", "a", "-i", "request=b"), "feature"); err == nil ||
		err.Error() != i18n.T("cmd.run.message_and_request") {
		t.Fatalf("-m with -i request= : %v", err)
	}
	if _, err := runOptionsFromFlags(runFlagsCmd(t, "-m", "  "), "feature"); err == nil {
		t.Fatal("empty -m accepted")
	}
	if opts, _ := runOptionsFromFlags(runFlagsCmd(t, "-i", "request=b"), "feature"); opts.Message {
		t.Fatal("-i request= taken for -m")
	}
}

// The recap names the model the session really uses (the entry agent's, else
// the bundle default) on one row, and the other agents that run another model
// on a second one: two rows at most.
func TestRunRecapModelRows(t *testing.T) {
	sonnet := &sessionspec.ModelRef{Provider: "amazon-bedrock", Model: "anthropic.claude-sonnet-5-5"}
	opus := &sessionspec.ModelRef{Provider: "amazon-bedrock", Model: "anthropic.claude-opus-4-6-v1", Variant: "high"}
	haiku := &sessionspec.ModelRef{Provider: "amazon-bedrock", Model: "anthropic.claude-haiku-4-5-20251001-v1:0"}
	build := func(entry string, def *sessionspec.ModelRef, agents ...sessionspec.AgentDef) *launch.Prepared {
		return &launch.Prepared{
			Project:    &domain.Project{Name: "p"},
			Resolution: &workflowsvc.Resolution{Resolved: &workflow.Resolved{Spec: &workflow.Spec{ID: "ticket"}, Mode: "semi-auto"}},
			Bundle:     &bundle.Bundle{Spec: sessionspec.BundleSpec{EntryAgent: entry, DefaultModel: def, Agents: agents}},
			Plan:       &runsvc.RunPlan{Sessions: []runsvc.PlannedSession{{Location: runsvc.Location{Path: "/p", Kind: runsvc.LocationBase}}}},
		}
	}
	modelRows := func(p *launch.Prepared) (rows [][2]string) {
		all, _ := runRecap(p)
		for i, r := range all {
			if r[0] == i18n.T("cmd.run.recap.model") {
				rows = append(rows, r)
				if i+1 < len(all) && all[i+1][0] == i18n.T("cmd.run.recap.model_others") {
					rows = append(rows, all[i+1])
				}
			}
		}
		return rows
	}

	// One model for everyone: one row, the model of the entry agent.
	rows := modelRows(build("orchestrator", sonnet,
		sessionspec.AgentDef{ID: "orchestrator", Model: sonnet}, sessionspec.AgentDef{ID: "developer"}))
	require.Len(t, rows, 1)
	assert.Equal(t, "amazon-bedrock/anthropic.claude-sonnet-5-5", rows[0][1])

	// The entry agent without a model gets the bundle default.
	rows = modelRows(build("developer", sonnet, sessionspec.AgentDef{ID: "developer"}))
	require.Len(t, rows, 1)
	assert.Equal(t, "amazon-bedrock/anthropic.claude-sonnet-5-5", rows[0][1])

	// Other models: a second row, grouped by model.
	rows = modelRows(build("orchestrator", sonnet,
		sessionspec.AgentDef{ID: "orchestrator", Model: sonnet}, sessionspec.AgentDef{ID: "reviewer", Model: opus},
		sessionspec.AgentDef{ID: "benchmarker", Model: opus}, sessionspec.AgentDef{ID: "developer"}))
	require.Len(t, rows, 2)
	assert.Equal(t, "amazon-bedrock/anthropic.claude-sonnet-5-5", rows[0][1])
	assert.Equal(t, "amazon-bedrock/anthropic.claude-opus-4-6-v1#high (reviewer, benchmarker)", rows[1][1])

	// A third model does not add a third row.
	rows = modelRows(build("orchestrator", sonnet,
		sessionspec.AgentDef{ID: "orchestrator", Model: sonnet}, sessionspec.AgentDef{ID: "reviewer", Model: opus},
		sessionspec.AgentDef{ID: "doc", Model: haiku}))
	require.Len(t, rows, 2)
	assert.Contains(t, rows[1][1], "(reviewer)")
	assert.Contains(t, rows[1][1], "(doc)")

	// No model known (bundle without a default): no row.
	assert.Empty(t, modelRows(build("orchestrator", nil, sessionspec.AgentDef{ID: "orchestrator"})))
}
