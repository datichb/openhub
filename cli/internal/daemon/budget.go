package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
)

// Budgets (I6). The cost the tool reports for each session (and for its
// subagent sessions) is added to the usage ledger as it grows. When a
// session's budget or the daily budget is spent, a budget decision ($) is
// raised; while it is open, any new agent loop of the session is
// interrupted, and so is any model request (budgetgate.go). Answers: raise
// the budget, stop the session, or dismiss (one more step).
//
// The decision is raised as soon as the spending is seen (F-04): when a
// subagent spends or ends, and when a model request is admitted, not only
// at the end of the root step, which may delegate to a dozen subagents.
// Budgets stay soft caps: a request in flight is not cut, and dismissing the
// decision lets the step run on until it ends.

// isLimitDecision reports whether a decision is a budget decision of the
// restrictions.
func isLimitDecision(d domain.Decision) bool {
	return d.Kind == domain.DecisionBudget && limits.IsBudgetData(d.Payload.Data)
}

// ledger keeps the last usage accounted per tool session (deltas).
type ledger struct {
	mu   sync.Mutex
	last map[string]adapters.SessionResult
}

// account records in the ledger what a tool session spent since the last
// call (toolSession is root itself or one of its subagent sessions).
func (w *watcher) account(ctx context.Context, root, toolSession string, res adapters.SessionResult) {
	store := w.d.opts.Usage
	if store == nil {
		return
	}
	l := &w.d.ledger
	l.mu.Lock()
	defer l.mu.Unlock()
	prev, ok := l.last[toolSession]
	if !ok {
		tot, err := store.ToolSessionTotal(ctx, toolSession)
		if err != nil {
			return
		}
		// A fork reports the copied history from the start: only what it
		// spends above that baseline is its own.
		base := limits.LoadBaseline(w.d.opts.SessionsDir, toolSession)
		prev = adapters.SessionResult{Cost: tot.CostUSD + base.CostUSD, TokensIn: tot.TokensIn + base.TokensIn, TokensOut: tot.TokensOut + base.TokensOut}
	}
	d := domain.SessionUsage{Day: domain.UsageDay(time.Now()), SessionID: toolSession, RootID: root,
		ProjectID: w.srv.ProjectID, GroupKey: w.srv.GroupKey,
		CostUSD: math.Max(0, res.Cost-prev.Cost), TokensIn: max(0, res.TokensIn-prev.TokensIn), TokensOut: max(0, res.TokensOut-prev.TokensOut)}
	// What is accounted never goes below the baseline: a fork read before
	// its copied history shows keeps it.
	seen := adapters.SessionResult{Cost: math.Max(res.Cost, prev.Cost), TokensIn: max(res.TokensIn, prev.TokensIn), TokensOut: max(res.TokensOut, prev.TokensOut)}
	if d.CostUSD == 0 && d.TokensIn == 0 && d.TokensOut == 0 {
		l.last[toolSession] = seen
		return
	}
	if err := store.AddSession(ctx, d); err != nil {
		slog.Debug("ohd: usage not recorded", "session", toolSession, "error", err)
		return
	}
	l.last[toolSession] = seen
}

// accountChild reads and records the usage of a subagent session.
func (w *watcher) accountChild(ctx context.Context, root, child string) {
	if w.d.opts.Usage == nil {
		return
	}
	if res, err := w.usage(ctx, child); err == nil {
		w.account(ctx, root, child, res)
		w.storeTotal(ctx, root)
	}
}

// sessionTotal returns the usage of an oh session to show (v5 corrections,
// A21): the ledger total of the root and its subagent sessions, as the
// budget counts it, in place of what the root tool session alone reports
// (res, kept when the ledger has nothing yet).
func (w *watcher) sessionTotal(ctx context.Context, root string, res adapters.SessionResult) adapters.SessionResult {
	if w.d.opts.Usage == nil {
		return res
	}
	tot, err := w.d.opts.Usage.SessionTotal(ctx, root)
	if err != nil || (tot.CostUSD == 0 && tot.TokensIn == 0 && tot.TokensOut == 0) {
		return res
	}
	res.Cost, res.TokensIn, res.TokensOut = tot.CostUSD, tot.TokensIn, tot.TokensOut
	return res
}

// storeTotal writes the usage total of an oh session after one of its
// subagent sessions spent (the root itself may stay idle meanwhile).
func (w *watcher) storeTotal(ctx context.Context, root string) {
	sess, err := w.d.opts.Sessions.Get(ctx, root)
	if err != nil || isTerminal(sess.State) {
		return
	}
	res := w.sessionTotal(ctx, root, adapters.SessionResult{Cost: sess.Cost, TokensIn: sess.TokensIn, TokensOut: sess.TokensOut})
	if res.Cost == sess.Cost && res.TokensIn == sess.TokensIn && res.TokensOut == sess.TokensOut {
		return
	}
	sess.Cost, sess.TokensIn, sess.TokensOut = res.Cost, res.TokensIn, res.TokensOut
	if err := w.d.opts.Sessions.Update(ctx, sess); err != nil {
		slog.Debug("ohd: session usage update failed", "session", root, "error", err)
		return
	}
	w.d.feed.publishChange(domain.SessionChange{SessionID: root, GroupKey: w.srv.GroupKey, State: sess.State})
}

// budgetLine is one budget of a session with what it spent.
type budgetLine struct {
	kind, scope, day string // limits.BudgetSession | BudgetDaily; raise scope; day of a daily raise
	budget, spent    float64
	allowance        float64 // budget + raises
}

// reservationKey names what the reservations of a line are held against.
func (l budgetLine) reservationKey() string { return l.kind + ":" + l.scope + ":" + l.day }

// budgetLines returns the budgets the restrictions of a session set, with
// their spending (a ledger error leaves the line out).
func (d *Daemon) budgetLines(ctx context.Context, root string) []budgetLine {
	if d.opts.Usage == nil || d.opts.SessionsDir == "" {
		return nil
	}
	lim, err := limits.Load(d.opts.SessionsDir, root)
	if err != nil || (lim.SessionBudgetUSD == 0 && lim.DailyBudgetUSD == 0) {
		return nil
	}
	var out []budgetLine
	line := func(kind, scope, day string, budget, spent float64) {
		extra, _ := d.opts.Usage.BudgetExtra(ctx, scope, day)
		out = append(out, budgetLine{kind: kind, scope: scope, day: day, budget: budget, spent: spent, allowance: budget + extra})
	}
	if lim.SessionBudgetUSD > 0 {
		if tot, err := d.opts.Usage.SessionTotal(ctx, root); err == nil {
			line(limits.BudgetSession, limits.SessionScope(root), "", lim.SessionBudgetUSD, tot.CostUSD)
		}
	}
	if lim.DailyBudgetUSD > 0 {
		day, scope := domain.UsageDay(time.Now()), lim.DailyScope()
		if spent, err := d.opts.Usage.DayCost(ctx, day, limits.ScopeProject(scope)); err == nil {
			line(limits.BudgetDaily, scope, day, lim.DailyBudgetUSD, spent)
		}
	}
	return out
}

// overBudget returns the budget decision a session must get now, if any.
func (d *Daemon) overBudget(ctx context.Context, w *watcher, root string) *domain.Decision {
	for _, l := range d.budgetLines(ctx, root) {
		if l.spent < l.allowance {
			continue
		}
		return &domain.Decision{
			ID:        domain.DecisionID(domain.DecisionBudget, root, fmt.Sprintf("%s-%s-%.4f", l.kind, l.day, l.allowance)),
			SessionID: root, GroupKey: w.srv.GroupKey, Kind: domain.DecisionBudget,
			Payload: domain.DecisionPayload{Message: i18n.Tf("cmd.budget.decision."+l.kind, l.spent, l.allowance), Data: map[string]any{
				limits.DataBudgetLimit: l.kind, limits.DataBudgetScope: l.scope, limits.DataBudgetDay: l.day,
				limits.DataBudgetUSD: l.allowance, limits.DataBudgetSpent: l.spent, limits.DataBudgetUnit: l.budget,
			}},
		}
	}
	return nil
}

// raiseBudget records the budget decision of a session over its budget
// (idempotent per allowance: a raise opens a new one later). It reports
// whether a decision is open for the session after the call.
//
// A decision dismissed by the user (one more step) is asked again after that
// step: at its end (stepEnd), not when a subagent spends or a request is
// admitted meanwhile.
func (w *watcher) raiseBudget(ctx context.Context, root string, stepEnd bool) bool {
	store := w.d.opts.Decisions
	if store == nil {
		return false
	}
	dec := w.d.overBudget(ctx, w, root)
	if dec == nil {
		return false
	}
	if prev, err := store.Get(ctx, dec.ID); err == nil && prev != nil {
		if prev.Open() {
			return true
		}
		if !stepEnd {
			return false
		}
		if err := store.Reopen(ctx, dec.ID); err != nil {
			return false
		}
		w.refreshAlerts(ctx, root)
		return true
	}
	if err := store.Upsert(ctx, dec); err != nil {
		slog.Debug("ohd: budget decision not recorded", "session", root, "error", err)
		return false
	}
	slog.Info("ohd: session over budget", "session", root, "limit", dec.Payload.Data[limits.DataBudgetLimit])
	w.refreshAlerts(ctx, root)
	if dec.Payload.Data[limits.DataBudgetLimit] == limits.BudgetSession {
		w.syncBudget(ctx, root, true)
	}
	return true
}

// openLimitDecision reports whether a session has an open budget decision
// from the restrictions.
func (d *Daemon) openLimitDecision(ctx context.Context, root string) bool {
	if d.opts.Decisions == nil {
		return false
	}
	open, err := d.opts.Decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: root})
	if err != nil {
		return false
	}
	for _, dec := range open {
		if isLimitDecision(dec) {
			return true
		}
	}
	return false
}

// holdOverBudget interrupts an agent loop started while the session waits
// for its budget decision (the user typed in the tool, or a queued
// follow-up started). It reports whether it did.
func (w *watcher) holdOverBudget(ctx context.Context, root, toolSession string) bool {
	if !w.d.openLimitDecision(ctx, root) {
		return false
	}
	err := w.ad.Control(ctx, w.handle(), toolSession, adapters.ControlOp{Kind: adapters.ControlInterrupt})
	if err != nil {
		slog.Warn("ohd: over-budget step not interrupted", "session", toolSession, "error", err)
		return false
	}
	slog.Info("ohd: step interrupted, budget decision pending", "session", toolSession)
	w.mu.Lock()
	w.track(root).held = true
	w.mu.Unlock()
	return true
}

// afterOhDecision brings a session up to date once one of the decisions
// raised by oh (budget, error) was answered in another process (v5
// corrections, A22): its state no longer shows « waiting », the raised
// budget is written to its state at once, and a step interrupted while the
// decision was open is resumed by a short instruction.
func (w *watcher) afterOhDecision(ctx context.Context, root string) {
	w.refreshAlerts(ctx, root)
	w.persist(ctx, root, false)
	if w.d.openLimitDecision(ctx, root) {
		return
	}
	w.syncBudget(ctx, root, false)
	w.mu.Lock()
	t := w.track(root)
	held := t.held
	t.held = false
	w.mu.Unlock()
	if !held {
		return
	}
	op := adapters.ControlOp{Kind: adapters.ControlPrompt, Text: i18n.T("cmd.budget.resume"), Delivery: adapters.DeliveryQueue}
	if err := w.ad.Control(ctx, w.handle(), root, op); err != nil {
		slog.Warn("ohd: interrupted step not resumed", "session", root, "error", err)
		return
	}
	slog.Info("ohd: interrupted step resumed after the budget decision", "session", root)
}

// sessionDecided runs afterOhDecision in the watcher of the session.
func (d *Daemon) sessionDecided(ctx context.Context, id string) bool {
	if d.opts.Sessions == nil {
		return false
	}
	s, err := d.opts.Sessions.Get(ctx, id)
	if err != nil {
		return false
	}
	d.wmu.Lock()
	w := d.watchers[s.GroupKey]
	d.wmu.Unlock()
	if w == nil {
		return false
	}
	w.afterOhDecision(ctx, id)
	return true
}
