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
	unknownTTL = 5 * time.Second
	usageEvery = 5 * time.Second
)

type sessionTrack struct {
	executing bool
	pending   int // tool requests waiting (permissions, questions)
	alerts    int // decisions raised by oh (error, budget)
	lastUsage time.Time
}

type watcher struct {
	d      *Daemon
	srv    domain.Server
	ad     adapters.ToolAdapter
	cancel context.CancelFunc

	mu        sync.Mutex
	tracks    map[string]*sessionTrack
	known     map[string]bool      // session id → belongs to oh (row in sessions table for this group)
	unknownAt map[string]time.Time // session id → last negative lookup (short negative cache)
	synced    bool                 // a resync succeeded since the last (re)connection
	lastTouch time.Time
	lastEvent time.Time // last session activity seen (idle-sleep timer)
	started   time.Time
	done      chan struct{} // closed when run returns
}

// stop cancels the watcher and waits for its goroutine (no write after return).
func (w *watcher) stop(timeout time.Duration) {
	w.cancel()
	select {
	case <-w.done:
	case <-time.After(timeout):
	}
}

// snapshot summarizes the tracked sessions of the group.
func (w *watcher) snapshot() (executing, pending int, lastEvent time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, t := range w.tracks {
		if t.executing {
			executing++
		}
		pending += t.pending
	}
	lastEvent = w.lastEvent
	if lastEvent.IsZero() {
		lastEvent = w.started
	}
	return executing, pending, lastEvent
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
		w := &watcher{d: d, srv: s, ad: ad, cancel: cancel, tracks: map[string]*sessionTrack{}, known: map[string]bool{}, unknownAt: map[string]time.Time{}, started: time.Now(), done: make(chan struct{})}
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
	defer close(w.done)
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
	read:
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-evs:
				if !ok {
					w.mu.Lock()
					w.synced = false
					w.mu.Unlock()
					break read
				}
				w.onEvent(ctx, ev)
			}
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
	w.lastEvent = time.Now()
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

	switch {
	case ev.Kind == adapters.EventExecStarted:
		w.clearAlerts(ctx, ev.SessionID)
	case ev.Kind == adapters.EventExecEnded && ev.Outcome == "failed":
		w.raiseFailure(ctx, ev)
	}
	if refreshPending {
		w.refreshPending(ctx, ev.SessionID)
	} else if ev.Kind == adapters.EventExecStarted {
		w.refreshAlerts(ctx, ev.SessionID)
	}
	w.persist(ctx, ev.SessionID, refreshUsage)
	if ev.Kind == adapters.EventExecEnded {
		w.d.wake() // a pending "sleep when idle" policy may apply now
	}
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
	known := w.known[id]
	at, negative := w.unknownAt[id]
	w.mu.Unlock()
	if known {
		return true
	}
	// Negative answers are cached briefly only: the oh row may be written
	// just after the tool session (or belong to a child session).
	if negative && time.Since(at) < unknownTTL {
		return false
	}
	s, err := w.d.opts.Sessions.Get(ctx, id)
	known = err == nil && s.GroupKey == w.srv.GroupKey
	w.mu.Lock()
	if known {
		w.known[id] = true
		delete(w.unknownAt, id)
	} else {
		w.unknownAt[id] = time.Now()
	}
	w.mu.Unlock()
	return known
}

// resync rebuilds the state of every oh session of the group from the tool.
func (w *watcher) resync(ctx context.Context) {
	sessions, err := w.d.opts.Sessions.List(ctx, w.srv.ProjectID)
	if err != nil {
		return
	}
	ids, err := w.ad.ActiveSessions(ctx, w.handle())
	if err != nil {
		return // not synced: the group must not be put to sleep on guesses
	}
	active := map[string]bool{}
	for _, id := range ids {
		active[id] = true
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
	w.mu.Lock()
	w.synced = true
	w.mu.Unlock()
}

func (w *watcher) isSynced() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.synced
}

func (w *watcher) refreshPending(ctx context.Context, id string) {
	pending, err := w.ad.Pending(ctx, w.handle(), id)
	if err != nil {
		return
	}
	w.syncToolDecisions(ctx, id, pending)
	alerts := w.openAlerts(ctx, id)
	w.mu.Lock()
	t := w.track(id)
	t.pending, t.alerts = len(pending), alerts
	w.mu.Unlock()
}

func (w *watcher) refreshAlerts(ctx context.Context, id string) {
	alerts := w.openAlerts(ctx, id)
	w.mu.Lock()
	w.track(id).alerts = alerts
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
	case t.pending > 0 || t.alerts > 0:
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
	if changed {
		now := time.Now()
		sess.StateChangedAt = &now
	}
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
