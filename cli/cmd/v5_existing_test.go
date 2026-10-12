package cmd

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func TestRunOptionsOnExistingFlags(t *testing.T) {
	opts, err := runOptionsFromFlags(runFlagsCmd(t, "--on-existing", "delete", "--discard-base-changes"), "ticket")
	if err != nil {
		t.Fatal(err)
	}
	if opts.OnExisting != runsvc.OnExistingDelete || !opts.DiscardBaseChanges {
		t.Fatalf("opts = %+v", opts)
	}
	if opts, _ = runOptionsFromFlags(runFlagsCmd(t), "ticket"); opts.OnExisting != runsvc.OnExistingUnset {
		t.Fatal("no flag: no choice")
	}
	if _, err := runOptionsFromFlags(runFlagsCmd(t, "--on-existing", "drop"), "ticket"); err == nil {
		t.Fatal("unknown choice refused")
	}
}

func existingRun() *preparedRun {
	e := runsvc.ExistingSession{ID: "ses_old", State: domain.RunIdle, WorkflowID: "ticket", Runtime: "container", LaunchPath: "/p-bd-1",
		Shared: []string{"bd-1"}, SameWorkflow: true, Diffs: []runsvc.FieldDiff{{Field: runsvc.DiffRuntime, Existing: "container", Planned: "local"}},
		Actions: []runsvc.OnExisting{runsvc.OnExistingResume, runsvc.OnExistingParallel}}
	return &preparedRun{
		project:    &domain.Project{Name: "p", Path: "/p"},
		resolution: &workflowsvc.Resolution{Resolved: &workflow.Resolved{Spec: &workflow.Spec{ID: "ticket"}, Mode: "auto"}},
		bundle:     &bundle.Bundle{},
		plan: &runsvc.RunPlan{Request: runsvc.RunRequest{Workflow: runsvc.WorkflowRef{ID: "ticket"}},
			Sessions: []runsvc.PlannedSession{{PlannedInput: runsvc.PlannedInput{Label: "bd-1"}, Location: runsvc.Location{Path: "/p", Kind: runsvc.LocationBase}, Existing: "ses_old"}},
			Existing: []runsvc.ExistingSession{e}, NeedsChoice: true},
	}
}

// Without a terminal, the same workflow on a ticket already worked on needs
// --on-existing; the recap shows the existing session and the need.
func TestExistingChoiceWithoutTerminal(t *testing.T) {
	p := existingRun()
	ok, err := resolveExistingChoice(context.Background(), &strings.Builder{}, p, false)
	if ok || err == nil || !strings.Contains(err.Error(), "--on-existing") || !strings.Contains(err.Error(), "ses_old") {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	rows, warns := runRecap(p)
	found := false
	for _, r := range rows {
		if r[0] == i18n.T("cmd.run.existing.row") && strings.Contains(r[1], "ses_old") && strings.Contains(r[1], i18n.T("cmd.run.existing.field.runtime")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("rows = %v", rows)
	}
	if len(warns) == 0 || !strings.Contains(warns[len(warns)-1], "--on-existing") {
		t.Fatalf("warnings = %v", warns)
	}

	// Another workflow: informational only.
	p.plan.NeedsChoice = false
	p.plan.Existing[0].SameWorkflow = false
	if ok, err := resolveExistingChoice(context.Background(), &strings.Builder{}, p, false); !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	// A choice given by flag goes through.
	p.plan.NeedsChoice, p.opts.OnExisting = true, runsvc.OnExistingResume
	if ok, err := resolveExistingChoice(context.Background(), &strings.Builder{}, p, false); !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestPrintExistingTableMarksDiffs(t *testing.T) {
	var b strings.Builder
	printExistingTable(&b, existingRun().plan)
	out := b.String()
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, i18n.T("cmd.run.existing.field.runtime")) && !strings.Contains(l, "!") {
			t.Fatalf("runtime differs: %q", l)
		}
	}
	if !strings.Contains(out, "container") || !strings.Contains(out, "local") {
		t.Fatal(out)
	}
}

func TestDeleteReportText(t *testing.T) {
	s := deleteReportText(&runsvc.DeleteReport{SessionID: "ses_x", WorktreeRemoved: "/w", BranchDeleted: "feat/x", RemoteBranch: true, Errors: []string{"branch: boom"}})
	for _, want := range []string{"ses_x", "/w", "feat/x", "boom"} {
		if !strings.Contains(s, want) {
			t.Fatalf("%q lacks %q", s, want)
		}
	}
}

// The TUI recap: the existing sessions as a comparison block (not rows),
// the choices of the targeted session.
func TestTUIExistingBlock(t *testing.T) {
	p := existingRun()
	rows, warns := launchRecap(p)
	for _, r := range rows {
		if r[0] == i18n.T("cmd.run.existing.row") {
			t.Fatalf("existing row in the TUI recap: %v", rows)
		}
	}
	for _, w := range warns {
		if strings.Contains(w, "--on-existing") {
			t.Fatalf("CLI hint in the TUI recap: %v", warns)
		}
	}
	got := tuiExisting(p.plan)
	if len(got) != 1 {
		t.Fatalf("existing = %+v", got)
	}
	e := got[0]
	if !e.Target || !e.SameWorkflow || e.ID != "ses_old" || e.Runtime != "container" || e.PlannedRuntime != "local" || e.PlannedPath != "/p" {
		t.Fatalf("existing = %+v", e)
	}
	if !e.Differs(views.LaunchDiffRuntime) || e.Differs(views.LaunchDiffMode) {
		t.Fatalf("diffs = %v", e.Diffs)
	}
	if !e.Allows(views.LaunchExistingResume) || !e.Allows(views.LaunchExistingParallel) || e.Allows(views.LaunchExistingDelete) {
		t.Fatalf("actions = %v", e.Actions)
	}
	// The view constants are the runsvc values.
	for view, svc := range map[string]string{views.LaunchDiffWorkflow: runsvc.DiffWorkflow, views.LaunchDiffVersion: runsvc.DiffVersion,
		views.LaunchDiffRuntime: runsvc.DiffRuntime, views.LaunchDiffMode: runsvc.DiffMode, views.LaunchDiffModel: runsvc.DiffModel,
		views.LaunchDiffLocation: runsvc.DiffLocation, views.LaunchDiffBranch: runsvc.DiffBranch,
		views.LaunchExistingResume: string(runsvc.OnExistingResume), views.LaunchExistingTransfer: string(runsvc.OnExistingTransfer),
		views.LaunchExistingKeep: string(runsvc.OnExistingKeep), views.LaunchExistingDelete: string(runsvc.OnExistingDelete),
		views.LaunchExistingParallel: string(runsvc.OnExistingParallel)} {
		if view != svc {
			t.Fatalf("%q != %q", view, svc)
		}
	}
}

func TestTUILaunchErrorTransfer(t *testing.T) {
	err := tuiLaunchError(&runsvc.TransferError{SessionID: "ses_old", Err: errors.New("boom")})
	if !errors.Is(err, views.ErrTransferFailed) || !strings.Contains(err.Error(), "ses_old") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestDeleteSummary(t *testing.T) {
	s := deleteSummary(&runsvc.DeletePlan{SessionID: "ses_x", Worktree: "/w", Branch: "feat/x", UnpushedCommits: []string{"abc fix"}})
	if !s.Losses || s.BaseChanges != 0 || !strings.Contains(strings.Join(s.Items, "\n"), "abc fix") {
		t.Fatalf("summary = %+v", s)
	}
	s = deleteSummary(&runsvc.DeletePlan{SessionID: "ses_y", InBase: true, ChangedFiles: []string{"a.go", "b.go"}})
	if s.Losses || s.BaseChanges != 2 || !strings.Contains(strings.Join(s.Items, "\n"), i18n.Tf("tui.delete.base_changes", 2)) {
		t.Fatalf("summary = %+v", s)
	}
}

// Another choice for the existing session plans the cached launch again
// (no new preparation).
func TestTUIRunCacheReplansChoice(t *testing.T) {
	base := runOptions{Workflow: "ticket", Tickets: []string{"bd-1"}}
	a, b := base, base
	b.OnExisting, b.DiscardBaseChanges = runsvc.OnExistingDelete, true
	if !reflect.DeepEqual(withoutExistingChoice(a), withoutExistingChoice(b)) {
		t.Fatal("the choice alone does not change the preparation")
	}
}
