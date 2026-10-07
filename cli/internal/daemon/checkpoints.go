package daemon

import (
	"context"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Checkpoints in the watcher (P3-T03, P3-T04, P3-T06): tool calls feed the
// CheckpointService; each change of the workflow state re-applies the
// session rules (agent locks, circuit breaker) to the oh session and to the
// subagent sessions it delegated to.

// onCall follows a tool call of root (or of one of its subagent sessions).
func (w *watcher) onCall(ctx context.Context, root string, call *adapters.ToolCall) {
	cp := w.d.opts.Checkpoints
	if cp == nil || call == nil {
		return
	}
	c := *call
	w.mu.Lock()
	if w.callAgent == nil {
		w.callAgent = map[string]string{}
	}
	if agent, _ := c.Input["agent"].(string); c.Status == adapters.CallCalled && agent != "" {
		w.callAgent[c.ID] = agent
	} else if c.Status != adapters.CallCalled {
		if agent := w.callAgent[c.ID]; agent != "" {
			c.Input = map[string]any{"agent": agent}
		}
		delete(w.callAgent, c.ID)
	}
	w.mu.Unlock()
	changed, raised, err := cp.OnCall(ctx, root, c)
	if err != nil {
		slog.Debug("ohd: checkpoint state", "session", root, "error", err)
		return
	}
	if raised != nil && w.d.opts.Decisions != nil {
		if err := w.d.opts.Decisions.Upsert(ctx, raised); err != nil {
			slog.Warn("ohd: circuit breaker decision", "session", root, "error", err)
		}
		w.refreshAlerts(ctx, root)
	}
	if changed {
		w.applyRules(ctx, root)
	}
}

// onUserInput resets the circuit breaker counter of a session.
func (w *watcher) onUserInput(ctx context.Context, root string) {
	if cp := w.d.opts.Checkpoints; cp != nil {
		cp.OnUserInput(ctx, root)
	}
}

// applyRules sets the session rules of root on it and on its subagent sessions.
func (w *watcher) applyRules(ctx context.Context, root string) {
	w.mu.Lock()
	targets := []string{root}
	for child, r := range w.children {
		if r == root {
			targets = append(targets, child)
		}
	}
	w.mu.Unlock()
	w.applyRulesTo(ctx, root, targets...)
	w.syncCheckpoints(ctx, root)
}

// applyRulesTo sets the rules of root on targets (nothing for a session
// that does not run a workflow: its creation rules stay).
func (w *watcher) applyRulesTo(ctx context.Context, root string, targets ...string) {
	cp := w.d.opts.Checkpoints
	setter, ok := w.ad.(adapters.SessionRulesSetter)
	if cp == nil || !ok {
		return
	}
	rules, err := cp.Rules(ctx, root)
	if err != nil || rules == nil {
		return
	}
	for _, id := range targets {
		if err := setter.SetSessionRules(ctx, w.handle(), id, rules); err != nil {
			slog.Warn("ohd: session rules not applied", "session", id, "error", err)
		}
	}
}

// applyRules re-applies the rules of an oh session, wherever its server runs.
func (d *Daemon) applyRules(ctx context.Context, root string) {
	if d.opts.Sessions == nil {
		return
	}
	s, err := d.opts.Sessions.Get(ctx, root)
	if err != nil {
		return
	}
	d.wmu.Lock()
	w := d.watchers[s.GroupKey]
	d.wmu.Unlock()
	if w != nil {
		w.applyRules(ctx, root)
	}
}

// applyAllRules re-applies the rules of every tracked session of the group
// (after a reconnection: the server may have restarted).
func (w *watcher) applyAllRules(ctx context.Context, sessions []domain.Session) {
	for _, s := range sessions {
		if s.GroupKey == w.srv.GroupKey && !isTerminal(s.State) {
			w.applyRules(ctx, s.ID)
		}
	}
}
