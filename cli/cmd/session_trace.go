package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// `oh session trace` (E11, P0.4): the whole history of a session, one event
// per line (JSONL with --json), secrets masked. The SessionService gathers it.

func init() {
	cmd := &cobra.Command{
		Use:   "trace <session-id>",
		Short: i18n.T("cmd.session.trace.short"),
		Long:  i18n.T("cmd.session.trace.long"),
		Args:  cobra.ExactArgs(1),
		RunE:  runSessionTrace,
	}
	cmd.Flags().Bool("json", false, i18n.T("cmd.session.trace.flags.json"))
	cmd.Flags().StringP("output", "o", "", i18n.T("cmd.session.trace.flags.output"))
	sessionCmd.AddCommand(cmd)
}

func runSessionTrace(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	svc, err := sessionService(cmd)
	if err != nil {
		return err
	}
	sess, err := svc.Resolve(ctx, args[0])
	if err != nil {
		return sessionRefError(args[0], err)
	}
	opts := sessionsvc.TraceOptions{}
	if store != nil {
		opts.Usage = sqlite.NewUsageStore(store)
	}
	tr, err := svc.Trace(ctx, sess.ID, opts)
	if err != nil {
		return err
	}
	appendNextEvents(ctx, MustApp(), tr, sess.ID)
	if out, _ := cmd.Flags().GetString("output"); out != "" {
		f, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		if err := tr.WriteJSONL(f); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf("cmd.session.trace.written", len(tr.Events), out))
		return nil
	}
	if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
		return tr.WriteJSONL(cmd.OutOrStdout())
	}
	printTrace(cmd.OutOrStdout(), tr)
	return nil
}

// printTrace prints a trace as a readable timeline.
func printTrace(w io.Writer, tr *sessionsvc.Trace) {
	fmt.Fprintln(w, i18n.Tf("cmd.session.trace.title", tr.SessionID, i18n.T("cmd.session.trace.source."+tr.Transcript)))
	for _, e := range tr.Events {
		if e.Kind == sessionsvc.TraceSession || e.Kind == sessionsvc.TraceSummary {
			continue
		}
		who := e.Agent
		if e.InSubagent && who == "" {
			who = "↳"
		}
		fmt.Fprintf(w, "%s  %-10s %-14s %s\n", e.Time.Local().Format("15:04:05"), e.Kind, clipRunes(who, 14), traceLine(e))
	}
	if n := len(tr.Events); n > 0 {
		d := tr.Events[n-1].Data
		fmt.Fprintln(w, i18n.Tf("cmd.session.trace.total", d["cost_usd"], d["tokens_in"], d["tokens_out"], d["tool_calls"], d["subagent_calls"], d["decisions"], d["denied"]))
	}
}

// traceLine summarizes one event.
func traceLine(e sessionsvc.TraceEvent) string {
	var s string
	switch e.Kind {
	case sessionsvc.TracePrompt, sessionsvc.TraceText:
		s = oneLineText(e.Text)
	case sessionsvc.TraceSkill:
		s = e.Skill
	case sessionsvc.TraceSubagent:
		desc, _ := e.Input["description"].(string)
		s = "→ " + e.Subagent + "  " + oneLineText(desc)
	case sessionsvc.TraceTool:
		s = e.Tool + "  " + oneLineText(toolArg(e.Input)) + "  " + e.Status
		if e.Verification {
			s += "  " + i18n.T("cmd.session.trace.verification")
		}
		if e.Denied {
			s += "  " + i18n.T("cmd.session.trace.denied")
		}
	case sessionsvc.TraceDecision:
		s = e.DecisionKind + "  " + oneLineText(strings.TrimSpace(e.Title+" "+e.Action+" "+strings.Join(e.Resources, " ")))
		if e.Resolution != "" {
			s += "  → " + e.Resolution
		} else if e.ResolvedAt == nil {
			s += "  → " + i18n.T("cmd.session.trace.open")
		}
	case sessionsvc.TraceCheckpoint:
		s = e.Event + "  " + e.Checkpoint
		if e.By != "" {
			s += "  (" + e.By + ")"
		}
		if e.Identity != "" {
			s += "  " + i18n.Tf("cmd.session.trace.identity", e.Identity)
		}
		if e.ApproverMismatch {
			s += "  " + i18n.Tf("cmd.session.trace.approver_mismatch", strings.Join(e.Approver, ", "))
		}
	case sessionsvc.TraceUsage:
		s = fmt.Sprintf("$%.4f  %d → %d", e.Cost, e.TokensIn, e.TokensOut)
	case sessionsvc.TraceEnd:
		s = e.Outcome
	case sessionsvc.TraceOutput:
		s = fmt.Sprintf("%s = %v", e.Title, e.Data["value"])
	}
	return clipRunes(s, 160)
}

func toolArg(in map[string]any) string {
	for _, k := range []string{"command", "filePath", "path", "pattern", "query", "url", "description"} {
		if v, ok := in[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func oneLineText(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " …"
	}
	return s
}

func clipRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
