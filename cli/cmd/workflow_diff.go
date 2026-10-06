package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func init() {
	workflowCmd.AddCommand(workflowDiffCmd())
}

func workflowDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <id>|<layer>:<id>",
		Short: i18n.T("cmd.workflow.diff.short"),
		Long:  i18n.T("cmd.workflow.diff.long"),
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflowDiff,
	}
	cmd.Flags().String("against", "published", i18n.T("cmd.workflow.diff.flags.against"))
	cmd.Flags().String("layer", "", i18n.T("cmd.workflow.edit.flags.layer"))
	cmd.Flags().Bool("json", false, i18n.T("cmd.workflow.diff.flags.json"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runWorkflowDiff(cmd *cobra.Command, args []string) error {
	c, err := workflowCmdContext(cmd)
	if err != nil {
		return err
	}
	svc := newWorkflowService(ctxOf(cmd))
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	draft, layer, err := editTarget(cmd, svc, c, args[0])
	if err != nil {
		return workflowEditError(errOut, err)
	}
	if !draft.Draft {
		return errors.New(i18n.Tf("teamstate.workflow.cli.no_draft", draft.Ref.String()))
	}
	ref := string(layer) + ":" + draft.Ref.ID
	against, _ := cmd.Flags().GetString("against")

	var base *workflowsvc.Text
	label := "published"
	if against != "published" {
		v, err := strconv.Atoi(against)
		if err != nil {
			return errors.New(i18n.Tf("cmd.workflow.diff.bad_against", against))
		}
		if base, err = svc.VersionText(ctxOf(cmd), c, ref, v); err != nil {
			return workflowEditError(errOut, err)
		}
		label = "v" + against
	}
	preview, perr := svc.Preview(ctxOf(cmd), c, ref)
	var invalid *workflowsvc.InvalidError
	if perr != nil && !errors.As(perr, &invalid) {
		return workflowEditError(errOut, perr)
	}
	if base == nil && preview != nil {
		base = preview.Published
	}

	yamlDiff := workflowsvc.UnifiedDiff(workflowsvc.TextString(base, false), string(draft.YAML), ref+"@"+label, ref+"@draft")
	promptDiff := workflowsvc.UnifiedDiff(workflowsvc.TextString(base, true), string(draft.Prompt), "prompt@"+label, "prompt@draft")
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		res := map[string]any{"ref": ref, "against": label, "diff": yamlDiff, "prompt_diff": promptDiff}
		if preview != nil {
			res["impact"], res["next_version"], res["diagnostics"] = preview.Impact, preview.NextVersion, preview.Diagnostics
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	printDiff(out, yamlDiff)
	if promptDiff != "" {
		printDiff(out, promptDiff)
	}
	if yamlDiff == "" && promptDiff == "" {
		fmt.Fprintln(out, i18n.T("cmd.workflow.diff.same"))
	}
	if invalid != nil {
		printDiagnostics(out, invalid.Diagnostics)
		return errors.New(i18n.Tf("teamstate.workflow.cli.invalid", invalid.Ref, len(invalid.Diagnostics.Errors())))
	}
	fmt.Fprintln(out)
	printImpact(out, preview.Impact)
	return nil
}

func printDiff(w io.Writer, diff string) {
	for _, line := range strings.Split(strings.TrimRight(diff, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			fmt.Fprintln(w, theme.Bold.Render(line))
		case strings.HasPrefix(line, "+"):
			fmt.Fprintln(w, theme.SuccessStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			fmt.Fprintln(w, theme.ErrorStyle.Render(line))
		default:
			fmt.Fprintln(w, line)
		}
	}
}
