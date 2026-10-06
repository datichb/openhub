package cmd

import (
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
)

func init() {
	workflowCmd.AddCommand(workflowArchiveCmd())
}

func workflowArchiveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <id>|<layer>:<id>",
		Short: i18n.T("cmd.workflow.archive.short"),
		Long:  i18n.T("cmd.workflow.archive.long"),
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowArchive,
	}
	cmd.Flags().StringP("message", "m", "", i18n.T("cmd.workflow.archive.flags.message"))
	cmd.Flags().BoolP("yes", "y", false, i18n.T("cmd.workflow.publish.flags.yes"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runWorkflowArchive(cmd *cobra.Command, args []string) error {
	c, err := workflowCmdContext(cmd)
	if err != nil {
		return err
	}
	if ok, err := confirm(cmd, i18n.Tf("cmd.workflow.archive.confirm", args[0])); err != nil || !ok {
		return err
	}
	message, _ := cmd.Flags().GetString("message")
	p, err := newWorkflowService(ctxOf(cmd)).Archive(ctxOf(cmd), c, args[0], message)
	if err != nil {
		return workflowEditError(cmd.ErrOrStderr(), err)
	}
	printPublication(cmd.OutOrStdout(), p)
	return nil
}
