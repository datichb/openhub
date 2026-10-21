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
	// Task is the task of a child of a fan-out `source: tasks` (Ticket is
	// then its id): the child gets no ticket.
	Task *domain.PlanTask
	// Inputs are inputs of the workflow (merger: the branch merged and the
	// files in conflict; child: the inputs of the parent and its task).
	Inputs map[string]string
	// Record is the fan-out record of the new session.
	Record *domain.SessionFanout
	// BudgetUSD is the share of the global budget granted to the session
	// (a child, a merger or a fixer: its share of what remains, see
	// childShare and auxShare; 0 = no cap).
	BudgetUSD float64
}

// Launcher starts and follows the sessions of a fan-out.
type Launcher interface {
	// Start launches a session without interface and returns its id.
	Start(ctx context.Context, req LaunchRequest) (string, error)
	// Status reports where a session is: ended, or its turn over with no
	// decision waiting (the driver decides whether it finished its work).
	Status(ctx context.Context, sessionID string) (SessionStatus, error)
	// Stop stops a session.
	Stop(ctx context.Context, sessionID string) error
	// Tell sends a message to a session (the parent, once the children are
	// done).
	Tell(ctx context.Context, sessionID, text string) error
}

// SessionStatus is what the driver reads of a session it launched.
type SessionStatus struct {
	// Ended: completed, stopped or failed.
	Ended bool
	// Idle: its turn is over and no decision waits.
	Idle bool
}

// Why a session of the fan-out counts as finished (event child_done).
const (
	// DoneEnded: the session ended (completed, stopped, failed).
	DoneEnded = "ended"
	// DoneSignal: its end of work is seen by oh (child: the checkpoints
	// that unlock its commit passed, a commit on its branch, a clean
	// worktree; merger: the merge concluded), its turn over.
	DoneSignal = "signal"
	// DoneIdle: idle without that signal for IdleTimeout (guard).
	DoneIdle = "idle"
)

// DefaultIdleTimeout is how long a session of the fan-out may stay idle,
// no decision waiting, without its end of work, before oh stops it.
const DefaultIdleTimeout = 10 * time.Minute

// DefaultMergerIdleTimeout is the guard of a merger (p6-debts): it runs in
// auto, nobody answers it, and its work is one merge; idle without the
// merge concluded, it gave up.
const DefaultMergerIdleTimeout = 2 * time.Minute

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
	// IdleTimeout is the guard on an idle session without its end of work
	// (0 = DefaultIdleTimeout).
	IdleTimeout time.Duration
	// MergerIdleTimeout is the guard on an idle merger without the merge
	// concluded (0 = DefaultMergerIdleTimeout, IdleTimeout when shorter).
	MergerIdleTimeout time.Duration
	// Log receives a line per step (nil = none).
	Log func(format string, args ...any)

	mu      sync.Mutex // the record of the parent
	mergeMu sync.Mutex // the integration worktree
}

// Errors of the driver.
var (
	ErrNotFanout = errors.New("this session is not the parent of a fan-out")
	ErrDisabled  = errors.New("the fan-out is experimental and disabled in this configuration")

	// errRecord wraps what stops the driver because the record of a state
	// already taken (merged in git, a merger started) could not be written:
	// going on would build on a false state; started again, the driver
	// reads the record again.
	errRecord = errors.New("the record of the fan-out could not be written")
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

// recordTimeout bounds the write of the record of the parent once ctx
// ended (see update).
const recordTimeout = 5 * time.Second

// update changes the record of the parent (read again first). It reads and
// writes with a context detached from ctx (bounded by recordTimeout): a
// state already taken in git, a merge for instance, is recorded even when
// the driver is being stopped. With ctx the read would fail too, and
// nothing would ever be written.
func (d *Driver) update(ctx context.Context, parent string, fn func(*domain.SessionFanout)) (*domain.SessionFanout, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
	defer cancel()
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

// record is update for the writes whose failure does not stop the driver:
// it is written in the journal, never dropped.
func (d *Driver) record(ctx context.Context, parent string, fn func(*domain.SessionFanout)) error {
	_, err := d.update(ctx, parent, fn)
	if err != nil {
		d.logf("fan-out %s: record not written: %v", parent, err)
	}
	return err
}

// Run drives the fan-out of parent until every child is done (or ctx
// ends): it waits for the checkpoints that come before the children,
// creates the integration branch, then launches, follows and merges the
// children, and tells the parent the result.
func (d *Driver) Run(ctx context.Context, parent string) error {
	return d.run(ctx, parent)
}

// Who started the driver again (Relaunched).
const (
	RelaunchByDaemon = "daemon"
	RelaunchByUser   = "user"
)

// Relaunched records that the driver was started again by the daemon
// (p6-debts) or by the user (from the TUI), before Run. An origin other
// than RelaunchByUser is the daemon's.
func (d *Driver) Relaunched(ctx context.Context, parent, by string) error {
	if by != RelaunchByUser {
		by = RelaunchByDaemon
	}
	_, err := d.update(ctx, parent, func(f *domain.SessionFanout) {
		f.AddEvent(domain.FanoutEventDriver, "", "", "started again by the "+by+" ("+string(f.Status)+")")
	})
	return err
}

func (d *Driver) run(ctx context.Context, parent string) error {
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
	switch p.Fanout.Status {
	case domain.FanoutMerged, domain.FanoutFailed, domain.FanoutStopped:
		return nil
	case domain.FanoutReady, domain.FanoutFixing:
		// Started again once the children are done: the fix rounds.
		wf, _, err := d.Workflow(ctx, parent)
		if err != nil {
			return err
		}
		return d.await(ctx, parent, wf)
	}
	wf, st, err := d.waitStart(ctx, p)
	if err != nil || wf == nil {
		return err
	}
	if p, err = d.Sessions.Get(ctx, parent); err != nil {
		return err
	}
	if p.Fanout.Integration == nil {
		if err := d.startIntegration(ctx, p, wf, st); err != nil {
			_ = d.record(ctx, parent, func(f *domain.SessionFanout) {
				f.Status = domain.FanoutFailed
				f.AddEvent(domain.FanoutEventFailed, "", "", err.Error())
			})
			return err
		}
	}
	if err := d.runChildren(ctx, parent, wf); err != nil {
		return err
	}
	if p, err := d.Sessions.Get(ctx, parent); err == nil && p.State == domain.RunStopped {
		return d.abandonDone(ctx, p)
	}
	if err := d.finish(ctx, parent); err != nil {
		return err
	}
	return d.await(ctx, parent, wf)
}

// abandonDone stops what still runs of the fan-out of a parent stopped
// for good and cleans it up: what can go goes, the work merged so far
// stays on the integration branch.
func (d *Driver) abandonDone(ctx context.Context, p *domain.Session) error {
	if err := d.abandon(ctx, p); err != nil {
		return err
	}
	if cur, err := d.Sessions.Get(ctx, p.ID); err == nil {
		p = cur
	}
	if res, err := Clean(ctx, d.Sessions, p, CleanOptions{}); err != nil {
		d.logf("fan-out %s: clean-up: %v", p.ID, err)
	} else {
		d.logf("fan-out %s: abandoned, %d worktree(s) and %d branch(es) removed, %d kept", p.ID, len(res.Worktrees), len(res.Branches), len(res.Kept))
	}
	return nil
}

// abandon stops the fan-out of a parent stopped for good: no child starts
// any more and the running ones are stopped. The error is the one of the
// record: with the fan-out still running in it, the loop of runChildren
// would start again.
func (d *Driver) abandon(ctx context.Context, p *domain.Session) error {
	for _, id := range Sessions(p.Fanout) {
		if s, err := d.Sessions.Get(ctx, id); err == nil && !terminal(s.State) {
			_ = d.Launcher.Stop(ctx, id)
		}
	}
	if err := d.record(ctx, p.ID, func(f *domain.SessionFanout) {
		if x, ok := f.LastFix(); ok && (x.State == domain.ChildPending || x.State == domain.ChildRunning) {
			x.State, x.Error = domain.ChildFailed, i18n.T("cmd.fanout.parent_stopped")
		}
		if f.Status == domain.FanoutRunning || f.Status == domain.FanoutFixing {
			f.Status = domain.FanoutStopped
			f.AddEvent(domain.FanoutEventFailed, "", p.ID, "parent stopped")
		}
		for i := range f.Children {
			if c := &f.Children[i]; c.State == domain.ChildPending || c.State == domain.ChildRunning {
				c.State, c.Error = domain.ChildFailed, i18n.T("cmd.fanout.parent_stopped")
			}
		}
	}); err != nil {
		return fmt.Errorf("%w: %w", errRecord, err)
	}
	d.logf("fan-out %s: the parent was stopped, children stopped", p.ID)
	return nil
}

// waitStart waits until the parent passed every mandatory checkpoint
// declared before its merge checkpoint (cp-plan), and returns its workflow
// and checkpoint state then (the tasks recorded at cp-plan). nil when the
// parent ended first (the fan-out is stopped).
func (d *Driver) waitStart(ctx context.Context, p *domain.Session) (*sessionspec.WorkflowRuntime, domain.CheckpointState, error) {
	t := time.NewTicker(d.poll())
	defer t.Stop()
	for {
		wf, st, err := d.Workflow(ctx, p.ID)
		if err != nil {
			return nil, st, err
		}
		if wf == nil || wf.Fanout == nil {
			return nil, st, ErrNotFanout
		}
		if gatesPassed(wf, st) {
			return wf, st, nil
		}
		cur, err := d.Sessions.Get(ctx, p.ID)
		if err != nil {
			return nil, st, err
		}
		if cur.State == domain.RunStopped || cur.State == domain.RunFailed {
			_, err := d.update(ctx, p.ID, func(f *domain.SessionFanout) { f.Status = domain.FanoutStopped })
			return nil, st, err
		}
		if cur.State == domain.RunCompleted {
			// p6-debts: the parent ended its step before the checkpoints
			// that start the children; the fan-out waits, the daemon starts
			// the driver again when the parent goes on.
			return nil, st, nil
		}
		select {
		case <-ctx.Done():
			return nil, st, ctx.Err()
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

// Sessions are the sessions of the fan-out of a parent: children,
// mergers and fixers.
func Sessions(f *domain.SessionFanout) []string {
	if f == nil {
		return nil
	}
	var out []string
	for _, c := range f.Children {
		for _, id := range []string{c.Session, c.Merger} {
			if id != "" {
				out = append(out, id)
			}
		}
	}
	for _, x := range f.Fixes {
		if x.Session != "" {
			out = append(out, x.Session)
		}
	}
	return out
}

func terminal(s domain.RunState) bool {
	return s == domain.RunCompleted || s == domain.RunFailed || s == domain.RunStopped
}

// startIntegration creates the integration branch from the HEAD of the
// parent, and the children of its tickets, or of the tasks oh recorded at
// its tasks checkpoint (source: tasks; never what an agent declares).
func (d *Driver) startIntegration(ctx context.Context, p *domain.Session, wf *sessionspec.WorkflowRuntime, st domain.CheckpointState) error {
	var children []domain.FanoutChild
	if wf.Fanout.Tasks() {
		if len(st.Tasks) == 0 {
			return errors.New(i18n.Tf("cmd.fanout.no_tasks", wf.TasksCheckpoint()))
		}
		for _, t := range st.Tasks {
			task := t
			children = append(children, domain.FanoutChild{Ticket: t.ID, Task: &task, State: domain.ChildPending})
		}
	} else {
		for _, t := range p.Tickets {
			children = append(children, domain.FanoutChild{Ticket: t, State: domain.ChildPending})
		}
	}
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
			f.Children = children
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
	// fatal is what stops the driver: a state taken (merged, ...) that could
	// not be recorded. The followers end with ctx.
	var fatal error
	setFatal := func(err error) {
		runMu.Lock()
		defer runMu.Unlock()
		if fatal == nil {
			fatal = err
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for {
		runMu.Lock()
		stop := fatal
		runMu.Unlock()
		if stop != nil {
			cancel()
			wg.Wait()
			return stop
		}
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
		if p.State == domain.RunStopped && f.Status == domain.FanoutRunning {
			if err := d.abandon(ctx, p); err != nil { // the parent was stopped for good
				cancel()
				wg.Wait()
				return err
			}
			continue
		}
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
			if c.State == domain.ChildRunning && c.Session == "" {
				runMu.Lock()
				launching := running[c.Ticket]
				runMu.Unlock()
				if !launching {
					// A driver stopped between the reservation of the share
					// of the child and the start of its session: it starts
					// again, with a new share.
					_ = d.record(ctx, parent, func(f *domain.SessionFanout) {
						if ch, ok := f.Child(c.Ticket); ok && ch.State == domain.ChildRunning && ch.Session == "" {
							ch.State, ch.BudgetUSD = domain.ChildPending, 0
						}
					})
				}
			}
			if c.State == domain.ChildRunning && c.Session != "" {
				runMu.Lock()
				resumed := !running[c.Ticket]
				runMu.Unlock()
				if resumed {
					c, resumed = d.resumable(ctx, parent, c)
				}
				if resumed {
					runMu.Lock()
					running[c.Ticket] = true // a previous driver launched it
					runMu.Unlock()
					n++
					wg.Add(1)
					go func(c domain.FanoutChild) {
						defer wg.Done()
						err := d.follow(ctx, parent, wf, c)
						if err != nil {
							d.logf("fan-out %s: %s: %v", parent, c.Ticket, err)
							setFatal(err)
						}
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
				_ = d.record(ctx, parent, func(f *domain.SessionFanout) {
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
			// The share of the global budget is reserved here, one child
			// after the other, before the child starts (F-04).
			share, ok, err := d.reserveShare(ctx, parent, c.Ticket)
			if err != nil {
				d.logf("fan-out %s: %s: %v", parent, c.Ticket, err)
				continue
			}
			if !ok {
				continue // what is left is held by the children running: wait for one of them
			}
			runMu.Lock()
			running[c.Ticket] = true
			runMu.Unlock()
			n++
			wg.Add(1)
			go func(ticket string) {
				defer wg.Done()
				if err := d.launch(ctx, parent, wf, ticket, share); err != nil {
					d.logf("fan-out %s: %s: %v", parent, ticket, err)
					if errors.Is(err, errRecord) {
						setFatal(err)
					}
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

// resumable reads again child c, seen running in a record read before:
// its follower may have ended since (merged, failed) and is then gone
// from the running ones; following it again would merge it a second
// time (p8-fixes: a merged child marked failed, « no commit »). The
// follower writes the record before it leaves the running ones.
func (d *Driver) resumable(ctx context.Context, parent string, c domain.FanoutChild) (domain.FanoutChild, bool) {
	cur, ok := d.childOf(ctx, parent, c.Ticket)
	if !ok || cur.State != domain.ChildRunning || cur.Session == "" {
		return c, false
	}
	return cur, true
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
		if c.Task != nil {
			continue // a task: no ticket, independent of the others
		}
		list, closed, err := get(ctx, p.LaunchPath, c.Ticket)
		if err != nil {
			continue
		}
		if closed && c.State == domain.ChildPending {
			_ = d.record(ctx, p.ID, func(f *domain.SessionFanout) {
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

func bdTicket(_ context.Context, dir, ticket string) (deps []string, closed bool, err error) {
	t, err := beads.Show(dir, ticket)
	if err != nil {
		return nil, false, err
	}
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
	for _, id := range Sessions(p.Fanout) {
		if s, err := d.Sessions.Get(ctx, id); err == nil {
			total += s.Cost
		}
	}
	return total
}

func (d *Driver) budgetReached(ctx context.Context, parent, ticket string, spent, budget float64) {
	msg := fmt.Sprintf("%.2f / %.2f USD", spent, budget)
	_ = d.record(ctx, parent, func(f *domain.SessionFanout) {
		f.SpentUSD = spent
		if c, ok := f.Child(ticket); ok && !c.State.Done() {
			c.State, c.Error = domain.ChildBudget, i18n.Tf("cmd.fanout.budget_reached", msg)
		}
		f.AddEvent(domain.FanoutEventBudget, ticket, "", msg)
	})
	d.logf("fan-out %s: %s: global budget reached (%s)", parent, ticket, msg)
}

// budgetHeld records that the merger of a conflict was not launched: the
// budget that is left is held by the children still running.
func (d *Driver) budgetHeld(ctx context.Context, parent, ticket string, spent, budget float64) {
	msg := i18n.Tf("cmd.fanout.budget_held", fmt.Sprintf("%.2f / %.2f USD", spent, budget))
	_ = d.record(ctx, parent, func(f *domain.SessionFanout) {
		f.SpentUSD = spent
		f.AddEvent(domain.FanoutEventBudget, ticket, "", msg)
	})
	d.logf("fan-out %s: %s: %s", parent, ticket, msg)
}

// releaseMergerShare gives back the share reserved for a merger that did not
// start.
func (d *Driver) releaseMergerShare(ctx context.Context, parent, ticket string) {
	_ = d.record(ctx, parent, func(f *domain.SessionFanout) {
		if c, ok := f.Child(ticket); ok {
			c.MergerBudgetUSD = 0
		}
	})
}

// minChildBudget is the smallest share of the global budget worth starting
// a child with (and the floor of the cap of a session).
const minChildBudget = 0.01

// childShare is the part of the global budget a child about to start may
// spend (audit v5, F-04 follow-up): what no one has a claim on, split among
// the children that have not started, this one included.
//
//   - free is the budget minus what was spent and minus what the running
//     children may still spend (the unspent part of their share): the sum of
//     the shares of the children at the same time never exceeds the budget,
//     where each used to get all that remained (N children could spend N
//     times the budget).
//   - dividing among the children left, not among the slots, keeps a share
//     for those that come after: a child started first cannot starve the
//     others, and the one that comes last has what the others left.
//   - nothing is stored for a child that ends: its unspent share is free
//     again at the next start, so the children after it get more.
//
// ok is false when the share is below minChildBudget while children run:
// the child waits for one of them to end (and release its claim).
func childShare(budget, spent, claims float64, pending, running int) (share float64, ok bool) {
	free := budget - spent - claims
	share = free / float64(max(pending, 1))
	if share >= minChildBudget {
		return share, true
	}
	if running > 0 {
		return 0, false
	}
	return minChildBudget, true
}

// reserveShare grants the child of ticket its share of the global budget and
// marks it running, in one update of the record (so that the children start
// one after the other, each seeing the claims of the previous ones). With no
// global budget the share is 0 (no cap). ok is false when the child must wait.
func (d *Driver) reserveShare(ctx context.Context, parent, ticket string) (share float64, ok bool, err error) {
	ok = true
	_, err = d.update(ctx, parent, func(f *domain.SessionFanout) {
		c, found := f.Child(ticket)
		if !found || c.State != domain.ChildPending {
			ok = false
			return
		}
		if f.BudgetUSD > 0 {
			var claims float64
			pending, running := 0, 0
			for i := range f.Children {
				switch ch := &f.Children[i]; ch.State {
				case domain.ChildPending:
					pending++
				case domain.ChildRunning:
					running++
					claims += d.claim(ctx, ch)
				}
			}
			if share, ok = childShare(f.BudgetUSD, d.spentOf(ctx, parent, f), claims, pending, running); !ok {
				return
			}
			c.BudgetUSD = share
		}
		c.State = domain.ChildRunning
	})
	return share, ok, err
}

// claim is what a running child, and the merger of its conflict, may still
// spend of the shares they were granted.
func (d *Driver) claim(ctx context.Context, ch *domain.FanoutChild) float64 {
	return max(ch.BudgetUSD-d.costOf(ctx, ch.Session), 0) + max(ch.MergerBudgetUSD-d.costOf(ctx, ch.Merger), 0)
}

// auxShare is the cap of a merger or of a fixer: the rule of the children
// (childShare), with the session counted as one more child to start. So it
// does not take "all that remains": it leaves their part to the children
// not started, and what the children still running may spend (their claim,
// and the one of a merger already running) is kept apart. merging is the
// child whose conflict the merger resolves: it has stopped, it holds
// nothing. ok is false when what is left is held by running children: the
// merger cannot wait for them (they need the integration worktree it holds
// to end), it is not launched.
func (d *Driver) auxShare(ctx context.Context, parent string, f *domain.SessionFanout, merging string) (share float64, ok bool) {
	var claims float64
	pending, running := 0, 0
	for i := range f.Children {
		switch ch := &f.Children[i]; {
		case ch.Ticket == merging:
		case ch.State == domain.ChildPending:
			pending++
		case ch.State == domain.ChildRunning:
			running++
			claims += d.claim(ctx, ch)
		}
	}
	return childShare(f.BudgetUSD, d.spentOf(ctx, parent, f), claims, pending+1, running)
}

// reserveMergerShare grants the merger of the conflict of ticket its share of
// the global budget, in one update of the record (like reserveShare: the
// children that start meanwhile see its claim). With no global budget the
// share is 0 (no cap). ok is false when the merger must not be launched.
func (d *Driver) reserveMergerShare(ctx context.Context, parent, ticket string) (share float64, ok bool, err error) {
	ok = true
	_, err = d.update(ctx, parent, func(f *domain.SessionFanout) {
		c, found := f.Child(ticket)
		if !found {
			ok = false
			return
		}
		if f.BudgetUSD > 0 {
			if share, ok = d.auxShare(ctx, parent, f, ticket); !ok {
				return
			}
			c.MergerBudgetUSD = share
		}
	})
	return share, ok, err
}

// costOf is the cost of a session ("" or unknown = 0).
func (d *Driver) costOf(ctx context.Context, id string) float64 {
	if id == "" {
		return 0
	}
	if s, err := d.Sessions.Get(ctx, id); err == nil {
		return s.Cost
	}
	return 0
}

// spentOf is what the parent and the sessions of the record f spent.
func (d *Driver) spentOf(ctx context.Context, parent string, f *domain.SessionFanout) float64 {
	total := d.costOf(ctx, parent)
	for _, id := range Sessions(f) {
		total += d.costOf(ctx, id)
	}
	return total
}

// launch creates the branch and the worktree of the child of ticket from
// the integration head (the child is already marked running, with its share
// of the budget: reserveShare), launches it and follows it.
func (d *Driver) launch(ctx context.Context, parent string, wf *sessionspec.WorkflowRuntime, ticket string, share float64) error {
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
	child, _ := f.Child(ticket)
	var task *domain.PlanTask
	if child != nil && child.Task != nil {
		t := *child.Task
		task = &t
	}
	rec := &domain.SessionFanout{Role: domain.RoleChild, Parent: parent, Ticket: ticket, Branch: branch, Task: task}
	// Listed as running before the child exists: its delegated checkpoint
	// is checked against this record (CP-R3).
	if _, err := d.update(ctx, parent, func(f *domain.SessionFanout) {
		if c, ok := f.Child(ticket); ok {
			c.State, c.Branch, c.Path, c.Head = domain.ChildRunning, branch, path, from
		}
	}); err != nil {
		return err
	}
	sid, err := d.Launcher.Start(ctx, LaunchRequest{Parent: p, Workflow: wf.Fanout.Child, Dir: path, Ticket: ticket, Task: task, Record: rec, BudgetUSD: share,
		Inputs: childInputs(f.ChildInputs, task)})
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
		// Not in the record: nobody would follow the child, stop it (the
		// context may be the one that ended, hence a detached one).
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
		defer cancel()
		_ = d.Launcher.Stop(sctx, sid)
		return fmt.Errorf("%w: %w", errRecord, err)
	}
	d.logf("fan-out %s: %s → session %s on %s", parent, ticket, sid, branch)
	c, _ := d.childOf(ctx, parent, ticket)
	return d.follow(ctx, parent, wf, c)
}

// childInputs are the inputs of a child: those of the parent (fanout.inputs)
// and, for a task, the task as oh recorded it (task, files, domain).
func childInputs(parent map[string]string, task *domain.PlanTask) map[string]string {
	if task == nil {
		return parent
	}
	out := make(map[string]string, len(parent)+3)
	for k, v := range parent {
		out[k] = v
	}
	text := task.Title
	if task.Description != "" {
		text += "\n\n" + task.Description
	}
	out["task"], out["files"] = text, strings.Join(task.Files, "\n")
	if task.Domain != "" {
		out["domain"] = task.Domain
	}
	return out
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
	_ = d.record(ctx, parent, func(f *domain.SessionFanout) {
		if c, ok := f.Child(ticket); ok {
			c.State, c.Error = domain.ChildFailed, why
		}
		f.AddEvent(domain.FanoutEventFailed, ticket, "", why)
	})
}

// follow waits for the child to finish (stopping it on the global budget),
// stops it and merges its branch. The error is the one of a record that
// could not be written after a merge (see merge).
func (d *Driver) follow(ctx context.Context, parent string, wf *sessionspec.WorkflowRuntime, c domain.FanoutChild) error {
	why, ok := d.wait(ctx, c.Session, func() bool { return d.childFinished(ctx, c) }, func() bool {
		if p, err := d.Sessions.Get(ctx, parent); err == nil && p.Fanout.BudgetUSD > 0 {
			if spent := d.spent(ctx, p); spent > p.Fanout.BudgetUSD {
				_ = d.Launcher.Stop(ctx, c.Session)
				d.budgetReached(ctx, parent, c.Ticket, spent, p.Fanout.BudgetUSD)
				return true
			}
		}
		return false
	})
	if !ok {
		return nil
	}
	_ = d.Launcher.Stop(ctx, c.Session)
	if p, err := d.Sessions.Get(ctx, parent); err != nil || p.Fanout.Status != domain.FanoutRunning {
		return nil //nolint:nilerr // abandoned or failed meanwhile (or unreadable): nothing more is merged
	}
	cost := 0.0
	if s, err := d.Sessions.Get(ctx, c.Session); err == nil {
		cost = s.Cost
	}
	_ = d.record(ctx, parent, func(f *domain.SessionFanout) {
		if ch, ok := f.Child(c.Ticket); ok {
			ch.CostUSD = cost
		}
		f.AddEvent(domain.FanoutEventChildDone, c.Ticket, c.Session, fmt.Sprintf("%s, %.3f USD", why, cost))
	})
	return d.merge(ctx, parent, wf, c)
}

// wait waits until session id finished its work and tells why (DoneEnded,
// DoneSignal, DoneIdle); ok false when ctx ended or stop (checked at each
// poll, nil = none) returned true. The end signal is only believed once
// the turn of the session is over, on two checks in a row; an idle
// session without it is stopped after IdleTimeout.
func (d *Driver) wait(ctx context.Context, id string, finished, stop func() bool) (why string, ok bool) {
	timeout := d.IdleTimeout
	if timeout <= 0 {
		timeout = DefaultIdleTimeout
	}
	return d.waitIdle(ctx, id, timeout, finished, stop)
}

// mergerGuard is the idle guard of a merger.
func (d *Driver) mergerGuard() time.Duration {
	if d.MergerIdleTimeout > 0 {
		return d.MergerIdleTimeout
	}
	if d.IdleTimeout > 0 {
		return min(DefaultMergerIdleTimeout, d.IdleTimeout)
	}
	return DefaultMergerIdleTimeout
}

func (d *Driver) waitIdle(ctx context.Context, id string, timeout time.Duration, finished, stop func() bool) (why string, ok bool) {
	t := time.NewTicker(d.poll())
	defer t.Stop()
	var idleSince time.Time
	signals := 0
	for {
		st, err := d.Launcher.Status(ctx, id)
		switch {
		case err != nil:
		case st.Ended:
			return DoneEnded, true
		case !st.Idle:
			idleSince, signals = time.Time{}, 0
		default:
			if idleSince.IsZero() {
				idleSince = time.Now()
			}
			if finished() {
				signals++
				if signals >= 2 {
					return DoneSignal, true
				}
			} else {
				signals = 0
			}
			if time.Since(idleSince) >= timeout {
				d.logf("session %s idle for %s without its end of work: stopped", id, timeout)
				return DoneIdle, true
			}
		}
		if stop != nil && stop() {
			return "", false
		}
		select {
		case <-ctx.Done():
			return "", false
		case <-t.C:
		}
	}
}

// childFinished reports the end of work of child c: the checkpoints of
// its workflow that unlock the commit (its delegated cp-2) passed, at
// least one commit on its branch since it started, and nothing left
// uncommitted in its worktree.
func (d *Driver) childFinished(ctx context.Context, c domain.FanoutChild) bool {
	if c.Path == "" || c.Head == "" {
		return false
	}
	if wf, st, err := d.Workflow(ctx, c.Session); err != nil {
		return false
	} else if wf != nil {
		for _, cp := range wf.Checkpoints {
			if cp.Mandatory && slices.Contains(cp.Unlocks, sessionspec.UnlockCommit) && !st.HasPassed(cp.ID) {
				return false
			}
		}
	}
	if n, err := CommitsAhead(ctx, c.Path, c.Head, c.Branch); err != nil || n == 0 {
		return false
	}
	dirty, err := Dirty(ctx, c.Path)
	return err == nil && !dirty
}

// merge merges the branch of child c into the integration branch, with the
// merger on a conflict. The error is a record that could not be written
// once git changed (merged, conflict left): the driver stops, the record
// being behind git.
func (d *Driver) merge(ctx context.Context, parent string, wf *sessionspec.WorkflowRuntime, c domain.FanoutChild) error {
	d.mergeMu.Lock()
	defer d.mergeMu.Unlock()
	p, err := d.Sessions.Get(ctx, parent)
	if err != nil {
		return nil
	}
	ig := p.Fanout.Integration
	if dirty, err := Dirty(ctx, c.Path); err == nil && dirty {
		d.childFailed(ctx, parent, c.Ticket, i18n.T("cmd.fanout.child_dirty"))
		return nil
	}
	n, err := CommitsAhead(ctx, ig.Path, ig.Head, c.Branch)
	if err != nil || n == 0 {
		d.childFailed(ctx, parent, c.Ticket, i18n.T("cmd.fanout.child_no_commit"))
		return nil
	}
	head, err := Head(ctx, ig.Path)
	if err != nil || head != ig.Head {
		d.tampered(ctx, parent, c.Ticket, ig, head)
		return nil
	}
	childHead, _ := BranchHead(ctx, ig.Path, c.Branch)
	conflicts, err := Merge(ctx, ig.Path, c.Branch, fmt.Sprintf("Merge %s (%s, fan-out %s)", c.Branch, c.Ticket, parent))
	switch {
	case err == nil:
		return d.merged(ctx, parent, c.Ticket, childHead, ig.Path, "")
	case !errors.Is(err, ErrConflict):
		AbortMerge(ctx, ig.Path)
		d.childFailed(ctx, parent, c.Ticket, err.Error())
		return nil
	}
	_ = d.record(ctx, parent, func(f *domain.SessionFanout) {
		if ch, ok := f.Child(c.Ticket); ok {
			ch.Conflicts = conflicts
		}
		f.AddEvent(domain.FanoutEventConflict, c.Ticket, "", strings.Join(conflicts, ", "))
	})
	d.logf("fan-out %s: %s: conflict in %s", parent, c.Ticket, strings.Join(conflicts, ", "))
	resolved := false
	if wf.Fanout.Merger != "" {
		if resolved, err = d.resolve(ctx, p, wf, c, ig, childHead, conflicts); err != nil {
			return err
		}
	}
	if !resolved {
		_ = ResetHard(ctx, ig.Path, ig.Head)
		if err := d.record(ctx, parent, func(f *domain.SessionFanout) {
			if ch, ok := f.Child(c.Ticket); ok {
				ch.State, ch.Error = domain.ChildConflict, i18n.Tf("cmd.fanout.conflict_unresolved", strings.Join(conflicts, ", "))
			}
		}); err != nil {
			return fmt.Errorf("%w: %w", errRecord, err)
		}
		return nil
	}
	return d.merged(ctx, parent, c.Ticket, childHead, ig.Path, domain.FanoutEventResolved)
}

// resolve launches the merger in the integration worktree on the merge in
// progress, waits for it and checks its result: the merge committed, both
// sides in the history, no marker left. The error is the record of the
// merger that could not be written: it is stopped and the merge undone, so
// that git and the record agree again.
func (d *Driver) resolve(ctx context.Context, p *domain.Session, wf *sessionspec.WorkflowRuntime, c domain.FanoutChild, ig *domain.FanoutIntegration, childHead string, conflicts []string) (bool, error) {
	share := 0.0
	if p.Fanout.BudgetUSD > 0 {
		spent := d.spent(ctx, p)
		if spent >= p.Fanout.BudgetUSD {
			d.budgetReached(ctx, p.ID, "", spent, p.Fanout.BudgetUSD)
			return false, nil
		}
		s, ok, err := d.reserveMergerShare(ctx, p.ID, c.Ticket)
		if err != nil {
			d.logf("fan-out %s: merger not launched: %v", p.ID, err)
			return false, nil
		}
		if !ok {
			d.budgetHeld(ctx, p.ID, c.Ticket, spent, p.Fanout.BudgetUSD)
			return false, nil
		}
		share = s
	}
	rec := &domain.SessionFanout{Role: domain.RoleMerger, Parent: p.ID, Ticket: c.Ticket, Branch: c.Branch}
	sid, err := d.Launcher.Start(ctx, LaunchRequest{Parent: p, Workflow: wf.Fanout.Merger, Dir: ig.Path, Ticket: c.Ticket, Record: rec, BudgetUSD: share,
		Inputs: map[string]string{"branch": c.Branch, "into": ig.Branch, "files": strings.Join(conflicts, "\n")}})
	if err != nil {
		d.logf("fan-out %s: merger not launched: %v", p.ID, err)
		d.releaseMergerShare(ctx, p.ID, c.Ticket)
		return false, nil
	}
	if err := d.record(ctx, p.ID, func(f *domain.SessionFanout) {
		if ch, ok := f.Child(c.Ticket); ok {
			ch.Merger = sid
		}
		f.AddEvent(domain.FanoutEventMerger, c.Ticket, sid, strings.Join(conflicts, ", "))
	}); err != nil {
		_ = d.Launcher.Stop(ctx, sid)
		_ = ResetHard(ctx, ig.Path, ig.Head)
		return false, fmt.Errorf("%w: %w", errRecord, err)
	}
	why, ok := d.waitIdle(ctx, sid, d.mergerGuard(), func() bool {
		head, err := Head(ctx, ig.Path)
		return err == nil && head != ig.Head && !MergeInProgress(ctx, ig.Path)
	}, nil)
	if !ok {
		return false, nil
	}
	_ = d.Launcher.Stop(ctx, sid)
	_ = d.record(ctx, p.ID, func(f *domain.SessionFanout) {
		f.AddEvent(domain.FanoutEventChildDone, c.Ticket, sid, why)
	})
	head, err := Head(ctx, ig.Path)
	if err != nil || MergeInProgress(ctx, ig.Path) || head == ig.Head || Parents(ctx, ig.Path, head) < 2 ||
		!IsAncestor(ctx, ig.Path, ig.Head, head) || !IsAncestor(ctx, ig.Path, childHead, head) || HasMarkers(ig.Path, conflicts) {
		return false, nil //nolint:nilerr // the merger's work is refused, not an error of the driver
	}
	if dirty, err := Dirty(ctx, ig.Path); err != nil || dirty {
		return false, nil //nolint:nilerr // the merger's work is refused, not an error of the driver
	}
	return true, nil
}

// merged records the merge of a child (resolved: by the merger). Git is
// already merged: the head is read, and the record written, out of ctx,
// which may end (the driver stopped) right after the merge; a record left
// behind git would make the next merge see an integration branch moved
// outside oh. The error is a record that could not be written.
func (d *Driver) merged(ctx context.Context, parent, ticket, childHead, dir, kind string) error {
	head, err := Head(context.WithoutCancel(ctx), dir)
	if err != nil {
		d.logf("fan-out %s: %s merged, head not read: %v", parent, ticket, err)
		return fmt.Errorf("%w: %w", errRecord, err)
	}
	if kind == "" {
		kind = domain.FanoutEventMerged
	}
	if err := d.record(ctx, parent, func(f *domain.SessionFanout) {
		f.Integration.Head = head
		if c, ok := f.Child(ticket); ok {
			c.State, c.Head, c.Merge = domain.ChildMerged, childHead, head
		}
		f.AddEvent(kind, ticket, "", short(head))
	}); err != nil {
		return fmt.Errorf("%w: %w", errRecord, err)
	}
	d.logf("fan-out %s: %s merged (%s)", parent, ticket, short(head))
	return nil
}

// tampered stops the fan-out: the integration branch moved outside oh.
func (d *Driver) tampered(ctx context.Context, parent, ticket string, ig *domain.FanoutIntegration, head string) {
	msg := i18n.Tf("cmd.fanout.integration_moved", ig.Branch, short(ig.Head), short(head))
	_ = d.record(ctx, parent, func(f *domain.SessionFanout) {
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
		if f.Status != domain.FanoutStopped {
			f.AddEvent(domain.FanoutEventResumed, "", parent, "")
		}
	})
	if err != nil {
		return err
	}
	if f.Status == domain.FanoutStopped {
		return nil // the parent ended: nobody to tell
	}
	// Telling the parent is the last write of the driver: as soon as the parent
	// is told it may ask a fix round (RequestFix, in the daemon, outside the
	// lock of update), and a record written by the driver after that, from a
	// copy read before, would take the round away.
	text := Summary(f)
	if err := d.Launcher.Tell(ctx, parent, text); err != nil {
		return fmt.Errorf("telling the parent: %w", err)
	}
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
	tasks := len(f.Children) > 0 && f.Children[0].Task != nil
	if tasks {
		b.WriteString("\n\n| Tâche | État | Session | Merge | Détail |\n|---|---|---|---|---|\n")
	} else {
		b.WriteString("\n\n| Ticket | État | Session | Merge | Détail |\n|---|---|---|---|---|\n")
	}
	for _, c := range f.Children {
		detail := c.Error
		if detail == "" && len(c.Conflicts) > 0 {
			detail = i18n.Tf("cmd.fanout.summary_resolved", strings.Join(c.Conflicts, ", "))
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", ChildLabel(c), c.State, c.Session, short(c.Merge), strings.ReplaceAll(detail, "|", "/"))
	}
	if f.BudgetUSD > 0 {
		b.WriteString("\n" + i18n.Tf("cmd.fanout.summary_budget", f.SpentUSD, f.BudgetUSD))
	}
	if f.Status != domain.FanoutFailed {
		if tasks {
			b.WriteString("\n\n" + i18n.T("cmd.fanout.summary_next_tasks"))
		} else {
			b.WriteString("\n\n" + i18n.T("cmd.fanout.summary_next"))
		}
	}
	return b.String()
}

// ChildLabel names a child: its ticket, or its task and title.
func ChildLabel(c domain.FanoutChild) string {
	if c.Task == nil {
		return c.Ticket
	}
	return c.Ticket + " " + strings.ReplaceAll(c.Task.Title, "|", "/")
}
