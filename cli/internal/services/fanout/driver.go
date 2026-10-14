package fanout

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// The driver of a fan-out (E3): it runs in an oh process (`oh run`, `oh
// fanout drive`, an evaluation pass), never in an agent.
//
//	parent passes cp-plan ─ integration branch from its HEAD
//	  └ per ticket (dependencies first, at most `parallel` at once):
//	      child branch from the integration head ─ child session (headless)
//	      ─ done ─ oh merges it (--no-ff) ─ conflict? merger session, else unmerged
//	  └ every child done (or stopped by the global budget) ─ ready ─ the parent is told
//
// The record of the parent (domain.SessionFanout) holds the whole state:
// a driver started again goes on where the previous one stopped.

// LaunchRequest is a session the driver launches.
type LaunchRequest struct {
	Parent   *domain.Session
	Workflow string
	Dir      string
	Ticket   string
	// Inputs are inputs of the workflow (merger: the branch merged and the
	// files in conflict).
	Inputs map[string]string
	// Record is the fan-out record of the new session.
	Record *domain.SessionFanout
	// BudgetUSD is what remains of the global budget (0 = no cap).
	BudgetUSD float64
}

// Launcher starts and follows the sessions of a fan-out.
type Launcher interface {
	// Start launches a session without interface and returns its id.
	Start(ctx context.Context, req LaunchRequest) (string, error)
	// Done reports whether a session finished its work: ended, or idle with
	// no decision waiting.
	Done(ctx context.Context, sessionID string) (bool, error)
	// Stop stops a session.
	Stop(ctx context.Context, sessionID string) error
	// Tell sends a message to a session (the parent, once the children are
	// done).
	Tell(ctx context.Context, sessionID, text string) error
}

// Driver runs fan-outs.
type Driver struct {
	Sessions Store
	// Workflow returns the workflow runtime and the checkpoint state of a
	// session (the parent).
	Workflow func(ctx context.Context, sessionID string) (*sessionspec.WorkflowRuntime, domain.CheckpointState, error)
	Launcher Launcher
	// Ticket returns the tickets a ticket depends on and whether it is
	// already closed (nil = bd show).
	Ticket func(ctx context.Context, dir, ticket string) (deps []string, closed bool, err error)
	// Parallel caps the children at the same time (configuration of oh,
	// 0 = the workflow's).
	Parallel int
	// Poll is the interval of the checks (0 = 5 s).
	Poll time.Duration
	// Log receives a line per step (nil = none).
	Log func(format string, args ...any)

	mu      sync.Mutex // the record of the parent
	mergeMu sync.Mutex // the integration worktree
}

// Errors of the driver.
var (
	ErrNotFanout = errors.New("this session is not the parent of a fan-out")
	ErrDisabled  = errors.New("the fan-out is experimental and disabled in this configuration")
)

func (d *Driver) poll() time.Duration {
	if d.Poll > 0 {
		return d.Poll
	}
	return 5 * time.Second
}

func (d *Driver) logf(format string, args ...any) {
	if d.Log != nil {
		d.Log(format, args...)
	}
}

// update changes the record of the parent (read again first).
func (d *Driver) update(ctx context.Context, parent string, fn func(*domain.SessionFanout)) (*domain.SessionFanout, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	p, err := d.Sessions.Get(ctx, parent)
	if err != nil {
		return nil, err
	}
	if p.Fanout == nil {
		return nil, ErrNotFanout
	}
	f := *p.Fanout
	f.Children = slices.Clone(p.Fanout.Children)
	if p.Fanout.Integration != nil {
		ig := *p.Fanout.Integration
		f.Integration = &ig
	}
	fn(&f)
	return &f, d.Sessions.SetSessionFanout(ctx, parent, &f)
}

// Run drives the fan-out of parent until every child is done (or ctx
// ends): it waits for the checkpoints that come before the children,
// creates the integration branch, then launches, follows and merges the
// children, and tells the parent the result.
func (d *Driver) Run(ctx context.Context, parent string) error {
	p, err := d.Sessions.Get(ctx, parent)
	if err != nil {
		return err
	}
	if p.Fanout == nil || p.Fanout.Role != domain.RoleParent {
		return ErrNotFanout
	}
	if !p.Fanout.Experimental {
		return ErrDisabled
	}
	if p.Fanout.Status.Terminal() {
		return nil
	}
	wf, err := d.waitStart(ctx, p)
	if err != nil || wf == nil {
		return err
	}
	if p, err = d.Sessions.Get(ctx, parent); err != nil {
		return err
	}
	if p.Fanout.Integration == nil {
		if err := d.startIntegration(ctx, p); err != nil {
			_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) {
				f.Status = domain.FanoutFailed
				f.AddEvent(domain.FanoutEventFailed, "", "", err.Error())
			})
			return err
		}
	}
	if err := d.runChildren(ctx, parent, wf); err != nil {
		return err
	}
	return d.finish(ctx, parent)
}

// waitStart waits until the parent passed every mandatory checkpoint
// declared before its merge checkpoint (cp-plan). nil when the parent ended
// first (the fan-out is stopped).
func (d *Driver) waitStart(ctx context.Context, p *domain.Session) (*sessionspec.WorkflowRuntime, error) {
	t := time.NewTicker(d.poll())
	defer t.Stop()
	for {
		wf, st, err := d.Workflow(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		if wf == nil || wf.Fanout == nil {
			return nil, ErrNotFanout
		}
		if gatesPassed(wf, st) {
			return wf, nil
		}
		cur, err := d.Sessions.Get(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		if terminal(cur.State) {
			_, err := d.update(ctx, p.ID, func(f *domain.SessionFanout) { f.Status = domain.FanoutStopped })
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.C:
		}
	}
}

// gatesPassed reports whether the mandatory checkpoints before the merge
// checkpoint are passed.
func gatesPassed(wf *sessionspec.WorkflowRuntime, st domain.CheckpointState) bool {
	for _, c := range wf.Checkpoints {
		if c.ID == wf.Fanout.Merge {
			return true
		}
		if c.Mandatory && !st.HasPassed(c.ID) {
			return false
		}
	}
	return true
}

func terminal(s domain.RunState) bool {
	return s == domain.RunCompleted || s == domain.RunFailed || s == domain.RunStopped
}

// startIntegration creates the integration branch from the HEAD of the
// parent, and the children of its tickets.
func (d *Driver) startIntegration(ctx context.Context, p *domain.Session) error {
	base, err := Head(ctx, p.LaunchPath)
	if err != nil {
		return err
	}
	branch := IntegrationBranch(p.ID)
	path, err := AddWorktree(ctx, p.LaunchPath, branch, base)
	if err != nil {
		return err
	}
	_, err = d.update(ctx, p.ID, func(f *domain.SessionFanout) {
		f.Integration = &domain.FanoutIntegration{Branch: branch, Path: path, Base: base, Head: base}
		f.Status = domain.FanoutRunning
		if len(f.Children) == 0 {
			for _, t := range p.Tickets {
				f.Children = append(f.Children, domain.FanoutChild{Ticket: t, State: domain.ChildPending})
			}
		}
		f.AddEvent(domain.FanoutEventStarted, "", "", branch+" "+short(base))
	})
	d.logf("fan-out %s: integration branch %s at %s", p.ID, branch, short(base))
	return err
}

// parallel is the number of children at the same time.
func (d *Driver) parallel(wf *sessionspec.WorkflowRuntime, f *domain.SessionFanout) int {
	n := max(wf.Fanout.Parallel, 1)
	if f.Parallel > 0 {
		n = min(n, f.Parallel)
	}
	if d.Parallel > 0 {
		n = min(n, d.Parallel)
	}
	return min(n, 3)
}

// runChildren launches the children, dependencies first, and waits for
// them all.
func (d *Driver) runChildren(ctx context.Context, parent string, wf *sessionspec.WorkflowRuntime) error {
	p, err := d.Sessions.Get(ctx, parent)
	if err != nil {
		return err
	}
	deps := d.dependencies(ctx, p)
	slots := d.parallel(wf, p.Fanout)
	var wg sync.WaitGroup
	running := map[string]bool{}
	var runMu sync.Mutex
	done := make(chan struct{}, 16)
	var fatal error
	for {
		if ctx.Err() != nil {
			wg.Wait()
			return ctx.Err()
		}
		p, err = d.Sessions.Get(ctx, parent)
		if err != nil {
			wg.Wait()
			return err
		}
		f := p.Fanout
		if f.Status != domain.FanoutRunning {
			wg.Wait()
			return nil // failed or stopped meanwhile
		}
		runMu.Lock()
		n := len(running)
		runMu.Unlock()
		allDone := true
		for _, c := range f.Children {
			if !c.State.Done() {
				allDone = false
			}
			if c.State == domain.ChildRunning && c.Session != "" {
				runMu.Lock()
				resumed := !running[c.Ticket]
				if resumed {
					running[c.Ticket] = true // a previous driver launched it
					n++
				}
				runMu.Unlock()
				if resumed {
					wg.Add(1)
					go func(c domain.FanoutChild) {
						defer wg.Done()
						d.follow(ctx, parent, wf, c)
						runMu.Lock()
						delete(running, c.Ticket)
						runMu.Unlock()
						done <- struct{}{}
					}(c)
				}
			}
		}
		if allDone && n == 0 {
			wg.Wait()
			return fatal
		}
		for _, c := range f.Children {
			runMu.Lock()
			launched := running[c.Ticket]
			runMu.Unlock()
			if n >= slots || c.State != domain.ChildPending || launched {
				continue
			}
			switch ready, blocked := depsState(f, deps[c.Ticket]); {
			case blocked != "":
				_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) {
					if ch, ok := f.Child(c.Ticket); ok {
						ch.State, ch.Error = domain.ChildFailed, i18n.Tf("cmd.fanout.dependency_failed", blocked)
					}
				})
				continue
			case !ready:
				continue
			}
			if spent, budget := d.spent(ctx, p), f.BudgetUSD; budget > 0 && spent >= budget {
				d.budgetReached(ctx, parent, c.Ticket, spent, budget)
				continue
			}
			runMu.Lock()
			running[c.Ticket] = true
			runMu.Unlock()
			n++
			wg.Add(1)
			go func(ticket string) {
				defer wg.Done()
				if err := d.launch(ctx, parent, wf, ticket); err != nil {
					d.logf("fan-out %s: %s: %v", parent, ticket, err)
				}
				runMu.Lock()
				delete(running, ticket)
				runMu.Unlock()
				done <- struct{}{}
			}(c.Ticket)
		}
		select {
		case <-ctx.Done():
		case <-done:
		case <-time.After(d.poll()):
		}
	}
}

// dependencies reads, for each ticket of the fan-out, the tickets of the
// fan-out it depends on; a ticket already closed gets no child (the plan
// leaves it out).
func (d *Driver) dependencies(ctx context.Context, p *domain.Session) map[string][]string {
	get := d.Ticket
	if get == nil {
		get = bdTicket
	}
	in := map[string]bool{}
	for _, c := range p.Fanout.Children {
		in[c.Ticket] = true
	}
	out := map[string][]string{}
	for _, c := range p.Fanout.Children {
		list, closed, err := get(ctx, p.LaunchPath, c.Ticket)
		if err != nil {
			continue
		}
		if closed && c.State == domain.ChildPending {
			_, _ = d.update(ctx, p.ID, func(f *domain.SessionFanout) {
				if ch, ok := f.Child(c.Ticket); ok {
					ch.State, ch.Error = domain.ChildFailed, i18n.T("cmd.fanout.ticket_closed")
				}
			})
			continue
		}
		for _, t := range list {
			if in[t] && t != c.Ticket {
				out[c.Ticket] = append(out[c.Ticket], t)
			}
		}
	}
	return out
}

func bdTicket(_ context.Context, dir, ticket string) ([]string, bool, error) {
	t, err := beads.Show(dir, ticket)
	if err != nil {
		return nil, false, err
	}
	var deps []string
	for _, dep := range t.Dependencies {
		if dep.ID != "" && (dep.Type == "" || dep.Type == "blocks") {
			deps = append(deps, dep.ID)
		}
	}
	return deps, beads.IsClosedStatus(t.Status), nil
}

// depsState tells whether every dependency is merged (ready), or the first
// one that will never be (blocked).
func depsState(f *domain.SessionFanout, deps []string) (ready bool, blocked string) {
	ready = true
	for _, t := range deps {
		c, ok := f.Child(t)
		switch {
		case !ok:
		case c.State == domain.ChildMerged:
		case c.State.Done():
			return false, t
		default:
			ready = false
		}
	}
	return ready, ""
}

// spent is what the parent and its sessions spent (F-04).
func (d *Driver) spent(ctx context.Context, p *domain.Session) float64 {
	total := 0.0
	if cur, err := d.Sessions.Get(ctx, p.ID); err == nil {
		total += cur.Cost
		p = cur
	}
	for _, c := range p.Fanout.Children {
		for _, id := range []string{c.Session, c.Merger} {
			if id == "" {
				continue
			}
			if s, err := d.Sessions.Get(ctx, id); err == nil {
				total += s.Cost
			}
		}
	}
	return total
}

func (d *Driver) budgetReached(ctx context.Context, parent, ticket string, spent, budget float64) {
	msg := fmt.Sprintf("%.2f / %.2f USD", spent, budget)
	_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) {
		f.SpentUSD = spent
		if c, ok := f.Child(ticket); ok && !c.State.Done() {
			c.State, c.Error = domain.ChildBudget, i18n.Tf("cmd.fanout.budget_reached", msg)
		}
		f.AddEvent(domain.FanoutEventBudget, ticket, "", msg)
	})
	d.logf("fan-out %s: %s: global budget reached (%s)", parent, ticket, msg)
}

// launch creates the branch and the worktree of the child of ticket from
// the integration head, launches it and follows it.
func (d *Driver) launch(ctx context.Context, parent string, wf *sessionspec.WorkflowRuntime, ticket string) error {
	p, err := d.Sessions.Get(ctx, parent)
	if err != nil {
		return err
	}
	f := p.Fanout
	branch := ChildBranch(parent, ticket)
	d.mergeMu.Lock()
	from := f.Integration.Head
	path, err := AddWorktree(ctx, p.LaunchPath, branch, from)
	d.mergeMu.Unlock()
	if err != nil {
		d.childFailed(ctx, parent, ticket, err.Error())
		return err
	}
	remaining := 0.0
	if f.BudgetUSD > 0 {
		remaining = max(f.BudgetUSD-d.spent(ctx, p), 0.01)
	}
	rec := &domain.SessionFanout{Role: domain.RoleChild, Parent: parent, Ticket: ticket, Branch: branch}
	// Listed as running before the child exists: its delegated checkpoint
	// is checked against this record (CP-R3).
	if _, err := d.update(ctx, parent, func(f *domain.SessionFanout) {
		if c, ok := f.Child(ticket); ok {
			c.State, c.Branch, c.Path, c.Head = domain.ChildRunning, branch, path, from
		}
	}); err != nil {
		return err
	}
	sid, err := d.Launcher.Start(ctx, LaunchRequest{Parent: p, Workflow: wf.Fanout.Child, Dir: path, Ticket: ticket, Record: rec, BudgetUSD: remaining,
		Inputs: f.ChildInputs})
	if err != nil {
		d.childFailed(ctx, parent, ticket, err.Error())
		return err
	}
	_, err = d.update(ctx, parent, func(f *domain.SessionFanout) {
		if c, ok := f.Child(ticket); ok {
			c.Session = sid
		}
		f.AddEvent(domain.FanoutEventChild, ticket, sid, branch)
	})
	if err != nil {
		return err
	}
	d.logf("fan-out %s: %s → session %s on %s", parent, ticket, sid, branch)
	c, _ := d.childOf(ctx, parent, ticket)
	d.follow(ctx, parent, wf, c)
	return nil
}

func (d *Driver) childOf(ctx context.Context, parent, ticket string) (domain.FanoutChild, bool) {
	p, err := d.Sessions.Get(ctx, parent)
	if err != nil || p.Fanout == nil {
		return domain.FanoutChild{}, false
	}
	c, ok := p.Fanout.Child(ticket)
	if !ok {
		return domain.FanoutChild{}, false
	}
	return *c, true
}

func (d *Driver) childFailed(ctx context.Context, parent, ticket, why string) {
	_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) {
		if c, ok := f.Child(ticket); ok {
			c.State, c.Error = domain.ChildFailed, why
		}
		f.AddEvent(domain.FanoutEventFailed, ticket, "", why)
	})
}

// follow waits for the child to finish (stopping it on the global budget),
// stops it and merges its branch.
func (d *Driver) follow(ctx context.Context, parent string, wf *sessionspec.WorkflowRuntime, c domain.FanoutChild) {
	t := time.NewTicker(d.poll())
	defer t.Stop()
	for {
		ok, err := d.Launcher.Done(ctx, c.Session)
		if err == nil && ok {
			break
		}
		if p, err := d.Sessions.Get(ctx, parent); err == nil && p.Fanout.BudgetUSD > 0 {
			if spent := d.spent(ctx, p); spent > p.Fanout.BudgetUSD {
				_ = d.Launcher.Stop(ctx, c.Session)
				d.budgetReached(ctx, parent, c.Ticket, spent, p.Fanout.BudgetUSD)
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
	_ = d.Launcher.Stop(ctx, c.Session)
	cost := 0.0
	if s, err := d.Sessions.Get(ctx, c.Session); err == nil {
		cost = s.Cost
	}
	_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) {
		if ch, ok := f.Child(c.Ticket); ok {
			ch.CostUSD = cost
		}
		f.AddEvent(domain.FanoutEventChildDone, c.Ticket, c.Session, fmt.Sprintf("%.3f USD", cost))
	})
	d.merge(ctx, parent, wf, c)
}

// merge merges the branch of child c into the integration branch, with the
// merger on a conflict.
func (d *Driver) merge(ctx context.Context, parent string, wf *sessionspec.WorkflowRuntime, c domain.FanoutChild) {
	d.mergeMu.Lock()
	defer d.mergeMu.Unlock()
	p, err := d.Sessions.Get(ctx, parent)
	if err != nil {
		return
	}
	ig := p.Fanout.Integration
	if dirty, err := Dirty(ctx, c.Path); err == nil && dirty {
		d.childFailed(ctx, parent, c.Ticket, i18n.T("cmd.fanout.child_dirty"))
		return
	}
	n, err := CommitsAhead(ctx, ig.Path, ig.Head, c.Branch)
	if err != nil || n == 0 {
		d.childFailed(ctx, parent, c.Ticket, i18n.T("cmd.fanout.child_no_commit"))
		return
	}
	head, err := Head(ctx, ig.Path)
	if err != nil || head != ig.Head {
		d.tampered(ctx, parent, c.Ticket, ig, head)
		return
	}
	childHead, _ := BranchHead(ctx, ig.Path, c.Branch)
	conflicts, err := Merge(ctx, ig.Path, c.Branch, fmt.Sprintf("Merge %s (%s, fan-out %s)", c.Branch, c.Ticket, parent))
	switch {
	case err == nil:
		d.merged(ctx, parent, c.Ticket, childHead, ig.Path, "")
		return
	case !errors.Is(err, ErrConflict):
		AbortMerge(ctx, ig.Path)
		d.childFailed(ctx, parent, c.Ticket, err.Error())
		return
	}
	_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) {
		if ch, ok := f.Child(c.Ticket); ok {
			ch.Conflicts = conflicts
		}
		f.AddEvent(domain.FanoutEventConflict, c.Ticket, "", strings.Join(conflicts, ", "))
	})
	d.logf("fan-out %s: %s: conflict in %s", parent, c.Ticket, strings.Join(conflicts, ", "))
	if wf.Fanout.Merger == "" || !d.resolve(ctx, p, wf, c, ig, childHead, conflicts) {
		_ = ResetHard(ctx, ig.Path, ig.Head)
		_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) {
			if ch, ok := f.Child(c.Ticket); ok {
				ch.State, ch.Error = domain.ChildConflict, i18n.Tf("cmd.fanout.conflict_unresolved", strings.Join(conflicts, ", "))
			}
		})
		return
	}
	d.merged(ctx, parent, c.Ticket, childHead, ig.Path, domain.FanoutEventResolved)
}

// resolve launches the merger in the integration worktree on the merge in
// progress, waits for it and checks its result: the merge committed, both
// sides in the history, no marker left.
func (d *Driver) resolve(ctx context.Context, p *domain.Session, wf *sessionspec.WorkflowRuntime, c domain.FanoutChild, ig *domain.FanoutIntegration, childHead string, conflicts []string) bool {
	remaining := 0.0
	if p.Fanout.BudgetUSD > 0 {
		spent := d.spent(ctx, p)
		if spent >= p.Fanout.BudgetUSD {
			d.budgetReached(ctx, p.ID, "", spent, p.Fanout.BudgetUSD)
			return false
		}
		remaining = p.Fanout.BudgetUSD - spent
	}
	rec := &domain.SessionFanout{Role: domain.RoleMerger, Parent: p.ID, Ticket: c.Ticket, Branch: c.Branch}
	sid, err := d.Launcher.Start(ctx, LaunchRequest{Parent: p, Workflow: wf.Fanout.Merger, Dir: ig.Path, Ticket: c.Ticket, Record: rec, BudgetUSD: remaining,
		Inputs: map[string]string{"branch": c.Branch, "into": ig.Branch, "files": strings.Join(conflicts, "\n")}})
	if err != nil {
		d.logf("fan-out %s: merger not launched: %v", p.ID, err)
		return false
	}
	_, _ = d.update(ctx, p.ID, func(f *domain.SessionFanout) {
		if ch, ok := f.Child(c.Ticket); ok {
			ch.Merger = sid
		}
		f.AddEvent(domain.FanoutEventMerger, c.Ticket, sid, strings.Join(conflicts, ", "))
	})
	t := time.NewTicker(d.poll())
	defer t.Stop()
	for {
		if ok, err := d.Launcher.Done(ctx, sid); err == nil && ok {
			break
		}
		select {
		case <-ctx.Done():
			return false
		case <-t.C:
		}
	}
	_ = d.Launcher.Stop(ctx, sid)
	head, err := Head(ctx, ig.Path)
	if err != nil || MergeInProgress(ctx, ig.Path) || head == ig.Head || Parents(ctx, ig.Path, head) < 2 ||
		!IsAncestor(ctx, ig.Path, ig.Head, head) || !IsAncestor(ctx, ig.Path, childHead, head) || HasMarkers(ig.Path, conflicts) {
		return false
	}
	if dirty, err := Dirty(ctx, ig.Path); err != nil || dirty {
		return false
	}
	return true
}

// merged records the merge of a child (resolved: by the merger).
func (d *Driver) merged(ctx context.Context, parent, ticket, childHead, dir, kind string) {
	head, err := Head(ctx, dir)
	if err != nil {
		return
	}
	if kind == "" {
		kind = domain.FanoutEventMerged
	}
	_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) {
		f.Integration.Head = head
		if c, ok := f.Child(ticket); ok {
			c.State, c.Head, c.Merge = domain.ChildMerged, childHead, head
		}
		f.AddEvent(kind, ticket, "", short(head))
	})
	d.logf("fan-out %s: %s merged (%s)", parent, ticket, short(head))
}

// tampered stops the fan-out: the integration branch moved outside oh.
func (d *Driver) tampered(ctx context.Context, parent, ticket string, ig *domain.FanoutIntegration, head string) {
	msg := i18n.Tf("cmd.fanout.integration_moved", ig.Branch, short(ig.Head), short(head))
	_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) {
		f.Status = domain.FanoutFailed
		if c, ok := f.Child(ticket); ok {
			c.State, c.Error = domain.ChildFailed, msg
		}
		f.AddEvent(domain.FanoutEventTampered, "", "", msg)
	})
	d.logf("fan-out %s: %s", parent, msg)
}

// finish marks the fan-out ready and tells the parent.
func (d *Driver) finish(ctx context.Context, parent string) error {
	p, err := d.Sessions.Get(ctx, parent)
	if err != nil {
		return err
	}
	spent := d.spent(ctx, p)
	f, err := d.update(ctx, parent, func(f *domain.SessionFanout) {
		if f.Status == domain.FanoutRunning {
			f.Status = domain.FanoutReady
			f.AddEvent(domain.FanoutEventReady, "", "", "")
		}
		f.SpentUSD = spent
	})
	if err != nil {
		return err
	}
	if f.Status == domain.FanoutStopped {
		return nil // the parent ended: nobody to tell
	}
	text := Summary(f)
	if err := d.Launcher.Tell(ctx, parent, text); err != nil {
		return fmt.Errorf("telling the parent: %w", err)
	}
	_, _ = d.update(ctx, parent, func(f *domain.SessionFanout) { f.AddEvent(domain.FanoutEventResumed, "", parent, "") })
	return nil
}

// Summary is the message given to the parent once the children are done:
// the state of each child and what to do next.
func Summary(f *domain.SessionFanout) string {
	var b strings.Builder
	if f.Status == domain.FanoutFailed {
		b.WriteString(i18n.T("cmd.fanout.summary_failed"))
	} else {
		b.WriteString(i18n.T("cmd.fanout.summary_head"))
	}
	if f.Integration != nil {
		b.WriteString("\n" + i18n.Tf("cmd.fanout.summary_branch", f.Integration.Branch, short(f.Integration.Head), short(f.Integration.Base)))
	}
	b.WriteString("\n\n| Ticket | État | Session | Merge | Détail |\n|---|---|---|---|---|\n")
	for _, c := range f.Children {
		detail := c.Error
		if detail == "" && len(c.Conflicts) > 0 {
			detail = i18n.Tf("cmd.fanout.summary_resolved", strings.Join(c.Conflicts, ", "))
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", c.Ticket, c.State, c.Session, short(c.Merge), strings.ReplaceAll(detail, "|", "/"))
	}
	if f.BudgetUSD > 0 {
		b.WriteString("\n" + i18n.Tf("cmd.fanout.summary_budget", f.SpentUSD, f.BudgetUSD))
	}
	if f.Status != domain.FanoutFailed {
		b.WriteString("\n\n" + i18n.T("cmd.fanout.summary_next"))
	}
	return b.String()
}
