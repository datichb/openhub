package daemon

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
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

// closeImitation answers a question of a workflow session that imitates a
// checkpoint (A36): it validates nothing, so oh ends it at once and tells
// the agent to call workflow_checkpoint. The answer carries the instruction
// when every field takes free text; otherwise the question is dismissed and
// the instruction sent to the agent.
func (w *watcher) closeImitation(ctx context.Context, root, toolSession string, p adapters.PendingDecision) bool {
	cp := w.d.opts.Checkpoints
	if cp == nil || p.Kind != adapters.DecisionQuestion {
		return false
	}
	c, ok := cp.Imitation(ctx, root, p)
	if !ok {
		return false
	}
	text := i18n.Tf("tui.checkpoint.imitation_steer", c.ID, c.LabelFor(i18n.Locale()))
	answer, free := map[string]any{}, len(p.Fields) > 0
	for _, f := range p.Fields {
		if !f.Custom || (f.Type != "" && f.Type != "string") {
			free = false
			break
		}
		answer[f.Key] = text
	}
	if free {
		err := w.ad.Reply(ctx, w.handle(), adapters.DecisionReply{SessionID: toolSession, ID: p.ID, Kind: adapters.DecisionQuestion, Answer: answer})
		if err == nil || errors.Is(err, adapters.ErrRequestGone) {
			return true
		}
		slog.Debug("ohd: imitated checkpoint not answered", "session", root, "error", err)
	}
	canceller, ok := w.ad.(adapters.QuestionCanceller)
	if !ok {
		return false // left to the user, as any question
	}
	if err := canceller.CancelQuestion(ctx, w.handle(), toolSession, p.ID); err != nil && !errors.Is(err, adapters.ErrRequestGone) {
		slog.Warn("ohd: imitated checkpoint not dismissed", "session", root, "error", err)
		return false
	}
	if err := w.ad.Control(ctx, w.handle(), toolSession, adapters.ControlOp{Kind: adapters.ControlPrompt, Text: text, Delivery: adapters.DeliverySteer}); err != nil {
		slog.Warn("ohd: instruction after an imitated checkpoint not sent", "session", root, "error", err)
	}
	return true
}

// finishWorkflow runs when the step of root ended: when its workflow reached
// its end and nothing waits, oh declares the outputs the agent did not and
// the session is completed (A38), which offers « Chain with… ».
func (w *watcher) finishWorkflow(ctx context.Context, root string) {
	cp := w.d.opts.Checkpoints
	if cp == nil {
		return
	}
	w.mu.Lock()
	t := w.track(root)
	idle := !t.executing && t.waiting() == 0 && t.alerts == 0
	w.mu.Unlock()
	if !idle {
		return
	}
	branch := func() string {
		if res, err := w.ad.Results(ctx, w.handle(), root); err == nil {
			return res.Branch
		}
		return ""
	}
	done, err := cp.Finish(ctx, root, branch)
	if err != nil || !done {
		return
	}
	sess, err := w.d.opts.Sessions.Get(ctx, root)
	if err != nil || isTerminal(sess.State) {
		return
	}
	now := time.Now()
	sess.State, sess.StateChangedAt, sess.Status, sess.EndedAt = domain.RunCompleted, &now, domain.SessionStatusCompleted, &now
	if err := w.d.opts.Sessions.Update(ctx, sess); err != nil {
		slog.Debug("ohd: session completion not saved", "session", root, "error", err)
		return
	}
	w.d.feed.publishChange(domain.SessionChange{SessionID: root, GroupKey: w.srv.GroupKey, State: sess.State})
}

// reopenCompleted puts back in play a session completed by the end of its
// workflow when it works again (the user went on in the tool).
func (w *watcher) reopenCompleted(ctx context.Context, root string) {
	sess, err := w.d.opts.Sessions.Get(ctx, root)
	if err != nil || sess.State != domain.RunCompleted {
		return
	}
	if cp := w.d.opts.Checkpoints; cp != nil {
		cp.Reopen(ctx, root)
	}
	now := time.Now()
	sess.State, sess.StateChangedAt, sess.Status, sess.EndedAt = domain.RunActive, &now, domain.SessionStatusRunning, nil
	if err := w.d.opts.Sessions.Update(ctx, sess); err != nil {
		slog.Debug("ohd: session not reopened", "session", root, "error", err)
		return
	}
	w.d.feed.publishChange(domain.SessionChange{SessionID: root, GroupKey: w.srv.GroupKey, State: sess.State})
}
