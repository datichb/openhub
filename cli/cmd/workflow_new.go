package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func init() {
	workflowCmd.AddCommand(workflowNewCmd())
}

func workflowNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <id>",
		Short: i18n.T("cmd.workflow.new.short"),
		Long:  i18n.T("cmd.workflow.new.long"),
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowNew,
	}
	f := cmd.Flags()
	f.String("layer", string(workflow.LayerTeam), i18n.T("cmd.workflow.new.flags.layer"))
	f.String("extends", "", i18n.T("cmd.workflow.new.flags.extends"))
	f.String("copy", "", i18n.T("cmd.workflow.new.flags.copy"))
	f.String("file", "", i18n.T("cmd.workflow.new.flags.file"))
	f.String("prompt-file", "", i18n.T("cmd.workflow.new.flags.prompt_file"))
	f.Bool("no-edit", false, i18n.T("cmd.workflow.new.flags.no_edit"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runWorkflowNew(cmd *cobra.Command, args []string) error {
	id := args[0]
	layer, err := layerFlag(cmd)
	if err != nil {
		return err
	}
	extends, _ := cmd.Flags().GetString("extends")
	copyFrom, _ := cmd.Flags().GetString("copy")
	file, _ := cmd.Flags().GetString("file")
	promptFile, _ := cmd.Flags().GetString("prompt-file")
	noEdit, _ := cmd.Flags().GetBool("no-edit")
	if extends != "" && copyFrom != "" {
		return errors.New(i18n.T("cmd.workflow.new.extends_and_copy"))
	}
	c, err := workflowCmdContext(cmd)
	if err != nil {
		return err
	}
	svc := newWorkflowService(ctxOf(cmd))
	errOut := cmd.ErrOrStderr()

	n := workflowsvc.NewDraft{ID: id, Layer: layer, Extends: extends, Copy: copyFrom}
	if file != "" {
		n.Extends, n.Copy = "", ""
	}
	t, err := svc.NewDraftText(ctxOf(cmd), c, n)
	if err != nil {
		return workflowEditError(errOut, err)
	}
	e := draftEdit{Layer: layer, Name: id + ".yaml", EditYAML: !noEdit && file == "", YAML: t.YAML, Prompt: t.Prompt}
	if file != "" {
		if e.YAML, err = readWorkflowFile(file); err != nil {
			return err
		}
		// A document naming its own template: the given one, else a starter
		// (before: refused, the template did not exist yet).
		e.Prompt = workflowsvc.StarterPrompt(e.YAML)
		if e.Prompt != nil && promptFile == "" {
			fmt.Fprintln(errOut, i18n.T("cmd.workflow.new.starter_prompt"))
		}
	}
	if promptFile != "" {
		if e.Prompt, err = readWorkflowFile(promptFile); err != nil {
			return err
		}
	}
	d, err := saveDraftEdit(cmd, svc, c, e)
	if err != nil {
		return workflowEditError(errOut, err)
	}
	printDraftSaved(cmd.OutOrStdout(), d)
	return nil
}
