package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func init() {
	workflowCmd.AddCommand(workflowPublishCmd())
}

func workflowPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish <id>|<layer>:<id> -m <message>",
		Short: i18n.T("cmd.workflow.publish.short"),
		Long:  i18n.T("cmd.workflow.publish.long"),
		Args:  cobra.RangeArgs(0, 1),
		RunE:  runWorkflowPublish,
	}
	cmd.Flags().StringP("message", "m", "", i18n.T("cmd.workflow.publish.flags.message"))
	cmd.Flags().Bool("retry", false, i18n.T("cmd.workflow.publish.flags.retry"))
	cmd.Flags().BoolP("yes", "y", false, i18n.T("cmd.workflow.publish.flags.yes"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runWorkflowPublish(cmd *cobra.Command, args []string) error {
	c, err := workflowCmdContext(cmd)
	if err != nil {
		return err
	}
	svc := newWorkflowService(ctxOf(cmd))
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()

	if retry, _ := cmd.Flags().GetBool("retry"); retry {
		return flushWorkflowQueue(cmd, svc, c)
	}
	if len(args) != 1 {
		return errors.New(i18n.T("cmd.workflow.publish.usage"))
	}
	message, _ := cmd.Flags().GetString("message")
	if strings.TrimSpace(message) == "" {
		return errors.New(i18n.T("cmd.workflow.publish.message_required"))
	}

	preview, err := svc.Preview(ctxOf(cmd), c, args[0])
	if err != nil {
		return workflowEditError(errOut, err)
	}
	fmt.Fprintln(out, i18n.Tf("cmd.workflow.publish.preview", preview.Ref.String(), preview.NextVersion))
	printDiagnostics(out, preview.Diagnostics)
	printImpact(out, preview.Impact)
	if len(preview.Impact.Widenings()) > 0 {
		ok, err := confirm(cmd, i18n.Tf("cmd.workflow.publish.confirm_widen", len(preview.Impact.Widenings())))
		if err != nil || !ok {
			return err
		}
	}
	p, err := svc.Publish(ctxOf(cmd), c, preview.Ref.String(), message)
	if err != nil {
		return workflowEditError(errOut, err)
	}
	if p.Queued {
		printPublication(out, p)
		return nil
	}
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("teamstate.workflow.cli.published", p.Ref.String(), p.Version))
	printDiagnostics(out, p.Diagnostics)
	return nil
}

// flushWorkflowQueue replays the operations queued while offline.
func flushWorkflowQueue(cmd *cobra.Command, svc *workflowsvc.Service, c workflowsvc.Context) error {
	out := cmd.OutOrStdout()
	pubs, errs := svc.FlushQueue(ctxOf(cmd), c)
	for _, p := range pubs {
		printPublication(out, p)
	}
	for _, e := range errs {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s %s\n", theme.ErrorStyle.Render(theme.IconError), workflowEditError(cmd.ErrOrStderr(), e))
	}
	if len(pubs) == 0 && len(errs) == 0 {
		fmt.Fprintln(out, i18n.T("cmd.workflow.publish.queue_empty"))
	}
	if len(errs) > 0 {
		return errors.New(i18n.Tf("cmd.workflow.publish.queue_failed", len(errs)))
	}
	return nil
}
