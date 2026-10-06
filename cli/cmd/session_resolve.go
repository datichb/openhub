package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	remotesvc "github.com/datichb/openhub/cli/internal/services/remote"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func init() {
	cmd := &cobra.Command{
		Use:   "resolve <session-id>",
		Short: i18n.T("cmd.session.resolve.short"),
		Long:  i18n.T("cmd.session.resolve.long"),
		Args:  cobra.ExactArgs(1),
		RunE:  runSessionResolve,
	}
	f := cmd.Flags()
	f.StringSlice("keep-local", nil, i18n.T("cmd.session.resolve.flags.keep_local"))
	f.StringSlice("apply-remote", nil, i18n.T("cmd.session.resolve.flags.apply_remote"))
	f.StringSlice("merge-notes", nil, i18n.T("cmd.session.resolve.flags.merge_notes"))
	f.String("all", "", i18n.T("cmd.session.resolve.flags.all"))
	f.BoolP("yes", "y", false, i18n.T("cmd.session.resolve.flags.yes"))
	f.Bool("dry-run", false, i18n.T("cmd.session.resolve.flags.dry_run"))
	sessionCmd.AddCommand(cmd)
}

// printReplayPlan shows what the replay will do.
func printReplayPlan(w io.Writer, p *remotesvc.ReplayPlan) {
	for _, it := range p.Items {
		icon := theme.IconPending
		switch it.State {
		case remotesvc.ItemApplied:
			icon = theme.SuccessStyle.Render(theme.IconSuccess)
		case remotesvc.ItemRefused, remotesvc.ItemFailed:
			icon = theme.ErrorStyle.Render(theme.IconError)
		case remotesvc.ItemSkipped:
			icon = theme.IconSkipped
		}
		line := fmt.Sprintf("  %s #%d bd %s — %s", icon, it.Seq, strings.Join(it.Argv, " "), i18n.T("cmd.session.resolve.state."+it.State))
		if it.Reason != "" {
			line += " (" + it.Reason + ")"
		}
		if it.Created != "" {
			line += " → " + it.Created
		}
		fmt.Fprintln(w, line)
	}
	for _, c := range p.Conflicts {
		fmt.Fprintf(w, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.session.resolve.conflict", c.Ticket, c.Title))
		for _, f := range c.Fields {
			fmt.Fprintf(w, "    %s : %s\n", f.Field, i18n.Tf("cmd.session.resolve.field", f.Local, f.Remote, f.Snapshot))
		}
		for _, n := range c.Notes {
			fmt.Fprintf(w, "    %s %s\n", i18n.T("cmd.session.resolve.note"), n)
		}
		if c.Resolution != "" {
			fmt.Fprintf(w, "    → %s\n", i18n.T("cmd.session.resolve.choice."+string(c.Resolution)))
		}
	}
}

func runSessionResolve(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	a := MustApp()
	out := cmd.OutOrStdout()
	id, err := resolveSessionRef(ctx, args[0])
	if err != nil {
		return err
	}
	svc := newRemoteReturnService(ctx, a)
	plan, err := svc.PlanReplay(ctx, id)
	if errors.Is(err, remotesvc.ErrNotRemote) {
		return errors.New(i18n.T("cmd.session.fetch.not_remote"))
	}
	if err != nil {
		return err
	}
	f := cmd.Flags()
	res := map[string]remotesvc.Resolution{}
	if all, _ := f.GetString("all"); all != "" {
		r := remotesvc.Resolution(strings.ReplaceAll(all, "-", "_"))
		if !r.Valid() {
			return errors.New(i18n.Tf("cmd.session.resolve.bad_choice", all))
		}
		for _, c := range plan.Conflicts {
			res[c.Ticket] = r
		}
	}
	for flag, r := range map[string]remotesvc.Resolution{"keep-local": remotesvc.KeepLocal, "apply-remote": remotesvc.ApplyRemote, "merge-notes": remotesvc.MergeNotes} {
		ts, _ := f.GetStringSlice(flag)
		for _, t := range ts {
			res[strings.TrimSpace(t)] = r
		}
	}
	for i := range plan.Conflicts {
		if r, ok := res[plan.Conflicts[i].Ticket]; ok {
			plan.Conflicts[i].Resolution = r
		}
	}
	printReplayPlan(out, plan)
	if plan.Pending() == 0 {
		fmt.Fprintln(out, i18n.T("cmd.session.resolve.nothing"))
		_, err := svc.ApplyReplay(ctx, id, res) // marks the session resolved
		return err
	}
	if dry, _ := f.GetBool("dry-run"); dry {
		return nil
	}
	yes, _ := f.GetBool("yes")
	tty := interactive()
	// Conflicts without a choice: ask (terminal), else refuse.
	for i, c := range plan.Conflicts {
		if c.Resolution.Valid() {
			continue
		}
		if !tty {
			return errors.New(i18n.Tf("cmd.session.resolve.unresolved", strings.Join(plan.Unresolved(), ", ")))
		}
		choice := string(remotesvc.KeepLocal)
		form := theme.NewForm(huh.NewGroup(huh.NewSelect[string]().Title(i18n.Tf("cmd.session.resolve.ask", c.Ticket)).
			Options(huh.NewOption(i18n.T("cmd.session.resolve.choice.keep_local"), string(remotesvc.KeepLocal)),
				huh.NewOption(i18n.T("cmd.session.resolve.choice.apply_remote"), string(remotesvc.ApplyRemote)),
				huh.NewOption(i18n.T("cmd.session.resolve.choice.merge_notes"), string(remotesvc.MergeNotes))).Value(&choice)))
		if err := form.Run(); err != nil {
			return err
		}
		res[c.Ticket] = remotesvc.Resolution(choice)
		plan.Conflicts[i].Resolution = remotesvc.Resolution(choice)
	}
	if !yes {
		if !tty {
			return errors.New(i18n.T("cmd.session.resolve.confirm_needed"))
		}
		ok := false
		form := theme.NewForm(huh.NewGroup(huh.NewConfirm().Title(i18n.Tf("cmd.session.resolve.confirm", plan.Pending())).Value(&ok)))
		if err := form.Run(); err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, i18n.T("cmd.run.cancelled"))
			return nil
		}
	}
	plan, err = svc.ApplyReplay(ctx, id, res)
	if err != nil {
		return err
	}
	fmt.Fprintln(out)
	printReplayPlan(out, plan)
	failed := 0
	for _, it := range plan.Items {
		if it.State == remotesvc.ItemFailed {
			failed++
		}
	}
	if failed > 0 {
		return errors.New(i18n.Tf("cmd.session.resolve.failed", failed, id))
	}
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("cmd.session.resolve.done"))
	return nil
}
