package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"

	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionctx"
)

// Evolving session state for the entry agent (S8, QB8): checkpoints at each
// transition, remaining budget at a $ decision and after a raise; the resume
// instruction (set by the RunService) is cleared after the next step.

func (w *watcher) contextWriter() *sessionctx.Writer {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ctxw == nil {
		w.ctxw = &sessionctx.Writer{Adapter: w.ad, Dir: w.d.opts.SessionsDir}
	}
	return w.ctxw
}

// checkpointsContext is the oh.checkpoints entry of a workflow status.
func checkpointsContext(st checkpoint.Status) map[string]any {
	c := sessionctx.Checkpoints{Workflow: st.Workflow, Mode: st.Mode, Next: st.Next, Breaker: st.Breaker}
	for _, cp := range st.Checkpoints {
		switch cp.State {
		case checkpoint.StatePassed, checkpoint.StateSkipped:
			c.Passed = append(c.Passed, sessionctx.CheckpointRef{ID: cp.ID, Label: cp.Label})
		case checkpoint.StateWaiting:
			c.Current = cp.ID
		}
	}
	return c.Value()
}

// syncCheckpoints writes the checkpoint state of root when it changed.
func (w *watcher) syncCheckpoints(ctx context.Context, root string) {
	cp := w.d.opts.Checkpoints
	if cp == nil {
		return
	}
	st, err := cp.Status(ctx, root)
	if err != nil || len(st.Checkpoints) == 0 {
		return // not a workflow session, or a workflow without checkpoint (A29)
	}
	if w.deferWhileExecuting(root, func(ctx context.Context) { w.syncCheckpoints(ctx, root) }) {
		return
	}
	if _, err := w.contextWriter().Set(ctx, w.handle(), root, sessionctx.Entry{Key: sessionctx.KeyCheckpoints, Value: checkpointsContext(st)}); err != nil {
		slog.Debug("ohd: checkpoint context not written", "session", root, "error", err)
	}
}

// deferWhileExecuting keeps a state write for the end of the running step
// of root (A29): the tool announces a change at the next step boundary, and
// a change written during the last step of a loop makes the agent run one
// more step (« context updated… ») that replaces its real answer. Written
// once the loop ended, the change is announced at the next turn. It reports
// whether the write was deferred (the last one for a key wins).
func (w *watcher) deferWhileExecuting(root string, write func(ctx context.Context)) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	t, ok := w.tracks[root]
	if !ok || !t.executing {
		return false
	}
	t.deferred = append(t.deferred, write)
	return true
}

// flushDeferred runs the state writes kept during the step of root.
func (w *watcher) flushDeferred(ctx context.Context, root string) {
	w.mu.Lock()
	var writes []func(ctx context.Context)
	if t, ok := w.tracks[root]; ok {
		writes, t.deferred = t.deferred, nil
	}
	w.mu.Unlock()
	for _, write := range writes {
		write(ctx)
	}
}

func cents(v float64) float64 { return math.Round(v*100) / 100 }

// sessionBudget returns the session allowance (budget + raises) and what
// the session spent (ok=false: no session budget).
func (d *Daemon) sessionBudget(ctx context.Context, root string) (allowance, spent float64, ok bool) {
	if d.opts.Usage == nil || d.opts.SessionsDir == "" {
		return 0, 0, false
	}
	lim, err := limits.Load(d.opts.SessionsDir, root)
	if err != nil || lim.SessionBudgetUSD <= 0 {
		return 0, 0, false
	}
	extra, _ := d.opts.Usage.BudgetExtra(ctx, limits.SessionScope(root), "")
	tot, err := d.opts.Usage.SessionTotal(ctx, root)
	if err != nil {
		return 0, 0, false
	}
	return lim.SessionBudgetUSD + extra, tot.CostUSD, true
}

// syncBudget writes the budget of root: at a $ decision (exhausted), and
// when the allowance changed since the last write (a raise). Spending alone
// does not rewrite it (each write is a message in the tool history).
func (w *watcher) syncBudget(ctx context.Context, root string, exhausted bool) {
	allowance, spent, ok := w.d.sessionBudget(ctx, root)
	if !ok {
		return
	}
	if w.deferWhileExecuting(root, func(ctx context.Context) { w.syncBudget(ctx, root, exhausted) }) {
		return
	}
	wr := w.contextWriter()
	if !exhausted {
		var last struct {
			Limit float64 `json:"limit_usd"`
		}
		raw := wr.Last(root, sessionctx.KeyBudget)
		if raw == nil || json.Unmarshal(raw, &last) != nil || last.Limit == cents(allowance) {
			return // never written (no decision yet) or no raise since
		}
	}
	v := map[string]any{"limit_usd": cents(allowance), "spent_usd": cents(spent),
		"remaining_usd": cents(math.Max(0, allowance-spent)), "exhausted": exhausted}
	if _, err := wr.Set(ctx, w.handle(), root, sessionctx.Entry{Key: sessionctx.KeyBudget, Value: v}); err != nil {
		slog.Debug("ohd: budget context not written", "session", root, "error", err)
	}
}

// clearResume removes the resume instruction once a step ran.
func (w *watcher) clearResume(ctx context.Context, root string) {
	if err := w.contextWriter().Clear(ctx, w.handle(), root, sessionctx.KeyResume); err != nil {
		slog.Debug("ohd: resume context not cleared", "session", root, "error", err)
	}
}
