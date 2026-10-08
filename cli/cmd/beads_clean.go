package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Beads « zero impact »: what `bd init` wrote in a project outside .beads/ is
// taken back out automatically when oh initializes Beads or registers a
// project (bd's content never committed, or its own commit not pushed), and
// on demand by `oh doctor --fix` (committed content too, after a summary and
// a confirmation).

// autoCleanBeads cleans a registered project after a `bd init` (nothing when
// it has no .beads/).
func autoCleanBeads(path string) beads.CleanReport {
	if path == "" || !beads.IsInitialized(path) {
		return beads.CleanReport{}
	}
	r, err := beads.CleanImpact(path, beads.CleanOptions{})
	if err != nil {
		slog.Warn("beads zero impact: clean-up failed", "path", path, "error", err)
	}
	return r
}

// printBeadsClean cleans a registered project and prints what changed.
func printBeadsClean(w io.Writer, path string) {
	r := autoCleanBeads(path)
	if !r.Changed() && len(r.Skipped()) == 0 {
		return
	}
	if r.Changed() {
		fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.beads.clean.auto_done"))
	}
	printCleanActions(w, r)
	if len(r.Skipped()) > 0 {
		fmt.Fprintf(w, "  %s\n", i18n.T("cmd.beads.clean.skipped_hint"))
	}
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
	plans, err := beadsFixPlans(cmd.Context(), a)
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
		if _, err := beads.FixImpact(p.project.Path, false); err != nil {
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

func beadsFixPlans(ctx context.Context, a *app.App) ([]beadsFixPlan, error) {
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
		r, err := beads.FixImpact(p.Path, true)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.Name, err)
		}
		if r.Changed() {
			out = append(out, beadsFixPlan{project: p, report: r})
		}
	}
	return out, nil
}
