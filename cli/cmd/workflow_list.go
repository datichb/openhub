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
		Short: "Liste les workflows disponibles (toutes couches)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			list, err := newWorkflowService(cmd.Context()).Catalog(cmd.Context(), workflowsvc.Context{})
			if err != nil {
				return err
			}
			return printWorkflowList(cmd.OutOrStdout(), list, asJSON)
		},
	}
	cmd.Flags().Bool("json", false, "Sortie JSON")
	return cmd
}

func printWorkflowList(w io.Writer, list []workflowsvc.Summary, asJSON bool) error {
	if asJSON {
		if list == nil {
			list = []workflowsvc.Summary{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(list)
	}
	if len(list) == 0 {
		fmt.Fprintln(w, i18n.T("cmd.workflow.list.empty"))
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\n",
		i18n.T("cmd.workflow.list.col_id"), i18n.T("cmd.workflow.list.col_layer"), i18n.T("cmd.workflow.list.col_version"),
		i18n.T("cmd.workflow.list.col_risk"), i18n.T("cmd.workflow.list.col_runtime"), i18n.T("cmd.workflow.list.col_description"))
	invalid := 0
	for _, s := range list {
		icon := theme.SuccessStyle.Render(theme.IconSuccess)
		if !s.Valid {
			icon = theme.ErrorStyle.Render(theme.IconError)
			invalid++
		} else if s.Warnings > 0 {
			icon = theme.WarningStyle.Render(theme.IconWarning)
		}
		desc := s.Description
		if desc == "" {
			desc = s.Label
		}
		fmt.Fprintf(tw, "%s %s\t%s\t%s\t%s\t%s\t%s\n", icon, s.ID, s.Layer, versionLabel(s.Version), s.Risk,
			runtimesLabel(s.Runtimes), desc)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if invalid > 0 {
		fmt.Fprintln(w, theme.Subtitle.Render(i18n.Tf("cmd.workflow.list.invalid_hint", invalid)))
	}
	return nil
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
