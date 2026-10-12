package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func init() {
	cmd := &cobra.Command{
		Use:   "delete <session-id>",
		Short: i18n.T("cmd.session.delete.short"),
		Long:  i18n.T("cmd.session.delete.long"),
		Args:  cobra.ExactArgs(1),
		RunE:  runSessionDelete,
	}
	cmd.Flags().BoolP("yes", "y", false, i18n.T("cmd.session.delete.flags.yes"))
	cmd.Flags().Bool("discard-base-changes", false, i18n.T("cmd.run.flags.discard_base"))
	sessionCmd.AddCommand(cmd)
}

func runSessionDelete(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	a := MustApp()
	out := cmd.OutOrStdout()
	id, err := resolveSessionRef(ctx, args[0])
	if err != nil {
		return err
	}
	svc, err := newRunService(ctx, a)
	if err != nil {
		return err
	}
	projectPath := sessionProjectPath(ctx, a, svc, id)
	yes, _ := cmd.Flags().GetBool("yes")
	discard, _ := cmd.Flags().GetBool("discard-base-changes")
	if !yes {
		if !isTerminal() {
			plan, err := svc.PlanDelete(ctx, id, runsvc.DeleteOptions{ProjectPath: projectPath, DiscardBaseChanges: discard})
			if err != nil {
				return err
			}
			printDeletePlan(out, plan, discard)
			return errors.New(i18n.T("cmd.session.delete.needs_yes"))
		}
		var ok bool
		if discard, ok, err = confirmDelete(ctx, out, svc, id, projectPath, discard); err != nil || !ok {
			if err == nil {
				fmt.Fprintln(out, i18n.T("cmd.run.cancelled"))
			}
			return err
		}
	}
	rep, err := svc.DeleteSession(ctx, id, runsvc.DeleteOptions{ProjectPath: projectPath, DiscardBaseChanges: discard})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), deleteReportText(rep))
	return nil
}
