package cmd

import (
	"errors"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
)

func init() {
	workflowCmd.AddCommand(workflowRestoreCmd())
}

func workflowRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <id>|<layer>:<id> <version>",
		Short: i18n.T("cmd.workflow.restore.short"),
		Long:  i18n.T("cmd.workflow.restore.long"),
		Args:  cobra.ExactArgs(2),
		RunE:  runWorkflowRestore,
	}
	cmd.Flags().BoolP("yes", "y", false, i18n.T("cmd.workflow.publish.flags.yes"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runWorkflowRestore(cmd *cobra.Command, args []string) error {
	version, err := strconv.Atoi(args[1])
	if err != nil || version <= 0 {
		return errors.New(i18n.Tf("cmd.workflow.restore.bad_version", args[1]))
	}
	c, err := workflowCmdContext(cmd)
	if err != nil {
		return err
	}
	if ok, err := confirm(cmd, i18n.Tf("cmd.workflow.restore.confirm", args[0], version)); err != nil || !ok {
		return err
	}
	p, err := newWorkflowService(ctxOf(cmd)).Restore(ctxOf(cmd), c, args[0], version)
	if err != nil {
		return workflowEditError(cmd.ErrOrStderr(), err)
	}
	printPublication(cmd.OutOrStdout(), p)
	return nil
}
