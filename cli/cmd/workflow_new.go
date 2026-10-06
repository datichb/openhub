package cmd

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
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
	f.Bool("no-edit", false, i18n.T("cmd.workflow.new.flags.no_edit"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runWorkflowNew(cmd *cobra.Command, args []string) error {
	id := args[0]
	if err := teamstate.ValidWorkflowID(id); err != nil {
		return errors.New(i18n.Tf("cmd.workflow.new.bad_id", id))
	}
	layer, err := layerFlag(cmd)
	if err != nil {
		return err
	}
	extends, _ := cmd.Flags().GetString("extends")
	copyFrom, _ := cmd.Flags().GetString("copy")
	file, _ := cmd.Flags().GetString("file")
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

	if t, err := svc.EditText(ctxOf(cmd), c, layer, id); err == nil {
		return errors.New(i18n.Tf("cmd.workflow.new.exists", t.Ref.String()))
	} else if !errors.Is(err, workflowsvc.ErrNotFound) {
		return workflowEditError(errOut, err)
	}

	e := draftEdit{Layer: layer, Name: id + ".yaml", EditYAML: !noEdit && file == ""}
	switch {
	case file != "":
		if e.YAML, err = readWorkflowFile(file); err != nil {
			return err
		}
	case copyFrom != "":
		src, err := svc.DocumentText(ctxOf(cmd), c, copyFrom)
		if err != nil {
			return workflowEditError(errOut, err)
		}
		e.YAML = copyDocument(src.YAML, id)
		e.Prompt = src.Prompt
	case extends != "":
		if _, err := workflow.ParseRef(extends); err != nil {
			return errors.New(i18n.Tf("cmd.workflow.new.bad_extends", extends))
		}
		if !svc.Has(ctxOf(cmd), c, extends) {
			return errors.New(i18n.Tf("cmd.workflow.show.unknown", extends))
		}
		e.YAML = []byte(fmt.Sprintf(i18n.T("cmd.workflow.new.template_extends"), id, extends))
	default:
		e.YAML = []byte(fmt.Sprintf(i18n.T("cmd.workflow.new.template_empty"), id))
	}
	d, err := saveDraftEdit(cmd, svc, c, e)
	if err != nil {
		return workflowEditError(errOut, err)
	}
	printDraftSaved(cmd.OutOrStdout(), d)
	return nil
}

var (
	reTopID      = regexp.MustCompile(`(?m)^id:[^\n]*$`)
	reTopVersion = regexp.MustCompile(`(?m)^version:[^\n]*\n?`)
)

// copyDocument renames a copied document and drops its published version.
func copyDocument(data []byte, id string) []byte {
	out := reTopID.ReplaceAll(data, []byte("id: "+id))
	return reTopVersion.ReplaceAll(out, nil)
}
