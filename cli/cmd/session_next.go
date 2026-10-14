package cmd

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// What follows a session (R5, E4): the outputs oh recorded and the `next:`
// steps of its workflow, as `oh run` commands. Shown when an attached
// session ends (oh run) and by `oh session next`; never launched by oh.

func init() {
	cmd := &cobra.Command{
		Use:   "next <session-id>",
		Short: i18n.T("cmd.session.next.short"),
		Args:  cobra.ExactArgs(1),
		RunE:  runSessionNext,
	}
	sessionCmd.AddCommand(cmd)
}

func runSessionNext(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	svc, err := sessionService(cmd)
	if err != nil {
		return err
	}
	sess, err := svc.Resolve(ctx, args[0])
	if err != nil {
		return sessionRefError(args[0], err)
	}
	if n := printNextSteps(ctx, MustApp(), cmd.OutOrStdout(), sess.ID); n == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), i18n.T("cmd.session.next.none"))
	}
	return nil
}

// printNextSteps prints the recorded outputs of a session and the steps
// proposed after it; it returns the number of steps.
func printNextSteps(ctx context.Context, a *app.App, w io.Writer, sessionID string) int {
	if a == nil || a.Sessions == nil {
		return 0
	}
	sess, err := a.Sessions.Get(ctx, sessionID)
	if err != nil || sess.WorkflowID == "" {
		return 0
	}
	steps, err := newWorkflowService(ctx).NextSteps(ctx, workflowsvc.Context{ProjectID: sess.ProjectID}, sess.WorkflowID, sess.Outputs)
	if err != nil {
		return 0
	}
	if len(sess.Outputs) > 0 {
		keys := make([]string, 0, len(sess.Outputs))
		for k := range sess.Outputs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintln(w, theme.Subtitle.Render(i18n.T("cmd.session.next.outputs")))
		for _, k := range keys {
			fmt.Fprintf(w, "  %s : %s\n", k, outputText(sess.Outputs[k]))
		}
	}
	for _, s := range steps {
		label := s.WorkflowID
		if s.Label != "" && s.Label != s.WorkflowID {
			label += " — " + s.Label
		}
		fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconArrow), i18n.Tf("cmd.session.next.step", label))
		fmt.Fprintf(w, "    %s\n", nextCommand(s, sess.ID))
	}
	return len(steps)
}

// nextCommand is the command line launching a step after sessionID.
func nextCommand(s workflowsvc.ChainSuggestion, sessionID string) string {
	line := "oh run " + s.WorkflowID + " --parent " + sessionID
	if len(s.Tickets) > 0 {
		line += " --tickets " + strings.Join(s.Tickets, ",")
	}
	for _, k := range sortedStringKeys(s.Prefill) {
		line += " -i " + k + "=" + shellQuote(s.Prefill[k])
	}
	return line
}

// appendNextEvents adds to a trace the workflows proposed after its session
// (R5: kind `next`, Title = workflow, Text = its inputs), before the summary.
func appendNextEvents(ctx context.Context, a *app.App, tr *sessionsvc.Trace, sessionID string) {
	if a == nil || a.Sessions == nil || tr == nil || len(tr.Events) == 0 {
		return
	}
	sess, err := a.Sessions.Get(ctx, sessionID)
	if err != nil || sess.WorkflowID == "" {
		return
	}
	steps, err := newWorkflowService(ctx).NextSteps(ctx, workflowsvc.Context{ProjectID: sess.ProjectID}, sess.WorkflowID, sess.Outputs)
	if err != nil || len(steps) == 0 {
		return
	}
	last := tr.Events[len(tr.Events)-1]
	head := append([]sessionsvc.TraceEvent(nil), tr.Events[:len(tr.Events)-1]...)
	for _, s := range steps {
		var in []string
		if len(s.Tickets) > 0 {
			in = append(in, "tickets="+strings.Join(s.Tickets, ","))
		}
		for _, k := range sortedStringKeys(s.Prefill) {
			in = append(in, k+"="+s.Prefill[k])
		}
		head = append(head, sessionsvc.TraceEvent{Time: last.Time, Kind: sessionsvc.TraceNext, ToolSession: sessionID, Title: s.WorkflowID, Text: strings.Join(in, " ")})
	}
	head = append(head, last)
	tr.Events = head
	for i := range tr.Events {
		tr.Events[i].Seq = i + 1
	}
}
