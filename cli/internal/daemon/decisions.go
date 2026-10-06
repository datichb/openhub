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
)

// Pending decisions (P3-T07): the watcher mirrors the tool requests
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
	for _, d := range open {
		if d.ToolRef == "" || listed[d.ID] || !isToolKind(d.Kind) || d.ToolSessionID() != toolSession {
			continue
		}
		if ok, _ := store.Resolve(ctx, d.ID, domain.ResolvedByTool, nil, now); ok && d.Kind == domain.DecisionCheckpoint && w.d.opts.Checkpoints != nil {
			w.d.opts.Checkpoints.Settled(ctx, d)
		}
	}
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
	d, auto, err := cp.Asked(ctx, root, toolSession, p)
	if err != nil {
		slog.Debug("ohd: checkpoint request", "session", root, "error", err)
		return nil, false
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

// closeDecisions resolves the open decisions of a server group whose server
// stops: tool requests die with it; everything goes when stopping for good.
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
	for _, dec := range open {
		if final || isToolKind(dec.Kind) {
			if ok, _ := store.Resolve(ctx, dec.ID, domain.ResolvedByGone, nil, now); ok {
				d.feed.publishChange(domain.SessionChange{SessionID: dec.SessionID, GroupKey: group, Decisions: true})
			}
		}
	}
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
