package fanout

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Fix round of a fan-out: the merge checkpoint of the parent refused with
// an instruction (its verification on the integration branch failed, or
// the review asks for changes), oh records the request (RequestFix, in the
// daemon) and the driver launches a session of the fix workflow (`fanout.
// fix`, quick) in the integration worktree, role fixer: no push, merge,
// ticket closing nor branch move, its commit checkpoint validated by the
// user (never delegated). Its work done, oh checks it (commits on top of
// the integration head, no merge commit, nothing left uncommitted, still
// on the integration branch) and takes the new head; else the branch goes
// back where it was. The parent then passes the merge checkpoint again,
// verified again by oh on the integration branch: CP-R3 does not change.

// Errors of a fix request.
var (
	ErrNoFix      = errors.New("this fan-out has no fix round")
	ErrFixRounds  = errors.New("no fix round left")
	ErrFixPending = errors.New("the fan-out is not waiting for its merge checkpoint")
)

// DefaultFixRounds is the number of fix rounds when the workflow sets
// none (limits.max_fix_rounds).
const DefaultFixRounds = 1

// FixRounds is the number of fix rounds of a workflow.
func FixRounds(wf *sessionspec.WorkflowRuntime) int {
	if wf != nil && wf.MaxFixRounds > 0 {
		return wf.MaxFixRounds
	}
	return DefaultFixRounds
}

// RequestFix records a fix round of the fan-out of the parent sess, with
// the instruction of the refusal (and what failed in the verification):
// the driver launches it. It returns the round and the number of rounds.
func RequestFix(ctx context.Context, store Store, sess *domain.Session, wf *sessionspec.WorkflowRuntime, instruction, failure string) (round, rounds int, err error) {
	if wf == nil || wf.Fanout == nil || wf.Fanout.Fix == "" {
		return 0, 0, ErrNoFix
	}
	f := sess.Fanout
	if f == nil || f.Role != domain.RoleParent || f.Integration == nil {
		return 0, 0, ErrNotFanout
	}
	rounds = FixRounds(wf)
	if f.Status != domain.FanoutReady {
		return 0, rounds, ErrFixPending
	}
	if len(f.Fixes) >= rounds {
		return 0, rounds, ErrFixRounds
	}
	request := strings.TrimSpace(instruction)
	if failure = strings.TrimSpace(failure); failure != "" {
		request += "\n\n" + i18n.Tf("cmd.fanout.fix_failure", failure)
	}
	rec := *f
	rec.Fixes = append(append([]domain.FanoutFix(nil), f.Fixes...), domain.FanoutFix{Round: len(f.Fixes) + 1, Request: request,
		Base: f.Integration.Head, State: domain.ChildPending})
	rec.Status = domain.FanoutFixing
	rec.AddEvent(domain.FanoutEventFix, "", sess.ID, fmt.Sprintf("round %d/%d", len(rec.Fixes), rounds))
	if err := store.SetSessionFanout(ctx, sess.ID, &rec); err != nil {
		return 0, rounds, err
	}
	return len(rec.Fixes), rounds, nil
}

// fix runs the pending fix round of the fan-out of parent: the fixer in
// the integration worktree, then the check of its work by oh.
func (d *Driver) fix(ctx context.Context, parent string, wf *sessionspec.WorkflowRuntime) {
	p, err := d.Sessions.Get(ctx, parent)
	if err != nil {
		return
	}
	f := p.Fanout
	x, ok := f.LastFix()
	if !ok || f.Integration == nil || x.State.Done() {
		_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) { f.Status = domain.FanoutReady })
		return
	}
	ig := f.Integration
	round := x.Round
	setFix := func(fn func(x *domain.FanoutFix)) {
		_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) {
			if x, ok := f.LastFix(); ok && x.Round == round {
				fn(x)
			}
		})
	}
	sid := x.Session
	if x.State == domain.ChildPending || sid == "" {
		if head, err := Head(ctx, ig.Path); err != nil || head != ig.Head || MergeInProgress(ctx, ig.Path) {
			d.fixDone(ctx, parent, round, ig, "", i18n.Tf("cmd.fanout.integration_moved", ig.Branch, short(ig.Head), short(head)))
			return
		}
		remaining := 0.0
		if f.BudgetUSD > 0 {
			spent := d.spent(ctx, p)
			if spent >= f.BudgetUSD {
				d.budgetReached(ctx, parent, "", spent, f.BudgetUSD)
				d.fixDone(ctx, parent, round, ig, "", i18n.Tf("cmd.fanout.budget_reached", fmt.Sprintf("%.2f / %.2f USD", spent, f.BudgetUSD)))
				return
			}
			remaining = f.BudgetUSD - spent
		}
		rec := &domain.SessionFanout{Role: domain.RoleFixer, Parent: parent, Branch: ig.Branch}
		inputs := map[string]string{}
		if wf.Fanout.FixInput != "" {
			inputs[wf.Fanout.FixInput] = x.Request
		}
		// Listed before the session exists: its record is checked against it.
		setFix(func(x *domain.FanoutFix) { x.State = domain.ChildRunning })
		sid, err = d.Launcher.Start(ctx, LaunchRequest{Parent: p, Workflow: wf.Fanout.Fix, Dir: ig.Path, Record: rec, BudgetUSD: remaining, Inputs: inputs})
		if err != nil {
			d.fixDone(ctx, parent, round, ig, "", err.Error())
			return
		}
		setFix(func(x *domain.FanoutFix) { x.Session = sid })
		_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) { f.AddEvent(domain.FanoutEventFix, "", sid, ig.Branch) })
		d.logf("fan-out %s: fix round %d → session %s on %s", parent, round, sid, ig.Branch)
	}
	fixer := domain.FanoutChild{Session: sid, Path: ig.Path, Head: ig.Head, Branch: ig.Branch}
	why, ok := d.wait(ctx, sid, func() bool { return d.childFinished(ctx, fixer) }, func() bool {
		cur, err := d.Sessions.Get(ctx, parent)
		return err == nil && cur.State == domain.RunStopped // abandoned: await cleans up
	})
	if !ok {
		return
	}
	_ = d.Launcher.Stop(ctx, sid)
	head, reason := d.checkFix(ctx, ig)
	if reason != "" {
		_ = ResetHard(ctx, ig.Path, ig.Head)
	}
	d.fixDone(ctx, parent, round, ig, head, reason)
	d.logf("fan-out %s: fix round %d done (%s) %s", parent, round, why, reason)
}

// checkFix checks the work of the fixer on the integration branch: the
// new head ("" when nothing was committed), or why it is refused.
func (d *Driver) checkFix(ctx context.Context, ig *domain.FanoutIntegration) (head, refused string) {
	if cur, err := git(ctx, ig.Path, "symbolic-ref", "--short", "HEAD"); err != nil || strings.TrimSpace(cur) != ig.Branch {
		return "", i18n.Tf("cmd.fanout.fix_branch", ig.Branch)
	}
	if MergeInProgress(ctx, ig.Path) {
		return "", i18n.T("cmd.fanout.fix_dirty")
	}
	if dirty, err := Dirty(ctx, ig.Path); err != nil || dirty {
		return "", i18n.T("cmd.fanout.fix_dirty")
	}
	head, err := Head(ctx, ig.Path)
	if err != nil {
		return "", err.Error()
	}
	if head == ig.Head {
		return "", i18n.T("cmd.fanout.fix_no_commit")
	}
	if !IsAncestor(ctx, ig.Path, ig.Head, head) {
		return "", i18n.T("cmd.fanout.fix_rewrote")
	}
	if out, err := git(ctx, ig.Path, "rev-list", "--merges", "--count", ig.Head+".."+head); err != nil || strings.TrimSpace(out) != "0" {
		return "", i18n.T("cmd.fanout.fix_merge")
	}
	return head, ""
}

// fixDone ends a fix round: the new head of the integration branch taken
// (head not ""), the fan-out ready for its merge checkpoint again, and
// the parent told.
func (d *Driver) fixDone(ctx context.Context, parent string, round int, ig *domain.FanoutIntegration, head, refused string) {
	f, err := d.update(ctx, parent, func(f *domain.SessionFanout) {
		if x, ok := f.LastFix(); ok && x.Round == round {
			x.State, x.Error = domain.ChildFailed, refused
			if head != "" {
				x.State, x.Head = domain.ChildMerged, head
				f.Integration.Head = head
			}
		}
		if f.Status == domain.FanoutFixing {
			f.Status = domain.FanoutReady
		}
		msg := short(head)
		if refused != "" {
			msg = refused
		}
		f.AddEvent(domain.FanoutEventFixed, "", "", fmt.Sprintf("round %d: %s", round, msg))
	})
	if err != nil || f.Status != domain.FanoutReady {
		return
	}
	text := i18n.Tf("cmd.fanout.fix_done", round, ig.Branch, short(head), short(ig.Head))
	if refused != "" {
		text = i18n.Tf("cmd.fanout.fix_refused", round, refused, ig.Branch, short(ig.Head))
	}
	if err := d.Launcher.Tell(ctx, parent, text); err != nil {
		d.logf("fan-out %s: telling the parent: %v", parent, err)
	}
}

// await follows the fan-out of a parent once its children are done, until
// its merge checkpoint passes or the parent stops: it runs the fix rounds
// asked at the merge checkpoint.
func (d *Driver) await(ctx context.Context, parent string, wf *sessionspec.WorkflowRuntime) error {
	if wf == nil || wf.Fanout == nil || wf.Fanout.Fix == "" {
		return nil
	}
	t := time.NewTicker(d.poll())
	defer t.Stop()
	for {
		p, err := d.Sessions.Get(ctx, parent)
		if err != nil {
			return err
		}
		switch {
		case p.State == domain.RunStopped:
			d.abandonDone(ctx, p)
			return nil
		case terminal(p.State) && p.Fanout.Status == domain.FanoutReady:
			return nil // the parent ended without its merge checkpoint
		case p.Fanout.Status == domain.FanoutFixing:
			d.fix(ctx, parent, wf)
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		case p.Fanout.Status != domain.FanoutReady:
			return nil // merged, failed or stopped
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}
