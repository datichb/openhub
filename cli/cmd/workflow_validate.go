package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func workflowValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [<file>|<id>|<layer>:<id>]",
		Short: "Valide un workflow (fichier ou identifiant du catalogue)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			layer, _ := cmd.Flags().GetString("layer")
			asJSON, _ := cmd.Flags().GetBool("json")
			all, _ := cmd.Flags().GetBool("all")
			if (len(args) == 0) == !all {
				return errors.New(i18n.T("cmd.workflow.validate.usage"))
			}
			target := ""
			if len(args) == 1 {
				target = args[0]
			}
			projectRef, _ := cmd.Flags().GetString("project")
			layers, err := resolveWorkflowTeamLayers(cmd.Context(), TryApp(), projectRef)
			if err != nil {
				return err
			}
			report, err := runWorkflowValidate(findHubDir(), target, workflow.Layer(layer), all, layers)
			if err != nil {
				return err
			}
			return printWorkflowValidate(cmd.OutOrStdout(), report, asJSON)
		},
	}
	cmd.Flags().String("layer", string(workflow.LayerHub), "Couche du fichier validé (hub, team, project)")
	cmd.Flags().Bool("json", false, "Sortie JSON")
	cmd.Flags().Bool("all", false, "Valide tous les workflows du hub")
	cmd.Flags().String("project", "", i18n.T("teamstate.workflow.flag_project"))
	return cmd
}

// workflowValidateReport is the result of `oh workflow validate`.
type workflowValidateReport = workflowsvc.ValidateReport

// runWorkflowValidate validates a file, a catalogue id or (all) every
// workflow against the hub content in hubDir and, when layers is set, the
// published team and project workflows (integrity-checked) and the brick
// catalogue merged with the team bricks.
func runWorkflowValidate(hubDir, target string, layer workflow.Layer, all bool, layers *workflowTeamLayers) (*workflowValidateReport, error) {
	svc := &workflowsvc.Service{HubDir: hubDir}
	if layers != nil {
		ts := &workflowsvc.TeamState{Repo: layers.Repo, TeamID: layers.TeamID, Member: layers.Member, Project: layers.Project}
		svc.TeamState = func(context.Context, workflowsvc.Context) (*workflowsvc.TeamState, error) { return ts, nil }
	}
	in := workflowsvc.ValidateInput{Layer: layer, All: all}
	switch {
	case all:
	case workflowsvc.IsWorkflowFile(target):
		in.File = target
	default:
		in.Target = target
	}
	return svc.Validate(context.Background(), workflowsvc.Context{}, in)
}

func printWorkflowValidate(w io.Writer, r *workflowValidateReport, asJSON bool) error {
	errs, warns := r.Counts()
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			return err
		}
	} else {
		for _, d := range r.Diagnostics {
			icon, style := theme.IconError, theme.ErrorStyle
			if d.Severity == workflow.SeverityWarning {
				icon, style = theme.IconWarning, theme.WarningStyle
			}
			loc := d.Source
			if p := d.Pos.String(); p != "" {
				loc += ":" + p
			}
			head := loc
			if d.Path != "" {
				head += "  " + d.Path
			}
			fmt.Fprintf(w, "%s %s\n    %s\n", style.Render(icon), head, d.Message)
			if d.Hint != "" {
				fmt.Fprintf(w, "    %s\n", theme.Subtitle.Render(d.Hint))
			}
		}
		for _, wf := range r.Workflows {
			if wf.Valid {
				fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.workflow.validate.valid", wf.Ref))
			} else {
				fmt.Fprintf(w, "%s %s\n", theme.ErrorStyle.Render(theme.IconError), i18n.Tf("cmd.workflow.validate.invalid", wf.Ref))
			}
		}
		if len(r.Workflows) == 0 {
			fmt.Fprintln(w, i18n.T("cmd.workflow.validate.none"))
		}
		if warns > 0 && errs == 0 {
			fmt.Fprintln(w, i18n.Tf("cmd.workflow.validate.warnings", warns))
		}
	}
	if errs > 0 {
		return errors.New(i18n.Tf("cmd.workflow.validate.failed", errs, warns))
	}
	return nil
}
