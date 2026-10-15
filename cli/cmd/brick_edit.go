package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
)

func brickNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <agent|skill> <id>",
		Short: i18n.T("cmd.brick.new.short"),
		Long:  i18n.T("cmd.brick.new.long"),
		Args:  cobra.ExactArgs(2),
		RunE:  runBrickNew,
	}
	f := cmd.Flags()
	f.String("family", "", i18n.T("cmd.brick.new.flags.family"))
	f.Bool("extends", false, i18n.T("cmd.brick.new.flags.extends"))
	f.String("file", "", i18n.T("cmd.brick.new.flags.file"))
	f.Bool("no-edit", false, i18n.T("cmd.brick.new.flags.no_edit"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runBrickNew(cmd *cobra.Command, args []string) error {
	kind := workflowsvc.BrickKind(args[0])
	if kind != workflowsvc.BrickAgent && kind != workflowsvc.BrickSkill {
		return errors.New(i18n.Tf("cmd.brick.new.bad_kind", args[0]))
	}
	family, _ := cmd.Flags().GetString("family")
	if family != "" && kind != workflowsvc.BrickAgent {
		return errors.New(i18n.T("cmd.brick.new.family_agent_only"))
	}
	extends, _ := cmd.Flags().GetBool("extends")
	file, _ := cmd.Flags().GetString("file")
	noEdit, _ := cmd.Flags().GetBool("no-edit")
	c, err := workflowCmdContext(cmd)
	if err != nil {
		return err
	}
	svc := newWorkflowService(ctxOf(cmd))
	errOut := cmd.ErrOrStderr()
	ref := workflowsvc.BrickRef{Kind: kind, ID: args[1]}
	t, err := svc.NewBrickText(ctxOf(cmd), c, workflowsvc.NewBrick{Ref: ref, Family: family, Extends: extends})
	if err != nil {
		return brickError(errOut, err)
	}
	in := workflowsvc.BrickInput{Ref: ref, Family: t.Family, Data: t.Data}
	if file != "" {
		if in.Data, err = readWorkflowFile(file); err != nil {
			return err
		}
	}
	d, err := saveBrickEdit(cmd, svc, c, in, !noEdit && file == "")
	if err != nil {
		return brickError(errOut, err)
	}
	printBrickDraft(cmd.OutOrStdout(), d)
	return nil
}

func brickEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <id>|agent:<id>|skill:<ref>",
		Short: i18n.T("cmd.brick.edit.short"),
		Long:  i18n.T("cmd.brick.edit.long"),
		Args:  cobra.ExactArgs(1),
		RunE:  runBrickEdit,
	}
	f := cmd.Flags()
	f.String("file", "", i18n.T("cmd.brick.edit.flags.file"))
	f.String("family", "", i18n.T("cmd.brick.new.flags.family"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runBrickEdit(cmd *cobra.Command, args []string) error {
	c, err := workflowCmdContext(cmd)
	if err != nil {
		return err
	}
	svc := newWorkflowService(ctxOf(cmd))
	errOut := cmd.ErrOrStderr()
	t, err := svc.EditableBrickText(ctxOf(cmd), c, args[0])
	if err != nil {
		return brickError(errOut, err)
	}
	in := workflowsvc.BrickInput{Ref: t.Ref, Family: t.Family, Data: t.Data}
	if family, _ := cmd.Flags().GetString("family"); family != "" {
		if t.Ref.Kind != workflowsvc.BrickAgent {
			return errors.New(i18n.T("cmd.brick.new.family_agent_only"))
		}
		in.Family = family
	}
	file, _ := cmd.Flags().GetString("file")
	if file != "" {
		if in.Data, err = readWorkflowFile(file); err != nil {
			return err
		}
	}
	d, err := saveBrickEdit(cmd, svc, c, in, file == "")
	if err != nil {
		return brickError(errOut, err)
	}
	printBrickDraft(cmd.OutOrStdout(), d)
	return nil
}
