package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Session tracking (E10): one watcher per live tool server subscribes to the
// tool event stream and keeps the oh sessions table up to date (run state,
// cost, tokens). The stream is live-only, so each (re)connection starts with a
// resynchronization from the tool list endpoints.

const (
	touchEvery = 10 * time.Second
	usageEvery = 5 * time.Second
)

type sessionTrack struct {
	executing bool
	pending   int
	lastUsage time.Time
}

type watcher struct {
	d      *Daemon
	srv    domain.Server
	ad     adapters.ToolAdapter
	cancel context.CancelFunc

	mu        sync.Mutex
	tracks    map[string]*sessionTrack
	known     map[string]bool // session id → belongs to oh (row in sessions table for this group)
	lastTouch time.Time
}

func (w *watcher) handle() adapters.ServerHandle {
	return adapters.ServerHandle{URL: w.srv.URL, Password: w.srv.Password, PID: w.srv.PID}
}

// syncWatchers starts watchers for live ready servers and stops the others.
func (d *Daemon) syncWatchers(ctx context.Context, live []domain.Server) {
	if d.opts.Adapter == nil || d.opts.Sessions == nil {
		return
	}
	d.wmu.Lock()
	defer d.wmu.Unlock()
	keep := map[string]bool{}
	for _, s := range live {
		keep[s.GroupKey] = true
		if w, ok := d.watchers[s.GroupKey]; ok && w.srv.PID == s.PID {
			continue
		} else if ok {
			w.cancel()
		}
		ad := d.opts.Adapter(s.Adapter)
		if ad == nil {
			continue
		}
		wctx, cancel := context.WithCancel(ctx)
		w := &watcher{d: d, srv: s, ad: ad, cancel: cancel, tracks: map[string]*sessionTrack{}, known: map[string]bool{}}
		d.watchers[s.GroupKey] = w
		go w.run(wctx)
	}
	for gk, w := range d.watchers {
		if !keep[gk] {
			w.cancel()
			delete(d.watchers, gk)
		}
	}
}

func (d *Daemon) stopWatchers() {
	d.wmu.Lock()
	defer d.wmu.Unlock()
	for gk, w := range d.watchers {
		w.cancel()
		delete(d.watchers, gk)
	}
}

func (w *watcher) run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		evs, err := w.ad.Events(ctx, w.handle())
		if err != nil {
			slog.Debug("ohd: event stream unavailable", "group", w.srv.GroupKey, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 10*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second
		for ev := range evs {
			w.onEvent(ctx, ev)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (w *watcher) onEvent(ctx context.Context, ev adapters.ToolEvent) {
	if ev.Kind == adapters.EventConnected {
		w.resync(ctx)
		return
	}
	if ev.SessionID == "" || !w.isKnown(ctx, ev.SessionID) {
		return
	}
	w.mu.Lock()
	t := w.track(ev.SessionID)
	refreshUsage := false
	refreshPending := false
	switch ev.Kind {
	case adapters.EventExecStarted:
		t.executing = true
	case adapters.EventExecEnded:
		t.executing = false
		refreshUsage = true
		refreshPending = true
	case adapters.EventDecisionAsked, adapters.EventDecisionReplied:
		refreshPending = true
	case adapters.EventUsage:
		if time.Since(t.lastUsage) > usageEvery {
			refreshUsage = true
		}
	}
	if refreshUsage {
		t.lastUsage = time.Now()
	}
	touch := time.Since(w.lastTouch) > touchEvery
	if touch {
		w.lastTouch = time.Now()
	}
	w.mu.Unlock()

	if refreshPending {
		w.refreshPending(ctx, ev.SessionID)
	}
	w.persist(ctx, ev.SessionID, refreshUsage)
	if touch && w.d.opts.Servers != nil {
		_ = w.d.opts.Servers.Touch(ctx, w.srv.GroupKey, time.Now())
	}
}

func (w *watcher) track(id string) *sessionTrack {
	t, ok := w.tracks[id]
	if !ok {
		t = &sessionTrack{}
		w.tracks[id] = t
	}
	return t
}

func (w *watcher) isKnown(ctx context.Context, id string) bool {
	w.mu.Lock()
	known, seen := w.known[id]
	w.mu.Unlock()
	if seen {
		return known
	}
	s, err := w.d.opts.Sessions.Get(ctx, id)
	known = err == nil && s.GroupKey == w.srv.GroupKey
	w.mu.Lock()
	w.known[id] = known
	w.mu.Unlock()
	return known
}

// resync rebuilds the state of every oh session of the group from the tool.
func (w *watcher) resync(ctx context.Context) {
	sessions, err := w.d.opts.Sessions.List(ctx, w.srv.ProjectID)
	if err != nil {
		return
	}
	active := map[string]bool{}
	if ids, err := w.ad.ActiveSessions(ctx, w.handle()); err == nil {
		for _, id := range ids {
			active[id] = true
		}
	}
	for _, s := range sessions {
		if s.GroupKey != w.srv.GroupKey || isTerminal(s.State) {
			continue
		}
		w.mu.Lock()
		w.known[s.ID] = true
		w.track(s.ID).executing = active[s.ID]
		w.mu.Unlock()
		w.refreshPending(ctx, s.ID)
		w.persist(ctx, s.ID, true)
	}
}

func (w *watcher) refreshPending(ctx context.Context, id string) {
	pending, err := w.ad.Pending(ctx, w.handle(), id)
	if err != nil {
		return
	}
	w.mu.Lock()
	w.track(id).pending = len(pending)
	w.mu.Unlock()
}

func isTerminal(s domain.RunState) bool {
	return s == domain.RunCompleted || s == domain.RunFailed || s == domain.RunStopped
}

// persist writes the derived run state (and usage when asked) to the store.
func (w *watcher) persist(ctx context.Context, id string, withUsage bool) {
	w.mu.Lock()
	t := w.track(id)
	state := domain.RunIdle
	switch {
	case t.pending > 0:
		state = domain.RunWaiting
	case t.executing:
		state = domain.RunActive
	}
	w.mu.Unlock()

	sess, err := w.d.opts.Sessions.Get(ctx, id)
	if err != nil || isTerminal(sess.State) {
		return
	}
	changed := sess.State != state
	sess.State = state
	if withUsage {
		if res, err := w.usage(ctx, id); err == nil {
			if res.Cost != sess.Cost || res.TokensIn != sess.TokensIn || res.TokensOut != sess.TokensOut {
				changed = true
			}
			sess.Cost, sess.TokensIn, sess.TokensOut = res.Cost, res.TokensIn, res.TokensOut
			sess.TokensReasoning, sess.TokensCacheRead = res.TokensReasoning, res.TokensCacheRead
		}
	}
	if changed {
		if err := w.d.opts.Sessions.Update(ctx, sess); err != nil {
			slog.Debug("ohd: session update failed", "session", id, "error", err)
		}
	}
}

// usageReader is implemented by adapters that can read usage without the diff.
type usageReader interface {
	Usage(ctx context.Context, h adapters.ServerHandle, sessionID string) (adapters.SessionResult, error)
}

func (w *watcher) usage(ctx context.Context, id string) (adapters.SessionResult, error) {
	if u, ok := w.ad.(usageReader); ok {
		return u.Usage(ctx, w.handle(), id)
	}
	return w.ad.Results(ctx, w.handle(), id)
}
