package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func brickPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish <id>|agent:<id>|skill:<ref> -m <message>",
		Short: i18n.T("cmd.brick.publish.short"),
		Long:  i18n.T("cmd.brick.publish.long"),
		Args:  cobra.RangeArgs(0, 1),
		RunE:  runBrickPublish,
	}
	cmd.Flags().StringP("message", "m", "", i18n.T("cmd.brick.publish.flags.message"))
	cmd.Flags().Bool("adopt", false, i18n.T("cmd.brick.publish.flags.adopt"))
	cmd.Flags().BoolP("yes", "y", false, i18n.T("cmd.workflow.publish.flags.yes"))
	addWorkflowContextFlags(cmd)
	return cmd
}

func runBrickPublish(cmd *cobra.Command, args []string) error {
	c, err := workflowCmdContext(cmd)
	if err != nil {
		return err
	}
	svc := newWorkflowService(ctxOf(cmd))
	out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
	message, _ := cmd.Flags().GetString("message")
	if strings.TrimSpace(message) == "" {
		return errors.New(i18n.T("cmd.workflow.publish.message_required"))
	}
	if adopt, _ := cmd.Flags().GetBool("adopt"); adopt {
		if len(args) != 0 {
			return errors.New(i18n.T("cmd.brick.publish.adopt_no_id"))
		}
		if ok, err := confirm(cmd, i18n.T("cmd.brick.publish.confirm_adopt")); err != nil || !ok {
			return err
		}
		p, err := svc.AdoptBricks(ctxOf(cmd), c, message)
		if err != nil {
			return brickError(errOut, err)
		}
		fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.brick.publish.adopted", len(p.Adopted), strings.Join(p.Adopted, ", ")))
		return nil
	}
	if len(args) != 1 {
		return errors.New(i18n.T("cmd.brick.publish.usage"))
	}
	preview, err := svc.PreviewBrick(ctxOf(cmd), c, args[0])
	if err != nil {
		return brickError(errOut, err)
	}
	fmt.Fprintln(out, i18n.Tf("cmd.brick.publish.preview", preview.Ref.String(), preview.Next))
	printDiagnostics(out, preview.Diagnostics)
	if len(preview.Unsealed) > 0 {
		return errors.New(i18n.Tf("cmd.brick.unsealed", strings.Join(preview.Unsealed, ", ")))
	}
	if preview.Diagnostics.HasErrors() {
		return errors.New(i18n.Tf("cmd.brick.invalid", preview.Ref.String(), len(preview.Diagnostics.Errors())))
	}
	if preview.Shadows != "" {
		ok, err := confirm(cmd, i18n.Tf("cmd.brick.publish.confirm_shadow", preview.Ref.String(), preview.Shadows))
		if err != nil || !ok {
			return err
		}
	}
	p, err := svc.PublishBrick(ctxOf(cmd), c, preview.Ref.String(), message)
	if err != nil {
		return brickError(errOut, err)
	}
	fmt.Fprintf(out, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.brick.publish.published", p.Ref.String(), p.Version))
	if p.Shadows != "" {
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("cmd.brick.publish.shadows", p.Shadows))
	}
	return nil
}

func brickHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <id>|agent:<id>|skill:<ref>",
		Short: i18n.T("cmd.brick.history.short"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := workflowCmdContext(cmd)
			if err != nil {
				return err
			}
			versions, err := newWorkflowService(ctxOf(cmd)).BrickHistory(ctxOf(cmd), c, args[0])
			if err != nil {
				return brickError(cmd.ErrOrStderr(), err)
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
		},
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.workflow.history.flags.json"))
	addWorkflowContextFlags(cmd)
	return cmd
}
