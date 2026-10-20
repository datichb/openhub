package daemon

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
)

// Control of the budgets at each model request (F-04, S-27). The credential
// proxy asks the daemon before relaying a request (admitModel):
//
//   - the budget of a session that is spent, or whose decision is open,
//     refuses the request (402) and raises the decision if it was not yet:
//     the subagents of a step that overran stop at their next request, not
//     when the root step ends. The step is marked as interrupted by the
//     budget, as for a loop stopped by holdOverBudget: it is no error and it
//     resumes once the decision is answered;
//   - the amount a request may cost is reserved while it runs: concurrent
//     requests cannot all be admitted on the same remaining budget; one that
//     would pass the budget with the others in flight is told to retry (429).
//
// The group of the grant is the only thing a request tells about its
// session: the requests are checked against the sessions of the group that
// are working. A request alone is never refused for its estimate, the
// budgets are soft caps. The cost itself is the one the tool reports.

// retryReserved is the delay announced to a request refused for the
// reservations of the others.
const retryReserved = 2 * time.Second

// budgetReservations are the dollars held by the requests in flight, by
// budget (budgetLine.reservationKey).
type budgetReservations struct {
	mu   sync.Mutex
	held map[string]float64
}

// admitModel is the Gate of the credential proxy.
func (d *Daemon) admitModel(ctx context.Context, owner string, a credproxy.Admission) (func(credproxy.Usage), error) {
	noop := func(credproxy.Usage) {}
	if d.opts.Usage == nil || d.opts.SessionsDir == "" {
		return noop, nil
	}
	d.wmu.Lock()
	w := d.watchers[owner]
	d.wmu.Unlock()
	if w == nil {
		return noop, nil
	}
	lines := map[string]budgetLine{}
	for _, root := range w.executingRoots() {
		own := d.budgetLines(ctx, root)
		if len(own) == 0 {
			continue
		}
		if d.openLimitDecision(ctx, root) || w.raiseBudget(ctx, root, false) {
			w.holdForBudget(root)
			return nil, &credproxy.Refusal{Message: i18n.Tf("cmd.budget.refused.spent", root)}
		}
		for _, l := range own {
			if _, ok := lines[l.reservationKey()]; !ok {
				lines[l.reservationKey()] = l
			}
		}
	}
	if len(lines) == 0 {
		return noop, nil
	}
	held := make(map[string]float64, len(lines))
	for k, l := range lines {
		held[k] = limits.Reserve(a.InputTokens, a.MaxOutputTokens, l.allowance)
	}
	r := &d.reservations
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, l := range lines {
		if other := r.held[k]; other > 0 && l.spent+other+held[k] > l.allowance {
			return nil, &credproxy.Refusal{Message: i18n.T("cmd.budget.refused.reserved"), RetryAfter: retryReserved}
		}
	}
	if r.held == nil {
		r.held = map[string]float64{}
	}
	for k, usd := range held {
		r.held[k] += usd
	}
	var once sync.Once
	return func(credproxy.Usage) {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			for k, usd := range held {
				if r.held[k] -= usd; r.held[k] < 1e-9 {
					delete(r.held, k)
				}
			}
		})
	}, nil
}

// executingRoots returns the oh sessions of the group whose agent loop runs.
func (w *watcher) executingRoots() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []string
	for id, t := range w.tracks {
		if t.executing {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// holdForBudget marks the step of a session as interrupted by its budget:
// it is no failure, and it resumes when the decision is answered
// (afterOhDecision).
func (w *watcher) holdForBudget(root string) {
	w.mu.Lock()
	w.track(root).held = true
	w.mu.Unlock()
}
