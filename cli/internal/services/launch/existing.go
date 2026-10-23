package launch

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
)

// ExistingChoiceMessage explains a launch refused for want of a choice about
// the open sessions of its tickets.
func ExistingChoiceMessage(existing []runsvc.ExistingSession) string {
	for _, e := range existing {
		if e.SameWorkflow {
			return i18n.Tf("cmd.run.existing.need_choice", e.ID, e.WorkflowID, strings.Join(e.Shared, ", "))
		}
	}
	return runsvc.ErrNeedsChoice.Error()
}

// DeleteReportText summarizes a deletion.
func DeleteReportText(d *runsvc.DeleteReport) string {
	parts := []string{i18n.Tf("cmd.session.delete.done", d.SessionID)}
	if d.WorktreeRemoved != "" {
		parts = append(parts, i18n.Tf("cmd.session.delete.worktree", d.WorktreeRemoved))
	}
	if d.BranchDeleted != "" {
		parts = append(parts, i18n.Tf("cmd.session.delete.branch", d.BranchDeleted))
	}
	if d.RemoteBranch {
		parts = append(parts, i18n.Tf("cmd.session.delete.remote_branch", d.BranchDeleted))
	}
	if d.BaseChangesKept {
		parts = append(parts, i18n.T("cmd.session.delete.base_kept"))
	}
	for _, e := range d.Errors {
		parts = append(parts, i18n.Tf("cmd.session.delete.not_done", e))
	}
	return strings.Join(parts, " · ")
}
