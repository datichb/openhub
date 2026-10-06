package daemon

import (
	"context"
	"log/slog"
	"sort"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
)

// Queue of first prompts (I6): a session created beyond the maximum of
// working sessions (or over the memory cap) waits in state "queued"; the
// daemon sends its prompt when a slot frees up, by priority (interactive
// first), then first queued first. A group with a queued session never
// sleeps.

type queuedSession struct {
	sess domain.Session
	q    limits.Queued
}

// dequeue starts the queued sessions that fit, and returns the groups that
// still hold queued sessions.
func (d *Daemon) dequeue(ctx context.Context, ready []domain.Server) map[string]bool {
	held := map[string]bool{}
	if d.opts.Sessions == nil || d.opts.SessionsDir == "" || d.opts.Adapter == nil {
		return held
	}
	all, err := d.opts.Sessions.List(ctx, "")
	if err != nil {
		return held
	}
	working := map[string]int{}
	var queue []queuedSession
	for _, s := range all {
		switch s.State {
		case domain.RunActive:
			working[limits.ScopeGlobal]++
			working[limits.ProjectScope(s.ProjectID)]++
		case domain.RunQueued:
			q, ok, err := limits.LoadQueued(d.opts.SessionsDir, s.ID)
			if err != nil || !ok {
				continue
			}
			queue = append(queue, queuedSession{sess: s, q: q})
		}
	}
	if len(queue) == 0 {
		return held
	}
	sort.SliceStable(queue, func(i, j int) bool { return queue[i].q.Less(queue[j].q) })
	servers := map[string]domain.Server{}
	for _, s := range ready {
		servers[s.GroupKey] = s
	}
	memory := d.memoryUsed()
	blocked := map[string]bool{} // scopes where an earlier session waits
	for _, e := range queue {
		srv, ok := servers[e.sess.GroupKey]
		full := blocked[e.q.Scope] ||
			(e.q.MaxActive > 0 && working[e.q.Scope] >= e.q.MaxActive) ||
			(e.q.MemoryMB > 0 && memory >= e.q.MemoryMB)
		if !ok || full {
			blocked[e.q.Scope] = true
			held[e.sess.GroupKey] = true
			continue
		}
		ad := d.opts.Adapter(srv.Adapter)
		if ad == nil {
			continue
		}
		h := adapters.ServerHandle{URL: srv.URL, Password: srv.Password, PID: srv.PID}
		if err := ad.SendPrompt(ctx, h, e.sess.ID, e.q.Prompt); err != nil {
			slog.Warn("ohd: queued session not started", "session", e.sess.ID, "error", err)
			held[e.sess.GroupKey] = true
			continue
		}
		_ = limits.RemoveQueued(d.opts.SessionsDir, e.sess.ID)
		s := e.sess
		s.State = domain.RunActive
		if err := d.opts.Sessions.Update(ctx, &s); err == nil {
			d.feed.publishChange(domain.SessionChange{SessionID: s.ID, GroupKey: s.GroupKey, State: s.State})
		}
		working[limits.ScopeGlobal]++
		working[limits.ProjectScope(s.ProjectID)]++
		slog.Info("ohd: queued session started", "session", s.ID)
	}
	return held
}
