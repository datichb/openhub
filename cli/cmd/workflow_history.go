package cmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func init() {
	workflowCmd.AddCommand(workflowHistoryCmd())
}

func workflowHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <id>|<layer>:<id>",
		Short: i18n.T("cmd.workflow.history.short"),
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowHistory,
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.workflow.history.flags.json"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runWorkflowHistory(cmd *cobra.Command, args []string) error {
	c, err := workflowCmdContext(cmd)
	if err != nil {
		return err
	}
	versions, err := newWorkflowService(ctxOf(cmd)).History(ctxOf(cmd), c, args[0])
	if err != nil {
		return workflowEditError(cmd.ErrOrStderr(), err)
	}
	out := cmd.OutOrStdout()
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(versions)
	}
	fmt.Fprintln(out, theme.Bold.Render(versions[0].Ref.String()))
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i18n.T("cmd.workflow.history.col_version"), i18n.T("cmd.workflow.history.col_date"),
		i18n.T("cmd.workflow.history.col_author"), i18n.T("cmd.workflow.history.col_message"))
	for _, v := range versions {
		ver := fmt.Sprintf("v%d", v.Version)
		if v.Current {
			ver += " " + i18n.T("cmd.workflow.history.current")
		}
		date := ""
		if !v.Entry.PublishedAt.IsZero() {
			date = v.Entry.PublishedAt.Local().Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ver, date, v.Entry.PublishedBy, v.Entry.Message)
	}
	return w.Flush()
}
