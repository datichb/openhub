package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/huh"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/services/launch"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Launch on a ticket already worked on (runsvc.RunPlan.Existing): the CLI
// shows the existing sessions next to the planned one and asks what to do
// (--on-existing otherwise). The TUI recap uses the same plan fields.

// existingRecapRows are the recap rows of the existing sessions (oh run
// --recap, TUI launch form).
func existingRecapRows(plan *runsvc.RunPlan) [][2]string {
	var rows [][2]string
	for _, e := range plan.Existing {
		v := fmt.Sprintf("%s · %s · %s · %s", e.ID, e.WorkflowID, e.State, e.LaunchPath)
		if len(e.Diffs) > 0 {
			fields := make([]string, len(e.Diffs))
			for i, d := range e.Diffs {
				fields[i] = i18n.T("cmd.run.existing.field." + d.Field)
			}
			v += " · " + theme.IconWarning + " " + strings.Join(fields, ", ")
		}
		rows = append(rows, [2]string{i18n.T("cmd.run.existing.row"), v})
	}
	return rows
}

// printExistingTable prints each existing session next to the planned one,
// the differing fields marked.
func printExistingTable(w io.Writer, plan *runsvc.RunPlan) {
	for _, e := range plan.Existing {
		ps := plan.Sessions[e.Planned]
		fmt.Fprintf(w, "\n%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.run.existing.title", e.ID, strings.Join(e.Shared, ", ")))
		tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
		fmt.Fprintf(tw, "  \t%s\t%s\n", i18n.T("cmd.run.existing.col.existing"), i18n.T("cmd.run.existing.col.new"))
		req := plan.Request
		row := func(field, existing, planned string) {
			mark := " "
			if e.Differs(field) {
				mark = theme.IconWarning
			}
			fmt.Fprintf(tw, "%s %s\t%s\t%s\n", mark, i18n.T("cmd.run.existing.field."+field), dash(existing), dash(planned))
		}
		version := func(v int) string {
			if v == 0 {
				return ""
			}
			return fmt.Sprintf("v%d", v)
		}
		bundleHash := ""
		if req.Base.Bundle != nil {
			bundleHash = req.Base.Bundle.Spec.Hash
		}
		row("state", string(e.State), i18n.T("cmd.run.existing.new_session"))
		row(runsvc.DiffWorkflow, e.WorkflowID, req.Workflow.ID)
		row(runsvc.DiffVersion, version(e.WorkflowVersion), version(req.Workflow.Version))
		row(runsvc.DiffBundle, shortHash(e.BundleHash), shortHash(bundleHash))
		row(runsvc.DiffRuntime, e.Runtime, plannedField(e, runsvc.DiffRuntime, e.Runtime))
		row(runsvc.DiffMode, e.Mode, plannedField(e, runsvc.DiffMode, e.Mode))
		row(runsvc.DiffProvider, e.Provider, plannedField(e, runsvc.DiffProvider, e.Provider))
		row(runsvc.DiffModel, e.Model, plannedField(e, runsvc.DiffModel, e.Model))
		row(runsvc.DiffLocation, e.LaunchPath, ps.Location.Path)
		row(runsvc.DiffBranch, e.Branch, ps.Location.Branch)
		work := i18n.T("cmd.run.existing.work_unknown")
		if e.Work.Known {
			work = i18n.Tf("cmd.run.existing.work_value", e.Work.CommitsAhead, e.Work.CommitsUnpushed, e.Work.ChangedFiles)
		}
		row("work", work, "")
		row("decisions", fmt.Sprint(e.OpenDecisions), "")
		row("cost", fmt.Sprintf("$%.2f", e.Cost), "")
		row("activity", e.LastActivity.Format("2006-01-02 15:04"), "")
		_ = tw.Flush()
	}
}

// plannedField is the planned value of a field: its diff, else the same
// value as the existing session.
func plannedField(e runsvc.ExistingSession, field, same string) string {
	for _, d := range e.Diffs {
		if d.Field == field {
			return d.Planned
		}
	}
	return same
}

// existingTarget is the existing session a choice applies to (the first
// planned session's, for the question).
func existingTarget(plan *runsvc.RunPlan) (runsvc.ExistingSession, bool) {
	for _, ps := range plan.Sessions {
		if ps.Existing == "" {
			continue
		}
		for _, e := range plan.Existing {
			if e.ID == ps.Existing {
				return e, true
			}
		}
	}
	return runsvc.ExistingSession{}, false
}

// askExistingChoice asks what to do with the existing sessions of a
// launch; ok is false when the user cancelled.
func askExistingChoice(ctx context.Context, out io.Writer, p *preparedRun) (choice runsvc.OnExisting, discardBase, ok bool, err error) {
	e, found := existingTarget(p.Plan)
	if !found {
		return runsvc.OnExistingParallel, false, true, nil
	}
	var opts []huh.Option[string]
	for _, c := range runsvc.OnExistingChoices {
		if e.Allows(c) {
			opts = append(opts, huh.NewOption(i18n.T("cmd.run.existing.choice."+string(c)), string(c)))
		}
	}
	opts = append(opts, huh.NewOption(i18n.T("cmd.run.existing.choice.cancel"), "cancel"))
	v := string(runsvc.OnExistingResume)
	if !e.SameWorkflow {
		v = string(runsvc.OnExistingParallel)
	}
	form := theme.NewForm(huh.NewGroup(huh.NewSelect[string]().Title(i18n.Tf("cmd.run.existing.ask", e.ID)).Options(opts...).Value(&v)))
	if err := form.Run(); err != nil {
		return "", false, false, err
	}
	if v == "cancel" {
		return "", false, false, nil
	}
	choice = runsvc.OnExisting(v)
	if choice == runsvc.OnExistingDelete {
		discardBase, ok, err = confirmDelete(ctx, out, p.Svc, e.ID, p.Project.Path, false)
		return choice, discardBase, ok, err
	}
	return choice, false, true, nil
}

// confirmDelete shows what deleting a session loses and asks for
// confirmation (twice for the changes of the project directory unless
// discardBase is already given). It returns the discard choice.
func confirmDelete(ctx context.Context, out io.Writer, svc *runsvc.Service, id, projectPath string, discardBase bool) (discard, ok bool, err error) {
	plan, err := svc.PlanDelete(ctx, id, runsvc.DeleteOptions{ProjectPath: projectPath, DiscardBaseChanges: discardBase})
	if err != nil {
		return false, false, err
	}
	printDeletePlan(out, plan, discardBase)
	// confirmPrompt, not confirmAction: --yes of the command never answers
	// for the loss of the changes of the project directory.
	if ok, err = confirmPrompt(i18n.Tf("cmd.session.delete.confirm", id), "", false); err != nil || !ok {
		return false, false, err
	}
	if plan.InBase && len(plan.ChangedFiles) > 0 && !discardBase {
		if discardBase, err = confirmPrompt(i18n.Tf("cmd.session.delete.confirm_discard", len(plan.ChangedFiles)), "", false); err != nil {
			return false, false, err
		}
	}
	return discardBase, true, nil
}

// printDeletePlan lists what a deletion removes and loses.
func printDeletePlan(w io.Writer, p *runsvc.DeletePlan, discardBase bool) {
	fmt.Fprintln(w, i18n.Tf("cmd.session.delete.plan_title", p.SessionID))
	for _, l := range deletePlanLines(p, discardBase, theme.WarningStyle.Render(theme.IconWarning), "cmd.session.delete.base_changes") {
		fmt.Fprintln(w, l)
	}
}

// deletePlanLines are the lines of a deletion plan: removed (→), lost
// (warn), kept. baseKey describes the kept changes of the project directory.
func deletePlanLines(p *runsvc.DeletePlan, discardBase bool, warn, baseKey string) []string {
	var out []string
	item := func(s string) { out = append(out, fmt.Sprintf("  %s %s", theme.IconArrow, s)) }
	lost := func(s string) { out = append(out, fmt.Sprintf("  %s %s", warn, s)) }
	list := func(items []string) {
		for i, it := range items {
			if i == 10 {
				out = append(out, fmt.Sprintf("      … (+%d)", len(items)-10))
				return
			}
			out = append(out, "      "+it)
		}
	}
	switch {
	case p.Worktree != "" && len(p.SharedWith) > 0:
		item(i18n.Tf("cmd.session.delete.worktree_shared", p.Worktree, strings.Join(p.SharedWith, ", ")))
	case p.Worktree != "":
		item(i18n.Tf("cmd.session.delete.worktree", p.Worktree))
		if p.Branch != "" {
			item(i18n.Tf("cmd.session.delete.branch", p.Branch))
		}
		if len(p.UnpushedCommits) > 0 {
			lost(i18n.Tf("cmd.session.delete.unpushed", len(p.UnpushedCommits)))
			list(p.UnpushedCommits)
		}
		if len(p.ChangedFiles) > 0 {
			lost(i18n.Tf("cmd.session.delete.changed", len(p.ChangedFiles)))
			list(p.ChangedFiles)
		}
	case p.InBase && len(p.ChangedFiles) > 0 && discardBase:
		lost(i18n.Tf("cmd.session.delete.base_discard", len(p.ChangedFiles)))
		list(p.ChangedFiles)
	case p.InBase && len(p.ChangedFiles) > 0:
		item(i18n.Tf(baseKey, len(p.ChangedFiles)))
	}
	if p.RemoteBranch {
		item(i18n.Tf("cmd.session.delete.remote_branch", p.Branch))
	}
	if p.OpenDecisions > 0 {
		item(i18n.Tf("cmd.session.delete.decisions", p.OpenDecisions))
	}
	if p.SessionDir != "" {
		item(i18n.Tf("cmd.session.delete.session_dir", p.SessionDir))
	}
	if p.GroupData != "" {
		item(i18n.Tf("cmd.session.delete.group_data", p.GroupData))
	}
	return out
}

// deleteSummary is the TUI confirmation of a deletion plan.
func deleteSummary(p *runsvc.DeletePlan) views.DeleteSummary {
	s := views.DeleteSummary{SessionID: p.SessionID, Items: deletePlanLines(p, false, "⚠", "tui.delete.base_changes"), Losses: p.Losses(false)}
	if p.InBase {
		s.BaseChanges = len(p.ChangedFiles)
	}
	return s
}

// sessionProjectPath is the project directory of a session ("" = unknown).
func sessionProjectPath(ctx context.Context, a *app.App, svc *runsvc.Service, id string) string {
	if sess, err := svc.Session(ctx, id); err == nil && a.Projects != nil {
		if proj, err := a.Projects.Get(ctx, sess.ProjectID); err == nil {
			return proj.Path
		}
	}
	return ""
}

// deleteReportText summarizes a deletion.
func deleteReportText(d *runsvc.DeleteReport) string { return launch.DeleteReportText(d) }

// resolveExistingChoice settles the existing sessions of a CLI launch: with
// a terminal and no --on-existing, the comparison is shown and a choice
// asked (the plan is made again with it); without a terminal, the same
// workflow needs --on-existing (another one: parallel, with a warning).
// It returns false when the user cancelled.
func resolveExistingChoice(ctx context.Context, out io.Writer, p *preparedRun, interactive bool) (bool, error) {
	if len(p.Plan.Existing) == 0 {
		return true, nil
	}
	if p.Opts.OnExisting != runsvc.OnExistingUnset {
		if p.Opts.OnExisting == runsvc.OnExistingDelete && interactive {
			e, found := existingTarget(p.Plan)
			if !found {
				return true, nil
			}
			discard, ok, err := confirmDelete(ctx, out, p.Svc, e.ID, p.Project.Path, p.Opts.DiscardBaseChanges)
			if err != nil || !ok {
				return false, err
			}
			if discard != p.Opts.DiscardBaseChanges {
				return true, launch.Replan(ctx, p, runsvc.OnExistingDelete, discard)
			}
		}
		return true, nil
	}
	if !interactive {
		if p.Plan.NeedsChoice {
			return false, errors.New(launch.ExistingChoiceMessage(p.Plan.Existing))
		}
		return true, nil
	}
	printExistingTable(out, p.Plan)
	choice, discard, ok, err := askExistingChoice(ctx, out, p)
	if err != nil || !ok {
		return false, err
	}
	return true, launch.Replan(ctx, p, choice, discard)
}

// askTransferFallback offers to keep the work in a new session when a
// transfer failed (the session is unchanged).
func askTransferFallback(out io.Writer, te *runsvc.TransferError) (bool, error) {
	fmt.Fprintf(out, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.run.existing.transfer_failed", te.SessionID, te.Err.Error()))
	return confirmPrompt(i18n.T("cmd.run.existing.transfer_fallback"), "", true)
}
