package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Pending decisions (P3-T07): the watcher mirrors the tool requests
// (permissions, questions) into the pending_decisions table and raises the
// decisions oh owns (agent loop failure, exhausted budget). The table is the
// source of the inbox; the tool remains the source of truth after a resync.

// syncToolDecisions records the requests the tool lists for a session and
// closes the ones it no longer lists (answered in the tool UI or the browser).
func (w *watcher) syncToolDecisions(ctx context.Context, id string, pending []adapters.PendingDecision) {
	store := w.d.opts.Decisions
	if store == nil {
		return
	}
	listed := map[string]bool{}
	for _, p := range pending {
		d := toolDecision(w.srv.GroupKey, id, p)
		listed[d.ID] = true
		if prev, err := store.Get(ctx, d.ID); err == nil && !prev.Open() && prev.ResolvedBy == domain.ResolvedByGone {
			// Lost with its server, asked again after the resume.
			_ = store.Reopen(ctx, d.ID)
		}
		if err := store.Upsert(ctx, &d); err != nil {
			slog.Debug("ohd: decision update failed", "decision", d.ID, "error", err)
		}
	}
	open, err := store.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
	if err != nil {
		return
	}
	now := time.Now()
	for _, d := range open {
		if d.ToolRef == "" || listed[d.ID] || !isToolKind(d.Kind) {
			continue
		}
		_, _ = store.Resolve(ctx, d.ID, domain.ResolvedByTool, nil, now)
	}
}

// isToolKind reports whether a decision kind mirrors a tool request.
func isToolKind(k domain.DecisionKind) bool {
	return k == domain.DecisionPermission || k == domain.DecisionQuestion
}

func toolDecision(group, sessionID string, p adapters.PendingDecision) domain.Decision {
	if p.SessionID != "" {
		sessionID = p.SessionID
	}
	d := domain.Decision{SessionID: sessionID, GroupKey: group, ToolRef: p.ID}
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
	if w.d.proxy != nil && w.srv.ProxyToken != "" && w.d.proxy.Exhausted(w.srv.ProxyToken) {
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
			_, _ = store.Resolve(ctx, dec.ID, domain.ResolvedByGone, nil, now)
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
