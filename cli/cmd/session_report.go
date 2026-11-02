package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/report"
	"github.com/datichb/openhub/cli/internal/textfmt"
)

// `oh session report` (modernisation L4): the reports the agents of a
// session filed with workflow_report, or one of them.

func init() {
	cmd := &cobra.Command{
		Use:   "report <session-id> [rp-id]",
		Short: i18n.T("cmd.session.report.short"),
		Long:  i18n.T("cmd.session.report.long"),
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runSessionReport,
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.session.report.flags.json"))
	sessionCmd.AddCommand(cmd)
}

// sessionReportJSON is a report in `oh session report --json`.
type sessionReportJSON struct {
	ID string `json:"id"`
	domain.Report
	Body string `json:"body,omitempty"`
}

func runSessionReport(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	svc, err := sessionService(cmd)
	if err != nil {
		return err
	}
	sess, err := svc.Resolve(ctx, args[0])
	if err != nil {
		return sessionRefError(args[0], err)
	}
	asJSON, _ := cmd.Flags().GetBool("json")
	out := cmd.OutOrStdout()
	if len(args) == 2 {
		r, body, err := svc.Report(ctx, sess.ID, args[1])
		if err != nil {
			return reportError(args[1], err)
		}
		if asJSON {
			return cliout.JSON(out, sessionReportJSON{ID: r.ID(), Report: r, Body: body})
		}
		fmt.Fprintln(out, i18n.Tf("cmd.session.report.header", r.ID(), firstNonEmpty(r.Agent, "?"), r.Title, textfmt.DateTime(r.CreatedAt)))
		fmt.Fprintln(out)
		fmt.Fprintln(out, report.Clean(body))
		return nil
	}
	list, err := svc.SessionReports(ctx, sess.ID)
	if err != nil {
		return err
	}
	if asJSON {
		rows := make([]sessionReportJSON, 0, len(list))
		for _, r := range list {
			rows = append(rows, sessionReportJSON{ID: r.ID(), Report: r})
		}
		return cliout.JSON(out, rows)
	}
	printSessionReports(out, sess.ID, list)
	return nil
}

func printSessionReports(w io.Writer, sessionID string, list []domain.Report) {
	if len(list) == 0 {
		fmt.Fprintln(w, i18n.Tf("cmd.session.report.none", sessionID))
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, i18n.T("cmd.session.report.columns"))
	for _, r := range list {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.ID(), firstNonEmpty(r.Agent, "?"), report.Clean(r.Title),
			textfmt.Bytes(int64(r.Bytes)), textfmt.Clock(r.CreatedAt), strings.Join(r.Sections, " · "))
	}
	_ = tw.Flush()
	fmt.Fprintln(w, i18n.Tf("cmd.session.report.hint", sessionID))
}

func reportError(id string, err error) error {
	switch {
	case errors.Is(err, report.ErrBadID):
		return errors.New(i18n.Tf("cmd.session.report.bad_id", id))
	case errors.Is(err, domain.ErrNotFound):
		return errors.New(i18n.Tf("cmd.session.report.not_found", id))
	case errors.Is(err, report.ErrAltered):
		return errors.New(i18n.Tf("cmd.session.report.altered", id))
	}
	return err
}
