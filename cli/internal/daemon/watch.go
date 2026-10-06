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
	executing    bool
	pending      int            // tool requests waiting (permissions, questions)
	childPending map[string]int // requests of its subagent sessions
	alerts       int            // decisions raised by oh (error, budget)
	lastUsage    time.Time
	agent        string // current agent shown in the live feed
}

// waiting is the number of tool requests of the session and its subagents.
func (t *sessionTrack) waiting() int {
	n := t.pending
	for _, c := range t.childPending {
		n += c
	}
	return n
}

type watcher struct {
	d      *Daemon
	srv    domain.Server
	ad     adapters.ToolAdapter
	cancel context.CancelFunc

	mu         sync.Mutex
	tracks     map[string]*sessionTrack
	known      map[string]bool      // session id → belongs to oh (row in sessions table for this group)
	unknownAt  map[string]time.Time // session id → last negative lookup (short negative cache)
	children   map[string]string    // subagent session id → oh session that delegated it
	childAgent map[string]string    // subagent session id → its agent
	synced     bool                 // a resync succeeded since the last (re)connection
	lastTouch  time.Time
	lastEvent  time.Time // last session activity seen (idle-sleep timer)
	started    time.Time
	done       chan struct{} // closed when run returns
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
		pending += t.waiting()
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
		w := &watcher{d: d, srv: s, ad: ad, cancel: cancel, tracks: map[string]*sessionTrack{}, known: map[string]bool{}, unknownAt: map[string]time.Time{}, children: map[string]string{}, childAgent: map[string]string{}, started: time.Now(), done: make(chan struct{})}
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
	if ev.SessionID == "" {
		return
	}
	if ev.Kind == adapters.EventSessionCreated && ev.ParentID != "" {
		w.adoptChild(ctx, ev.SessionID, ev.ParentID)
	}
	if root, ok := w.rootOf(ev.SessionID); ok {
		w.onChildEvent(ctx, root, ev)
		return
	}
	if !w.isKnown(ctx, ev.SessionID) {
		return
	}
	w.publish(ev.SessionID, ev.Feed)
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
		if ev.Outcome == "succeeded" {
			if attached, _ := w.d.clientState(w.srv.GroupKey); !attached {
				w.d.notes.turnDone(ev.SessionID)
			}
		}
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
	w.adoptChildren(ctx)
	w.mu.Lock()
	w.synced = true
	w.mu.Unlock()
}

// adoptChildren rebuilds the subagent links after a (re)connection (events
// are not replayed) and refreshes the requests the subagents wait for.
func (w *watcher) adoptChildren(ctx context.Context) {
	cl, ok := w.ad.(adapters.ChildLister)
	if !ok {
		return
	}
	parents, err := cl.Children(ctx, w.handle())
	if err != nil {
		return
	}
	// Parents before children: walk until no new link appears.
	for changed := true; changed; {
		changed = false
		for child, parent := range parents {
			if _, done := w.rootOf(child); done {
				continue
			}
			w.adoptChild(ctx, child, parent)
			if _, ok := w.rootOf(child); ok {
				changed = true
			}
		}
	}
	w.mu.Lock()
	links := make(map[string]string, len(w.children))
	for c, r := range w.children {
		links[c] = r
	}
	w.mu.Unlock()
	for child, root := range links {
		w.refreshChildPending(ctx, root, child)
	}
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
	w.syncToolDecisions(ctx, id, id, pending)
	alerts := w.openAlerts(ctx, id)
	w.mu.Lock()
	t := w.track(id)
	t.pending, t.alerts = len(pending), alerts
	w.mu.Unlock()
	w.decisionsChanged(id)
}

func (w *watcher) refreshAlerts(ctx context.Context, id string) {
	alerts := w.openAlerts(ctx, id)
	w.mu.Lock()
	w.track(id).alerts = alerts
	w.mu.Unlock()
	w.decisionsChanged(id)
}

func (w *watcher) decisionsChanged(id string) {
	w.d.feed.publishChange(domain.SessionChange{SessionID: id, GroupKey: w.srv.GroupKey, Decisions: true})
	w.d.notes.requestScan()
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
	case t.waiting() > 0 || t.alerts > 0:
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
			return
		}
		w.d.feed.publishChange(domain.SessionChange{SessionID: id, GroupKey: w.srv.GroupKey, State: sess.State})
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

// ── Subagent (child) sessions ───────────────────────────────────────────────

// adoptChild links a subagent session to the oh session at the root of its
// delegation chain.
func (w *watcher) adoptChild(ctx context.Context, child, parent string) {
	root := parent
	if r, ok := w.rootOf(parent); ok {
		root = r
	}
	if !w.isKnown(ctx, root) {
		return
	}
	w.mu.Lock()
	w.children[child] = root
	w.mu.Unlock()
}

func (w *watcher) rootOf(id string) (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r, ok := w.children[id]
	return r, ok
}

// onChildEvent reports the activity of a subagent on its root session: feed
// entries (with the subagent name) and the requests it waits for.
func (w *watcher) onChildEvent(ctx context.Context, root string, ev adapters.ToolEvent) {
	w.mu.Lock()
	w.lastEvent = time.Now()
	if ev.Feed != nil && ev.Feed.Kind == domain.FeedAgent {
		w.childAgent[ev.SessionID] = ev.Feed.Agent
	}
	agent := w.childAgent[ev.SessionID]
	w.mu.Unlock()
	if it := ev.Feed; it != nil && it.Kind != domain.FeedUsage && it.Kind != domain.FeedState {
		item := *it
		if item.Agent == "" {
			item.Agent = agent
		}
		w.publish(root, &item)
	}
	switch ev.Kind {
	case adapters.EventDecisionAsked, adapters.EventDecisionReplied, adapters.EventExecEnded:
		w.refreshChildPending(ctx, root, ev.SessionID)
	}
}

func (w *watcher) refreshChildPending(ctx context.Context, root, child string) {
	pending, err := w.ad.Pending(ctx, w.handle(), child)
	if err != nil {
		return
	}
	w.syncToolDecisions(ctx, root, child, pending)
	w.mu.Lock()
	t := w.track(root)
	if t.childPending == nil {
		t.childPending = map[string]int{}
	}
	if len(pending) == 0 {
		delete(t.childPending, child)
	} else {
		t.childPending[child] = len(pending)
	}
	w.mu.Unlock()
	w.persist(ctx, root, false)
	w.decisionsChanged(root)
}

// publish sends a feed entry of a tool session to the feed of its oh session.
// Agent entries are shown only when the agent changes.
func (w *watcher) publish(root string, it *domain.FeedItem) {
	if it == nil {
		return
	}
	item := *it
	item.SessionID = root
	if item.Kind == domain.FeedAgent {
		w.mu.Lock()
		t := w.track(root)
		same := t.agent == item.Agent
		t.agent = item.Agent
		w.mu.Unlock()
		if same {
			return
		}
	}
	w.d.feed.publishFeed(item)
}
