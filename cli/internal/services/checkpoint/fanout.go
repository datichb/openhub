package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/textfmt"
)

// Fan-out (R6, E3, CP-R3), the daemon side:
//
//   - control substitution: a delegated checkpoint of a child session
//     (cp-2) does not pause, if and only if oh recorded the session as a
//     child of a running fan-out parent that lists it, launched with the
//     experimental flag, whose bundle delegates this checkpoint to a
//     mandatory merge checkpoint verified by oh on the integration branch
//     (sessionspec.WorkflowRuntime.Substitutes). Everything is read again
//     at each call, from the records oh wrote: nothing an agent declares.
//     The verification of the checkpoint still runs, the pass is traced;
//   - no session of a fan-out pushes, merges, closes a ticket or moves its
//     branches (bundle.FanoutRoleRules, GuardClose);
//   - the merge checkpoint passes only once every child is done, on an
//     integration branch where oh left it, after its verification on that
//     branch (never overridden); oh then merges the integration branch
//     itself (Integrate) before the checkpoint is recorded as passed.

// ErrFanoutNotReady refuses the merge checkpoint while children run.
var ErrFanoutNotReady = errors.New("the children of the fan-out are not done")

// TimelineDelegated marks a checkpoint passed by delegation (CP-R3).
const TimelineDelegated = "delegated"

// delegation returns the parent and its merge checkpoint when checkpoint
// c of sess is delegated (CP-R3), ok false otherwise.
func (s *Service) delegation(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef) (parent, merge string, ok bool) {
	f := sess.Fanout
	if f == nil || f.Role != domain.RoleChild || f.Parent == "" || f.Parent == sess.ID || s.Sessions == nil {
		return "", "", false
	}
	p, pwf, err := s.session(ctx, f.Parent)
	if err != nil || p.Fanout == nil || p.Fanout.Role != domain.RoleParent || !p.Fanout.Experimental {
		return "", "", false
	}
	if p.ProjectID != sess.ProjectID || terminalRun(p.State) || p.Fanout.Status != domain.FanoutRunning {
		return "", "", false
	}
	child, listed := p.Fanout.Child(f.Ticket)
	if !listed || child.Session != sess.ID || child.State != domain.ChildRunning {
		return "", "", false
	}
	if !pwf.Substitutes(c.ID) {
		return "", "", false
	}
	return p.ID, pwf.Fanout.Merge, true
}

func terminalRun(st domain.RunState) bool {
	return st == domain.RunCompleted || st == domain.RunFailed || st == domain.RunStopped
}

// fanoutRole returns the role of sess in a fan-out ("" = none).
func fanoutRole(sess *domain.Session) string {
	if sess == nil || sess.Fanout == nil {
		return ""
	}
	return string(sess.Fanout.Role)
}

// isFanoutMerge reports whether c is the merge checkpoint of the fan-out
// of wf.
func isFanoutMerge(wf *sessionspec.WorkflowRuntime, c sessionspec.CheckpointDef) bool {
	return wf != nil && wf.Fanout != nil && wf.Fanout.Merge == c.ID
}

// fanoutReady checks that the merge checkpoint may pass: sess is the
// parent of a fan-out whose children are all done, and the integration
// branch is where oh left it.
func (s *Service) fanoutReady(ctx context.Context, sess *domain.Session) error {
	f := sess.Fanout
	if f == nil || f.Role != domain.RoleParent || f.Integration == nil {
		return &LockError{Msg: i18n.T("cmd.fanout.not_started")}
	}
	switch f.Status {
	case domain.FanoutReady:
	case domain.FanoutMerged:
		return &LockError{Msg: i18n.T("cmd.fanout.already_merged")}
	default:
		return &LockError{Msg: i18n.Tf("cmd.fanout.not_ready", string(f.Status))}
	}
	head, _, err := s.headErr(ctx, f.Integration.Path)
	if err != nil {
		return &LockError{Msg: i18n.Tf("cmd.fanout.integration_unreadable", f.Integration.Branch, err)}
	}
	if head != f.Integration.Head {
		return &LockError{Msg: i18n.Tf("cmd.fanout.integration_moved", f.Integration.Branch, short(f.Integration.Head), short(head))}
	}
	return nil
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// verifySession is the session as its verification of c sees it: the
// integration worktree of the fan-out for `on: integration` (no location
// when it has none: the verification cannot run, the checkpoint waits).
func verifySession(sess *domain.Session, c sessionspec.CheckpointDef) *domain.Session {
	if c.Verify == nil || c.Verify.On != sessionspec.VerifyOnIntegration {
		return sess
	}
	v := *sess
	v.LaunchPath, v.StartRef = "", ""
	if f := sess.Fanout; f != nil && f.Role == domain.RoleParent && f.Integration != nil {
		v.LaunchPath, v.StartRef = f.Integration.Path, f.Integration.Base
	}
	return &v
}

// guardFanoutClose refuses a ticket closing in a session of a fan-out: oh
// closes the tickets of the children it merged, at the merge checkpoint.
func guardFanoutClose(sess *domain.Session) error {
	if fanoutRole(sess) == "" {
		return nil
	}
	return &LockError{Msg: i18n.T("cmd.fanout.close_refused")}
}

// fanoutRules are the session rules of the role of sess in a fan-out.
func fanoutRules(sess *domain.Session) []sessionspec.PermissionRule {
	return bundle.FanoutRoleRules(fanoutRole(sess))
}

// fanoutLockedOps adds the operations a fan-out session never runs.
func fanoutLockedOps(sess *domain.Session, ops []string) []string {
	for _, op := range bundle.FanoutLockedOps(fanoutRole(sess)) {
		if !slices.Contains(ops, op) {
			ops = append(ops, op)
		}
	}
	return ops
}

// FanoutStatus is the fan-out part of workflow_status.
type FanoutStatus struct {
	Role     string              `json:"role"`
	Status   string              `json:"status,omitempty"`
	Parent   string              `json:"parent,omitempty"`
	Ticket   string              `json:"ticket,omitempty"`
	Branch   string              `json:"integration_branch,omitempty"`
	Children []FanoutChildStatus `json:"children,omitempty"`
	Budget   string              `json:"budget,omitempty"`
}

// FanoutChildStatus is a child in FanoutStatus.
type FanoutChildStatus struct {
	Ticket string `json:"ticket"`
	// Task is the title of the task of the child (sweep-fanout).
	Task      string   `json:"task,omitempty"`
	Session   string   `json:"session,omitempty"`
	State     string   `json:"state"`
	Merge     string   `json:"merge,omitempty"`
	Conflicts []string `json:"conflicts,omitempty"`
	Error     string   `json:"error,omitempty"`
}

func fanoutStatus(sess *domain.Session) *FanoutStatus {
	f := sess.Fanout
	if f == nil {
		return nil
	}
	st := &FanoutStatus{Role: string(f.Role), Status: string(f.Status), Parent: f.Parent, Ticket: f.Ticket}
	if f.Integration != nil {
		st.Branch = f.Integration.Branch
	}
	for _, c := range f.Children {
		task := ""
		if c.Task != nil {
			task = c.Task.Title
		}
		st.Children = append(st.Children, FanoutChildStatus{Ticket: c.Ticket, Task: task, Session: c.Session, State: string(c.State), Merge: short(c.Merge), Conflicts: c.Conflicts, Error: c.Error})
	}
	if f.BudgetUSD > 0 {
		st.Budget = fmt.Sprintf("%.2f / %.2f USD", f.SpentUSD, f.BudgetUSD)
	}
	return st
}

// integrate runs the merge by oh of the integration branch (Integrate) for
// the merge checkpoint c of sess.
func (s *Service) integrate(ctx context.Context, sess *domain.Session, c sessionspec.CheckpointDef) (string, error) {
	if s.Integrate == nil {
		return "", &LockError{Msg: i18n.T("cmd.fanout.no_integrate")}
	}
	msg, err := s.Integrate(ctx, sess, c.ID)
	if err != nil {
		var le *LockError
		if errors.As(err, &le) {
			return "", err
		}
		return "", &LockError{Msg: i18n.Tf("cmd.fanout.integrate_failed", strings.TrimSpace(err.Error()))}
	}
	return msg, nil
}

// isMergeDecision reports whether checkpoint id of a session is the merge
// checkpoint of its fan-out.
func (s *Service) isMergeDecision(ctx context.Context, sessionID, id string) bool {
	_, wf, err := s.session(ctx, sessionID)
	if err != nil {
		return false
	}
	c, ok := wf.Checkpoint(id)
	return ok && (isFanoutMerge(wf, c) || mergeCheckpoint(c))
}

// fanoutFix asks for a fix round when the merge checkpoint id of a fan-out
// is refused with an instruction: oh runs it on the integration branch,
// then the checkpoint is passed again, verified again (CP-R3 unchanged).
// It returns the line for the agent ("" = no fix round).
func (s *Service) fanoutFix(ctx context.Context, sessionID, id, instruction string) string {
	if s.RequestFix == nil {
		return ""
	}
	sess, wf, err := s.session(ctx, sessionID)
	if err != nil || wf.Fanout == nil || wf.Fanout.Fix == "" {
		return ""
	}
	c, ok := wf.Checkpoint(id)
	if !ok || !isFanoutMerge(wf, c) {
		return ""
	}
	failure := ""
	if st, err := s.state(ctx, sessionID); err == nil {
		if r, ok := st.Verify[id]; ok && r.Done() && !r.OK() {
			lines := []string{r.Status}
			for _, c := range r.Commands {
				lines = append(lines, fmt.Sprintf("$ %s → %d", c.Cmd, c.Exit))
			}
			lines = append(lines, r.Reason, textfmt.ClipBytesLeft(r.Excerpt, 2000))
			failure = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	return s.RequestFix(ctx, sess, wf, instruction, failure)
}
