package daemon

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Git configuration changed during a session (v5 critical N5): the
// checkpoint that unlocks an operation is refused to the agent, and the
// user is told by an alert (✗ in « À traiter », not cleared by the agent
// loop) which file changed and what to do.

// alertGitChanged is the alert of a session whose repository configuration
// or hooks changed since it started.
const alertGitChanged = "git_changed"

// raiseGitChanged records the alert of a checkpoint refused because of err
// (an ErrConfigChanged). One alert per state of the repository: the same
// change refused again (or dismissed) is not raised twice, a further change is.
func (d *Daemon) raiseGitChanged(ctx context.Context, root, checkpointID string, err error) {
	if d.opts.Decisions == nil || d.opts.Sessions == nil {
		return
	}
	sess, serr := d.opts.Sessions.Get(ctx, root)
	if serr != nil {
		return
	}
	var changed *gitutil.ConfigChangedError
	ref := alertGitChanged
	detail := i18n.Tf("cmd.daemon.workflow.git_changed_detail_any", checkpointID, sess.LaunchPath)
	if errors.As(err, &changed) {
		if changed.Fingerprint != "" {
			ref += ":" + changed.Fingerprint[:min(12, len(changed.Fingerprint))]
		}
		if len(changed.Files) > 0 {
			detail = i18n.Tf("cmd.daemon.workflow.git_changed_detail", checkpointID, strings.Join(changed.Files, ", "))
		} else if changed.Common != "" {
			detail = i18n.Tf("cmd.daemon.workflow.git_changed_detail_any", checkpointID, changed.Common)
		}
	}
	dec := domain.Decision{SessionID: root, GroupKey: sess.GroupKey, Kind: domain.DecisionError, ToolRef: ref,
		Payload: domain.DecisionPayload{Title: i18n.T("cmd.daemon.workflow.git_changed_title"), Message: detail,
			Data: map[string]any{dataOhAlert: alertGitChanged}}}
	dec.ID = domain.DecisionID(dec.Kind, root, dec.ToolRef)
	if prev, gerr := d.opts.Decisions.Get(ctx, dec.ID); gerr == nil && prev != nil {
		return // already raised (and maybe dismissed)
	}
	if uerr := d.opts.Decisions.Upsert(ctx, &dec); uerr != nil {
		slog.Warn("ohd: git configuration alert not recorded", "session", root, "error", uerr)
		return
	}
	slog.Warn("ohd: the git configuration of a session changed, checkpoint refused", "session", root, "checkpoint", checkpointID, "error", err)
	d.wmu.Lock()
	w := d.watchers[sess.GroupKey]
	d.wmu.Unlock()
	if w != nil {
		w.refreshAlerts(ctx, root)
		return
	}
	d.feed.publishChange(domain.SessionChange{SessionID: root, GroupKey: sess.GroupKey, Decisions: true})
	d.notes.requestScan()
}
