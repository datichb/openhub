package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Pending decisions: the watcher mirrors the tool requests
// (permissions, questions) into the pending_decisions table and raises the
// decisions oh owns (agent loop failure, exhausted budget). The table is the
// source of the inbox; the tool remains the source of truth after a resync.

// syncToolDecisions records the requests the tool lists for a tool session
// and closes the ones it no longer lists (answered in the tool UI or the
// browser). Requests of a subagent (child) session are filed under the oh
// session that delegated it (root).
func (w *watcher) syncToolDecisions(ctx context.Context, root, toolSession string, pending []adapters.PendingDecision) {
	store := w.d.opts.Decisions
	if store == nil {
		return
	}
	listed := map[string]bool{}
	for _, p := range pending {
		if w.closeImitation(ctx, root, toolSession, p) {
			continue
		}
		var d domain.Decision
		cd, handled := w.checkpointDecision(ctx, root, toolSession, p)
		switch {
		case handled && cd == nil:
			continue // let through by the workflow mode
		case handled:
			d = *cd
		default:
			d = toolDecision(w.srv.GroupKey, root, toolSession, p)
		}
		listed[d.ID] = true
		if prev, err := store.Get(ctx, d.ID); err == nil && !prev.Open() && prev.ResolvedBy == domain.ResolvedByGone {
			// Lost with its server, asked again after the resume.
			_ = store.Reopen(ctx, d.ID)
		}
		if err := store.Upsert(ctx, &d); err != nil {
			slog.Debug("ohd: decision update failed", "decision", d.ID, "error", err)
		}
	}
	open, err := store.ListOpen(ctx, domain.DecisionFilter{SessionID: root})
	if err != nil {
		return
	}
	now := time.Now()
	lost := 0
	for _, d := range open {
		if d.ToolRef == "" || listed[d.ID] || !isToolKind(d.Kind) || d.ToolSessionID() != toolSession {
			continue
		}
		if d.Asleep() {
			// The request died with the server that went to sleep and the tool
			// does not ask it again: the decision ends, the agent is told.
			if ok, _ := store.Resolve(ctx, d.ID, domain.ResolvedByGone, nil, now); ok {
				lost++
			}
			continue
		}
		if ok, _ := store.Resolve(ctx, d.ID, domain.ResolvedByTool, nil, now); ok && d.Kind == domain.DecisionCheckpoint && w.d.opts.Checkpoints != nil {
			w.d.opts.Checkpoints.Settled(ctx, d)
		}
	}
	if lost > 0 {
		w.resumeAfterSleep(ctx, root)
	}
}

// resumeAfterSleep asks the agent of a session that woke up to ask again what
// it was waiting for when the server went to sleep (a permission, a question,
// a checkpoint): the request is not in the tool any more, the instruction is
// queued behind whatever the user sends.
func (w *watcher) resumeAfterSleep(ctx context.Context, root string) {
	op := adapters.ControlOp{Kind: adapters.ControlPrompt, Text: i18n.T("v54n.decision.resume_after_sleep"), Delivery: adapters.DeliveryQueue}
	if err := w.ad.Control(ctx, w.handle(), root, op); err != nil {
		slog.Warn("ohd: the request lost in the sleep was not asked again", "session", root, "error", err)
		return
	}
	slog.Info("ohd: the agent was asked again for the request lost in the sleep", "session", root)
}

// onAnswer records the answer given in the tool to a checkpoint request:
// only an observed validation lets the checkpoint pass.
func (w *watcher) onAnswer(ctx context.Context, ev adapters.ToolEvent) {
	store, cp := w.d.opts.Decisions, w.d.opts.Checkpoints
	if store == nil || cp == nil || ev.Answer.ID == "" {
		return
	}
	root := ev.SessionID
	if r, ok := w.rootOf(ev.SessionID); ok {
		root = r
	}
	d, err := store.Get(ctx, domain.DecisionID(domain.DecisionCheckpoint, root, ev.Answer.ID))
	if err != nil || d == nil || d.Kind != domain.DecisionCheckpoint {
		return
	}
	cp.ToolReplied(ctx, *d, ev.Answer.Approved)
}

// isToolKind reports whether a decision kind mirrors a tool request.
func isToolKind(k domain.DecisionKind) bool {
	return k == domain.DecisionPermission || k == domain.DecisionQuestion || k == domain.DecisionCheckpoint
}

// checkpointDecision turns a workflow_checkpoint permission request into a
// ⏸ decision (handled, d set), or lets it through when the checkpoint is
// automatic in the session mode (handled, d nil). Other requests: not handled.
func (w *watcher) checkpointDecision(ctx context.Context, root, toolSession string, p adapters.PendingDecision) (*domain.Decision, bool) {
	cp := w.d.opts.Checkpoints
	if cp == nil || p.Call == nil || p.Call.Action != bundle.CheckpointAction() {
		return nil, false
	}
	d, auto, relocked, err := cp.Asked(ctx, root, toolSession, p)
	if err != nil {
		slog.Debug("ohd: checkpoint request", "session", root, "error", err)
		return nil, false
	}
	if relocked {
		w.applyRules(ctx, root)
	}
	if auto {
		err := w.ad.Reply(ctx, w.handle(), adapters.DecisionReply{SessionID: toolSession, ID: p.ID, Kind: adapters.DecisionPermission, Decision: "once"})
		if err != nil && !errors.Is(err, adapters.ErrRequestGone) {
			slog.Warn("ohd: automatic checkpoint not let through", "session", root, "error", err)
		}
		return nil, true
	}
	return d, true
}

func toolDecision(group, sessionID, toolSession string, p adapters.PendingDecision) domain.Decision {
	d := domain.Decision{SessionID: sessionID, GroupKey: group, ToolRef: p.ID}
	if toolSession != "" && toolSession != sessionID {
		d.Payload.Data = map[string]any{domain.DataToolSession: toolSession}
	}
	switch p.Kind {
	case adapters.DecisionQuestion:
		d.Kind = domain.DecisionQuestion
		d.Payload.Title = p.Title
		for _, f := range p.Fields {
			df := domain.DecisionField{Key: f.Key, Title: f.Title, Description: f.Description, Type: f.Type, Custom: f.Custom, Required: f.Required}
			for _, o := range f.Options {
				df.Options = append(df.Options, domain.DecisionOption{Value: o.Value, Label: o.Label, Description: o.Description})
			}
			d.Payload.Fields = append(d.Payload.Fields, df)
		}
	default:
		d.Kind = domain.DecisionPermission
		d.Payload.Action, d.Payload.Resources, d.Payload.Message = p.Action, p.Resources, p.Message
	}
	d.ID = domain.DecisionID(d.Kind, sessionID, p.ID)
	return d
}

// raiseFailure records a failed agent loop: a budget decision when the
// proxy grant of the group is exhausted, an error decision otherwise.
func (w *watcher) raiseFailure(ctx context.Context, ev adapters.ToolEvent) {
	store := w.d.opts.Decisions
	if store == nil {
		return
	}
	kind, msg := domain.DecisionError, eventError(ev.Data)
	if w.d.proxy != nil && w.srv.ProxyTokenHash != "" && w.d.proxy.Exhausted(w.srv.ProxyTokenHash) {
		kind, msg = domain.DecisionBudget, ""
	}
	ref := ev.ID
	if ref == "" {
		ref = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	d := domain.Decision{ID: domain.DecisionID(kind, ev.SessionID, ref), SessionID: ev.SessionID, GroupKey: w.srv.GroupKey,
		Kind: kind, Payload: domain.DecisionPayload{Message: msg}}
	if !ev.Time.IsZero() {
		d.CreatedAt = ev.Time
	}
	if err := store.Upsert(ctx, &d); err != nil {
		slog.Debug("ohd: decision update failed", "decision", d.ID, "error", err)
	}
}

// clearAlerts closes the error/budget decisions of a session whose agent loop
// started again (the user went on in the tool).
func (w *watcher) clearAlerts(ctx context.Context, id string) {
	store := w.d.opts.Decisions
	if store == nil {
		return
	}
	open, err := store.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
	if err != nil {
		return
	}
	now := time.Now()
	for _, d := range open {
		if isLimitDecision(d) || isOhAlert(d) {
			continue // answered in oh only (raise, stop, dismiss)
		}
		if d.Kind == domain.DecisionError || d.Kind == domain.DecisionBudget {
			_, _ = store.Resolve(ctx, d.ID, domain.ResolvedByTool, nil, now)
		}
	}
}

// openAlerts counts the open decisions raised by oh for a session.
func (w *watcher) openAlerts(ctx context.Context, id string) int {
	store := w.d.opts.Decisions
	if store == nil {
		return 0
	}
	open, err := store.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
	if err != nil {
		return 0
	}
	n := 0
	for _, d := range open {
		if !isToolKind(d.Kind) {
			n++
		}
	}
	return n
}

// closeDecisions settles the open decisions of a server group whose server
// stops. Everything goes when stopping for good. When the group only goes to
// sleep, the tool requests (they die with the server) stay in « À traiter »,
// marked asleep, and the user is told once that a decision waits: the session
// takes them up when it wakes up (syncToolDecisions).
func (d *Daemon) closeDecisions(ctx context.Context, group string, final bool) {
	store := d.opts.Decisions
	if store == nil {
		return
	}
	open, err := store.ListOpen(ctx, domain.DecisionFilter{GroupKey: group})
	if err != nil {
		return
	}
	now := time.Now()
	var slept []string
	for _, dec := range open {
		switch {
		case final:
			if ok, _ := store.Resolve(ctx, dec.ID, domain.ResolvedByGone, nil, now); ok {
				d.feed.publishChange(domain.SessionChange{SessionID: dec.SessionID, GroupKey: group, Decisions: true})
			}
		case isToolKind(dec.Kind) && !dec.Asleep():
			if d.markAsleep(ctx, dec, now) {
				slept = append(slept, dec.SessionID)
				d.feed.publishChange(domain.SessionChange{SessionID: dec.SessionID, GroupKey: group, Decisions: true})
			}
		}
	}
	d.notes.sleeping(slept)
}

// markAsleep records on an open tool request that its server went to sleep.
func (d *Daemon) markAsleep(ctx context.Context, dec domain.Decision, at time.Time) bool {
	data := make(map[string]any, len(dec.Payload.Data)+1)
	for k, v := range dec.Payload.Data {
		data[k] = v
	}
	data[domain.DataAsleep] = at.UTC().Format(time.RFC3339)
	dec.Payload.Data = data
	if err := d.opts.Decisions.Upsert(ctx, &dec); err != nil {
		slog.Debug("ohd: decision not marked asleep", "decision", dec.ID, "error", err)
		return false
	}
	return true
}

// eventError extracts a human-readable error from an event payload.
func eventError(data map[string]any) string {
	for _, k := range []string{"error", "message", "reason"} {
		switch v := data[k].(type) {
		case string:
			return v
		case map[string]any:
			for _, kk := range []string{"message", "data", "name"} {
				if s, ok := v[kk].(string); ok && s != "" {
					return s
				}
				if m, ok := v[kk].(map[string]any); ok {
					if s, ok := m["message"].(string); ok && s != "" {
						return s
					}
				}
			}
		}
	}
	return ""
}
