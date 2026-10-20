package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
)

// Responses cut before their end, in dollars (audit v5, S-27 follow-up).
// The cost of a session comes from the tool, which records nothing for a
// step that has no end (`finish`): a stream cut, often by oh itself when the
// budget is overrun, was counted in tokens by the proxy and never in
// dollars. When the proxy tells the daemon that a response was cut (Usage.Cut),
// the daemon adds an estimate to the ledger of the session: the tokens the
// proxy counted at the reservation rates (limits.CutCost), which err high.
//
// The estimate is a row of its own in the ledger (tool session
// "estimate:<root>", domain.EstimatedUsageID): it is summed with the others
// in the totals of the session and of the day, so the budgets see it at once,
// and it is the mark that it was not reported by the tool. No migration is
// needed.
//
// Double counting. The tool never records a cut step, so what it reports
// next is other steps. But if it did report this one, the cost would be
// counted twice. The estimate is therefore kept apart and given way for a
// minute: what the tool reports for the session in the minute that follows
// takes the place of the estimate, up to its amount (the ledger gets the
// difference back: the cost of that minute is the larger of the two, not their
// sum). After the minute the estimate is final. At worst this loses an estimate
// (a step reported soon after a cut hides it); it never counts a step twice. A
// restart of the daemon forgets what could still give way: the estimates stay.

// cutAbsorbWindow is how long what the tool reports takes the place of the
// estimate of a cut response.
const cutAbsorbWindow = time.Minute

type cutEstimate struct {
	usd   float64
	day   string
	until time.Time
}

// cutEstimates are the estimates that what the tool reports may still replace,
// by oh session.
type cutEstimates struct {
	mu   sync.Mutex
	open map[string][]cutEstimate
	now  func() time.Time // test hook (default time.Now)
}

func (c *cutEstimates) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *cutEstimates) add(root, day string, usd float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.open == nil {
		c.open = map[string][]cutEstimate{}
	}
	c.open[root] = append(c.open[root], cutEstimate{usd: usd, day: day, until: c.clock().Add(cutAbsorbWindow)})
}

// absorb takes reported dollars (the tool reported them for root) off the
// open estimates of root, oldest first, and returns what was taken, by day of
// the estimate: the amounts to give back to the ledger. Expired estimates are
// final and forgotten.
func (c *cutEstimates) absorb(root string, reported float64) (taken map[string]float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.clock()
	var keep []cutEstimate
	for _, e := range c.open[root] {
		if !now.Before(e.until) {
			continue
		}
		if take := min(reported, e.usd); take > 0 {
			if taken == nil {
				taken = map[string]float64{}
			}
			taken[e.day] += take
			reported -= take
			e.usd -= take
		}
		if e.usd > 1e-9 {
			keep = append(keep, e)
		}
	}
	if len(keep) == 0 {
		delete(c.open, root)
	} else {
		c.open[root] = keep
	}
	return taken
}

// countCut returns the function the gate calls when a request is over: when
// the response was cut, it adds the estimate of its dollars to the ledger of
// the session the request works for. roots are the sessions of the group
// whose loop ran when the request was admitted, the ones with a budget first;
// allowance bounds the estimate (0 = unbounded).
func (d *Daemon) countCut(w *watcher, roots []string, allowance float64) func(credproxy.Usage) {
	return func(u credproxy.Usage) {
		if u.Cut == 0 || d.opts.Usage == nil {
			return
		}
		usd := limits.CutCost(u.InputTokens, u.OutputTokens, u.CacheReadTokens, u.CacheWriteTokens, allowance)
		root := w.cutRoot(roots)
		if usd <= 0 || root == "" {
			slog.Debug("ohd: response cut, no session to count its cost for", "group", w.srv.GroupKey, "usd", usd)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		day := domain.UsageDay(d.cuts.clock())
		if err := d.opts.Usage.AddSession(ctx, domain.SessionUsage{Day: day, SessionID: domain.EstimatedUsageID(root), RootID: root,
			ProjectID: w.srv.ProjectID, GroupKey: w.srv.GroupKey, CostUSD: usd}); err != nil {
			slog.Debug("ohd: estimate of a cut response not recorded", "session", root, "error", err)
			return
		}
		d.cuts.add(root, day, usd)
		slog.Info("ohd: response cut before its end, estimated cost counted", "session", root, "usd", usd,
			"input_tokens", u.InputTokens, "output_tokens", u.OutputTokens, "estimated_tokens", u.Estimated > 0)
		w.storeTotal(ctx, root)
		w.raiseBudget(ctx, root, false)
	}
}

// cutRoot picks the oh session a cut response is counted for: the first of
// roots (the sessions working when it was admitted). Two sessions of a group
// are not told apart by the proxy: with several, the one with a budget comes
// first, since the daily budget does not depend on it. When none was working,
// the only session of the group.
func (w *watcher) cutRoot(roots []string) string {
	if len(roots) > 0 {
		if len(roots) > 1 {
			slog.Debug("ohd: several sessions work in the group, a cut response is counted for the first", "group", w.srv.GroupKey, "sessions", roots)
		}
		return roots[0]
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.tracks) == 1 {
		for id := range w.tracks {
			return id
		}
	}
	return ""
}

// absorbCuts gives back to the ledger what the tool's report of root takes
// the place of (see the comment above).
func (w *watcher) absorbCuts(ctx context.Context, root string, reported float64) {
	store := w.d.opts.Usage
	if store == nil || reported <= 0 {
		return
	}
	for day, usd := range w.d.cuts.absorb(root, reported) {
		if err := store.AddSession(ctx, domain.SessionUsage{Day: day, SessionID: domain.EstimatedUsageID(root), RootID: root,
			ProjectID: w.srv.ProjectID, GroupKey: w.srv.GroupKey, CostUSD: -usd}); err != nil {
			slog.Debug("ohd: estimate of a cut response not given back", "session", root, "error", err)
		}
	}
}
