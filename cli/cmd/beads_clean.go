package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Beads « zero impact »: what `bd init` wrote in a project outside .beads/ is
// taken back out when oh initializes Beads or registers a project, after the
// user saw the changes (N17, DC16), and on demand by `oh doctor --fix`
// (committed content too). The commit of `bd init` is undone only with an
// explicit --yes. [beads] zero_impact = ask (default) | auto | off.

// Values of [beads] zero_impact.
const (
	zeroImpactAsk  = "ask"
	zeroImpactAuto = "auto"
	zeroImpactOff  = "off"
)

func zeroImpactMode(a *app.App) string {
	if a != nil && a.Config != nil {
		switch a.Config.Beads.ZeroImpact {
		case zeroImpactAuto, zeroImpactOff:
			return a.Config.Beads.ZeroImpact
		}
	}
	return zeroImpactAsk
}

// beadsCleanPlan is the clean-up proposed for a project (nil: nothing to
// do, or zero impact off).
func beadsCleanPlan(a *app.App, path string, yes bool) (*beads.CleanReport, beads.CleanOptions) {
	mode := zeroImpactMode(a)
	opts := beads.CleanOptions{UndoCommit: yes, KeepUntracked: mode == zeroImpactAuto && !yes}
	if path == "" || mode == zeroImpactOff || !beads.IsInitialized(path) {
		return nil, opts
	}
	dry := opts
	dry.DryRun = true
	r, err := beads.CleanImpact(path, dry)
	if err != nil {
		slog.Warn("beads zero impact: planning failed", "path", path, "error", err)
		return nil, opts
	}
	if len(r.Actions) == 0 {
		return nil, opts
	}
	return &r, opts
}

// applyBeadsClean runs the clean-up of a plan.
func applyBeadsClean(path string, opts beads.CleanOptions) beads.CleanReport {
	r, err := beads.CleanImpact(path, opts)
	if err != nil {
		slog.Warn("beads zero impact: clean-up failed", "path", path, "error", err)
	}
	return r
}

// printBeadsClean offers the clean-up of a registered project (CLI): the
// changes are listed, then applied after a confirmation (auto: applied;
// without a terminal: listed only). yes applies without asking and undoes
// the commit of bd init when it was not pushed.
func printBeadsClean(w io.Writer, a *app.App, path string, yes bool) {
	plan, opts := beadsCleanPlan(a, path, yes)
	if plan == nil {
		return
	}
	if !plan.Changed() {
		printCleanKept(w, *plan)
		return
	}
	if !yes && zeroImpactMode(a) == zeroImpactAsk {
		fmt.Fprintln(w, theme.Bold.Render(i18n.T("cmd.beads.clean.plan")))
		printCleanActions(w, *plan)
		if !stdinIsTerminal() {
			fmt.Fprintf(w, "  %s\n", i18n.T("cmd.beads.clean.report_only"))
			return
		}
		ok, err := cliConfirm(w, i18n.T("cmd.beads.clean.confirm"))
		if err != nil || !ok {
			fmt.Fprintf(w, "  %s\n", i18n.T("cmd.beads.clean.declined"))
			return
		}
	}
	r := applyBeadsClean(path, opts)
	if r.Changed() {
		fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.beads.clean.auto_done"))
	}
	printCleanActions(w, r)
	printCleanHints(w, r)
}

// printCleanKept prints what is left in place (nothing to apply).
func printCleanKept(w io.Writer, r beads.CleanReport) {
	printCleanActions(w, r)
	printCleanHints(w, r)
}

func printCleanHints(w io.Writer, r beads.CleanReport) {
	kinds := map[string]bool{}
	for _, a := range r.Actions {
		kinds[a.Kind] = true
	}
	if kinds[beads.CleanSkipped] {
		fmt.Fprintf(w, "  %s\n", i18n.T("cmd.beads.clean.skipped_hint"))
	}
	if kinds[beads.CleanCommitKept] {
		fmt.Fprintf(w, "  %s\n", i18n.T("cmd.beads.clean.commit_hint"))
	}
	if kinds[beads.CleanAsk] {
		fmt.Fprintf(w, "  %s\n", i18n.T("cmd.beads.clean.ask_hint"))
	}
}

// offerBeadsClean is the clean-up after a project is registered or Beads
// initialized from the TUI (a confirmation window) or a wizard without it.
func offerBeadsClean(a *app.App, path string) {
	sh := tuiShell
	if sh == nil {
		printBeadsClean(os.Stderr, a, path, false)
		return
	}
	go func() {
		plan, opts := beadsCleanPlan(a, path, false)
		if plan == nil || !plan.Changed() {
			return
		}
		var body strings.Builder
		printCleanActions(&body, *plan)
		apply := func() {
			go func() {
				r := applyBeadsClean(path, opts)
				var hints strings.Builder
				printCleanHints(&hints, r)
				sh.App().QueueUpdateDraw(func() {
					msg := i18n.T("cmd.beads.clean.auto_done")
					if h := strings.TrimSpace(hints.String()); h != "" {
						msg += " — " + h
					}
					sh.ShowToastMsg(msg, true)
				})
			}()
		}
		sh.App().QueueUpdateDraw(func() {
			if zeroImpactMode(a) == zeroImpactAuto {
				apply()
				return
			}
			sh.ShowScrollableModal(i18n.T("cmd.beads.clean.plan"), body.String()+"\n"+i18n.T("cmd.beads.clean.tui_hint"), []views.ModalAction{
				{Label: i18n.T("cmd.beads.clean.apply"), Callback: apply},
				{Label: i18n.T("cmd.beads.clean.later"), Callback: func() {}},
			})
		})
	}()
}

func printCleanActions(w io.Writer, r beads.CleanReport) {
	for _, a := range r.Actions {
		fmt.Fprintf(w, "    %s\n", i18n.Tf("cmd.beads.clean.kind."+a.Kind, filepath.ToSlash(a.Path)))
	}
}

// runDoctorFix plans the clean-up of every Beads project of the hub, asks
// for a confirmation (--yes without terminal) and applies it.
func runDoctorFix(cmd *cobra.Command, a *app.App) error {
	w := cmd.OutOrStdout()
	yesFlag, _ := cmd.Flags().GetBool("yes")
	plans, err := beadsFixPlans(cmd.Context(), a, yesFlag)
	if err != nil {
		return err
	}
	if len(plans) == 0 {
		fmt.Fprintln(w, i18n.T("cmd.doctor.fix.nothing"))
		fmt.Fprintln(w)
		return nil
	}
	fmt.Fprintln(w, theme.Bold.Render(i18n.T("cmd.doctor.fix.title")))
	for _, p := range plans {
		fmt.Fprintf(w, "  %s (%s)\n", p.project.Name, p.project.Path)
		printCleanActions(w, p.report)
	}
	yes, _ := cmd.Flags().GetBool("yes")
	if !yes {
		printCleanHints(w, beads.CleanReport{Actions: commitKept(plans)})
	}
	if !yes {
		if !stdinIsTerminal() {
			return errors.New(i18n.T("cmd.doctor.fix.needs_yes"))
		}
		ok, err := launcher.NewCLIUI(cmd.ErrOrStderr()).Confirm(i18n.T("cmd.doctor.fix.confirm"))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(w, i18n.T("cmd.doctor.fix.cancelled"))
			return nil
		}
	}
	for _, p := range plans {
		if _, err := beads.FixImpact(p.project.Path, false, yes); err != nil {
			return fmt.Errorf("%s: %w", p.project.Name, err)
		}
	}
	fmt.Fprintf(w, "%s %s\n\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.doctor.fix.done", len(plans)))
	return nil
}

type beadsFixPlan struct {
	project domain.Project
	report  beads.CleanReport
}

func beadsFixPlans(ctx context.Context, a *app.App, undoCommit bool) ([]beadsFixPlan, error) {
	if a.Projects == nil {
		return nil, nil
	}
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil, err
	}
	var out []beadsFixPlan
	for _, p := range projects {
		if p.Path == "" || !beads.IsInitialized(p.Path) {
			continue
		}
		r, err := beads.FixImpact(p.Path, true, undoCommit)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name, err)
		}
		if r.Changed() {
			out = append(out, beadsFixPlan{project: p, report: r})
		}
	}
	return out, nil
}

func commitKept(plans []beadsFixPlan) []beads.CleanAction {
	var out []beads.CleanAction
	for _, p := range plans {
		for _, a := range p.report.Actions {
			if a.Kind == beads.CleanCommitKept {
				out = append(out, a)
			}
		}
	}
	return out
}
