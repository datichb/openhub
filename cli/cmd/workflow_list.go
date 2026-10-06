package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func init() {
	workflowCmd.AddCommand(workflowListCmd())
}

func workflowListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: i18n.T("cmd.workflow.list.short"),
		Long:  i18n.T("cmd.workflow.list.long"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			c, err := workflowCmdContext(cmd)
			if err != nil {
				return err
			}
			w, err := newWorkflowService(cmd.Context()).Workspace(cmd.Context(), c)
			if err != nil {
				return err
			}
			return printWorkflowList(cmd.OutOrStdout(), w, asJSON)
		},
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.workflow.list.flags.json"))
	addWorkflowContextFlags(cmd)
	return cmd
}

// printWorkflowList prints the catalogue, the current member's drafts, the
// integrity warnings and the offline queue. JSON: one array, drafts marked
// "draft": true.
func printWorkflowList(w io.Writer, ws *workflowsvc.Workspace, asJSON bool) error {
	if asJSON {
		list := append(append([]workflowsvc.Summary{}, ws.Workflows...), ws.Drafts...)
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(list)
	}
	if len(ws.Workflows) == 0 && len(ws.Drafts) == 0 {
		fmt.Fprintln(w, i18n.T("cmd.workflow.list.empty"))
	} else {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\n",
			i18n.T("cmd.workflow.list.col_id"), i18n.T("cmd.workflow.list.col_layer"), i18n.T("cmd.workflow.list.col_version"),
			i18n.T("cmd.workflow.list.col_risk"), i18n.T("cmd.workflow.list.col_runtime"), i18n.T("cmd.workflow.list.col_description"))
		invalid := 0
		for _, s := range ws.Workflows {
			if !s.Valid {
				invalid++
			}
			printWorkflowRow(tw, s)
		}
		if len(ws.Drafts) > 0 {
			fmt.Fprintf(tw, "\t\t\t\t\t\n%s\t\t\t\t\t\n", i18n.Tf("cmd.workflow.list.drafts", ws.Member))
			for _, s := range ws.Drafts {
				printWorkflowRow(tw, s)
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		if invalid > 0 {
			fmt.Fprintln(w, theme.Subtitle.Render(i18n.Tf("cmd.workflow.list.invalid_hint", invalid)))
		}
	}
	if len(ws.Integrity) > 0 {
		fmt.Fprintf(w, "\n%s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.workflow.list.integrity", len(ws.Integrity)))
		for _, d := range ws.Integrity {
			fmt.Fprintf(w, "  · %s\n", d.Message)
			if d.Source != "" {
				fmt.Fprintf(w, "    %s\n", theme.Subtitle.Render(d.Source))
			}
		}
	}
	if len(ws.Queue) > 0 {
		fmt.Fprintf(w, "\n%s\n", i18n.Tf("cmd.workflow.list.queued", len(ws.Queue)))
	}
	return nil
}

func printWorkflowRow(tw io.Writer, s workflowsvc.Summary) {
	icon := theme.SuccessStyle.Render(theme.IconSuccess)
	switch {
	case !s.Valid:
		icon = theme.ErrorStyle.Render(theme.IconError)
	case s.Warnings > 0:
		icon = theme.WarningStyle.Render(theme.IconWarning)
	}
	desc := s.Description
	if desc == "" {
		desc = s.Label
	}
	var marks []string
	if s.Draft {
		marks = append(marks, "✎")
		if !s.Valid {
			marks = append(marks, i18n.Tf("cmd.workflow.list.errors", s.Errors))
		}
	}
	if s.Queued {
		marks = append(marks, "⏳ "+i18n.T("cmd.workflow.list.pending"))
	}
	if len(s.NewBricks) > 0 {
		marks = append(marks, i18n.Tf("cmd.workflow.list.new_bricks", strings.Join(s.NewBricks, ", ")))
	}
	if len(marks) > 0 {
		desc = strings.Join(marks, " · ") + "  " + desc
	}
	fmt.Fprintf(tw, "%s %s\t%s\t%s\t%s\t%s\t%s\n", icon, s.ID, s.Layer, versionLabel(s.Version), s.Risk,
		runtimesLabel(s.Runtimes), desc)
}

func versionLabel(v int) string {
	if v == 0 {
		return "—"
	}
	return fmt.Sprintf("v%d", v)
}

func runtimesLabel(rs []workflow.Runtime) string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = string(r)
	}
	return strings.Join(out, ", ")
}
