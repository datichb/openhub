package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// Validation by batch (R6, CP-R6): `oh session approve --batch` lists the
// checkpoints waiting (one line each: decision, session, checkpoint,
// summary, verification by oh), then answers them one by one; --except
// leaves one waiting, --reject refuses one, --dry-run only lists.

func runApproveBatch(cmd *cobra.Command, svc *sessionsvc.Service) error {
	ctx := cmd.Context()
	sel := sessionsvc.BatchSelection{Reject: map[string]string{}}
	sel.Sessions, _ = cmd.Flags().GetStringArray("session")
	sel.Checkpoints, _ = cmd.Flags().GetStringSlice("checkpoint")
	sel.Except, _ = cmd.Flags().GetStringArray("except")
	rejects, _ := cmd.Flags().GetStringArray("reject")
	for _, r := range rejects {
		id, msg, _ := strings.Cut(r, "=")
		sel.Reject[strings.TrimSpace(id)] = strings.TrimSpace(msg)
	}
	if parent, _ := cmd.Flags().GetString("fanout"); parent != "" {
		sess, err := svc.Resolve(ctx, parent)
		if err != nil {
			return sessionRefError(parent, err)
		}
		sel.Sessions = append(sel.Sessions, fanoutFamily(ctx, MustApp(), sess)...)
	}
	items, err := svc.Inbox(ctx, domain.DecisionFilter{})
	if err != nil {
		return err
	}
	batch := sessionsvc.SelectBatch(items, sel)
	w := cmd.OutOrStdout()
	if len(batch) == 0 {
		fmt.Fprintln(w, i18n.T("cmd.session.batch.empty"))
		return nil
	}
	dry, _ := cmd.Flags().GetBool("dry-run")
	if !dry {
		batch = sessionsvc.ApplyBatch(ctx, batch, func(ctx2 context.Context, r sessionsvc.Reply) error { return decisionError(svc.Decide(ctx2, r)) })
	}
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, i18n.T("cmd.session.batch.header"))
	failed := 0
	for _, it := range batch {
		verify := batchVerify(ctx, svc, it)
		result := i18n.T("cmd.session.batch." + string(it.Action))
		if dry {
			result = i18n.Tf("cmd.session.batch.dry", result)
		}
		if it.Error != "" {
			result, failed = i18n.Tf("cmd.session.batch.failed", it.Error), failed+1
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", it.Decision.ID, it.Decision.SessionID, it.Checkpoint,
			truncate(sessionsvc.DecisionSummary(it.Decision), 60), verify, result)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	// CP-R9: a checkpoint validated out of its approver, item by item.
	for _, it := range batch {
		if it.Action != sessionsvc.BatchApprove || it.Error != "" {
			continue
		}
		if ac := approverOf(ctx, svc, it.Decision); ac.Mismatch {
			key := "cmd.session.approver_mismatch_done"
			if dry {
				key = "cmd.session.approver_mismatch_dry"
			}
			fmt.Fprintln(cmd.ErrOrStderr(), i18n.Tf("cmd.session.batch.approver", it.Decision.ID, it.Checkpoint,
				i18n.Tf(key, firstNonEmpty(ac.Identity.String(), "?"), ac.ApproverText())))
		}
	}
	if failed > 0 {
		return errors.New(i18n.Tf("cmd.session.batch.some_failed", failed, len(batch)))
	}
	return nil
}

// fanoutFamily is the fan-out of sess for a batch: its parent and every
// child and merger of it (sess alone when it is in no fan-out).
func fanoutFamily(ctx context.Context, a *app.App, sess *domain.Session) []string {
	parent := sess.ID
	if f := sess.Fanout; f != nil && f.Role != domain.RoleParent && f.Parent != "" {
		parent = f.Parent
	}
	return append([]string{parent}, fanoutChildren(ctx, a, parent)...)
}

// batchVerify is the state of the verification by oh of the checkpoint of
// a batch item ("—" = none).
func batchVerify(ctx context.Context, svc *sessionsvc.Service, it sessionsvc.BatchItem) string {
	if svc.Checkpoints != nil {
		if r, ok := svc.Checkpoints.VerifyResult(ctx, it.Decision.SessionID, it.Checkpoint); ok {
			return string(r.Status)
		}
	}
	return "—"
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// approverOf is the approver check of a checkpoint decision (CP-R9; zero
// without the checkpoint service).
func approverOf(ctx context.Context, svc *sessionsvc.Service, d domain.Decision) checkpoint.ApproverCheck {
	if svc.Checkpoints == nil || d.Kind != domain.DecisionCheckpoint {
		return checkpoint.ApproverCheck{}
	}
	id, _ := d.Payload.Data[checkpoint.DataCheckpoint].(string)
	return svc.Checkpoints.Approver(ctx, d.SessionID, id)
}

// approving reports whether a decision choice validates a checkpoint.
func approving(choice string) bool {
	switch choice {
	case "", "once", "always", checkpoint.ChoiceApprove, checkpoint.ChoiceOverride:
		return true
	}
	return false
}

// printApprover prints the approver of a checkpoint answered, and the
// warning when the person who validated is none of them (traced, not
// refused on this machine).
func printApprover(cmd *cobra.Command, ac checkpoint.ApproverCheck, approved bool) {
	if !ac.Expected() {
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), i18n.Tf("cmd.session.approver", ac.ApproverText()))
	if ac.Mismatch && approved {
		fmt.Fprintln(cmd.ErrOrStderr(), "⚠ "+i18n.Tf("cmd.session.approver_mismatch_done", firstNonEmpty(ac.Identity.String(), "?"), ac.ApproverText()))
	}
}
