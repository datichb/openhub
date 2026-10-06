package cmd

import (
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
)

func init() {
	workflowCmd.AddCommand(workflowEditCmd())
}

func workflowEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <id>|<layer>:<id>",
		Short: i18n.T("cmd.workflow.edit.short"),
		Long:  i18n.T("cmd.workflow.edit.long"),
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowEdit,
	}
	f := cmd.Flags()
	f.String("layer", "", i18n.T("cmd.workflow.edit.flags.layer"))
	f.Bool("prompt", false, i18n.T("cmd.workflow.edit.flags.prompt"))
	f.String("file", "", i18n.T("cmd.workflow.edit.flags.file"))
	f.String("prompt-file", "", i18n.T("cmd.workflow.edit.flags.prompt_file"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runWorkflowEdit(cmd *cobra.Command, args []string) error {
	c, err := workflowCmdContext(cmd)
	if err != nil {
		return err
	}
	svc := newWorkflowService(ctxOf(cmd))
	errOut := cmd.ErrOrStderr()
	t, layer, err := editTarget(cmd, svc, c, args[0])
	if err != nil {
		return workflowEditError(errOut, err)
	}
	file, _ := cmd.Flags().GetString("file")
	promptFile, _ := cmd.Flags().GetString("prompt-file")
	editPrompt, _ := cmd.Flags().GetBool("prompt")
	e := draftEdit{Layer: layer, Name: t.Ref.ID + ".yaml", YAML: t.YAML}
	switch {
	case file != "" || promptFile != "":
		if file != "" {
			if e.YAML, err = readWorkflowFile(file); err != nil {
				return err
			}
		}
		if promptFile != "" {
			if e.Prompt, err = readWorkflowFile(promptFile); err != nil {
				return err
			}
		}
	case editPrompt:
		e.EditPrompt = true
		e.Prompt = t.Prompt
	default:
		e.EditYAML = true
	}
	d, err := saveDraftEdit(cmd, svc, c, e)
	if err != nil {
		return workflowEditError(errOut, err)
	}
	printDraftSaved(cmd.OutOrStdout(), d)
	return nil
}
