package daemon

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/sessionctx"
)

// Session tracking (E10): one watcher per live tool server subscribes to the
// tool event stream and keeps the oh sessions table up to date (run state,
// cost, tokens). The stream is live-only, so each (re)connection starts with a
// resynchronization from the tool list endpoints.

const (
	touchEvery = 10 * time.Second
	usageEvery = 5 * time.Second
)

// unknownTTL is how long a session unknown to oh stays unknown (a variable
// for the tests).
var unknownTTL = 5 * time.Second

type sessionTrack struct {
	executing    bool
	pending      int            // tool requests waiting (permissions, questions)
	childPending map[string]int // requests of its subagent sessions
	alerts       int            // decisions raised by oh (error, budget)
	// held: a step was interrupted while a budget decision was open; it is
	// resumed once the decision is answered (A22).
	held bool
	// deferred are the session state writes kept for the end of the step.
	deferred  []func(ctx context.Context)
	lastUsage time.Time
	agent     string // current agent shown in the live feed
	// own is the agent of the session's own messages (its subagents aside).
	own string
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
	callAgent  map[string]string    // subagent tool call id → delegated agent (checkpoints)
	// shellChecked lists the sessions whose shell was checked (A16).
	shellChecked map[string]bool
	runs         map[string]*agentRun // tool session id → agent telemetry (agent_events)
	ctxw         *sessionctx.Writer   // evolving session state (S8)
	synced       bool                 // a resync succeeded since the last (re)connection
	lastTouch    time.Time
	lastEvent    time.Time // last session activity seen (idle-sleep timer)
	started      time.Time
	done         chan struct{} // closed when run returns

	// childUsageAt: last usage read of each subagent session (childUsageDue).
	childUsageAt map[string]time.Time

	// childGrantEnded: subagents whose gateway token was revoked at the end
	// of their run (given again when they resume, S-14).
	childGrantEnded map[string]bool
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
		d.startWatcher(wctx, w)
	}
	for gk, w := range d.watchers {
		if !keep[gk] {
			w.cancel()
			delete(d.watchers, gk)
		}
	}
}

// startWatcher runs the watcher of a group in the background. A panic of its
// loop is logged and the loop restarted (it resynchronizes at each
// connection): a group whose watcher died would no longer be followed
// (state, cost, decisions, idle sleep) until the daemon restarts, and
// stopping the daemon for it would cut every other group's proxy. The
// daemon waits for the watchers when it stops (stopWatchers).
func (d *Daemon) startWatcher(ctx context.Context, w *watcher) {
	d.watchWG.Add(1)
	safego.Go(func() {
		defer d.watchWG.Done()
		defer close(w.done)
		safego.Supervise(ctx, "ohd watcher "+w.srv.GroupKey, w.run)
	})
}

// watchersStopWait bounds the wait for the watchers when the daemon stops:
// one blocked on a tool server that does not answer must not hold it.
const watchersStopWait = 5 * time.Second

// stopWatchers cancels every watcher and waits (at most watchersStopWait)
// until they have returned, so that none writes to the stores or the feed
// once the daemon is stopped.
func (d *Daemon) stopWatchers() {
	d.wmu.Lock()
	for gk, w := range d.watchers {
		w.cancel()
		delete(d.watchers, gk)
	}
	d.wmu.Unlock()
	// Not under wmu: a watcher finishing its event may need it.
	wait := d.watchersWait
	if wait <= 0 {
		wait = watchersStopWait
	}
	done := make(chan struct{})
	safego.Go(func() {
		d.watchWG.Wait()
		close(done)
	})
	select {
	case <-done:
	case <-time.After(wait):
		slog.Warn("ohd: watchers still running at exit", "waited", wait)
	}
}

// run follows the event stream of the group until ctx ends. Its caller
// (startWatcher) closes w.done once run is over for good.
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
	if ev.Answer != nil {
		w.onAnswer(ctx, ev)
	}
	if root, ok := w.rootOf(ev.SessionID); ok {
		w.onChildEvent(ctx, root, ev)
		return
	}
	if !w.isKnown(ctx, ev.SessionID) {
		return
	}
	w.mu.Lock()
	checked := w.shellChecked[ev.SessionID]
	w.mu.Unlock()
	if !checked {
		safego.Go(func() { w.checkShellOnce(context.WithoutCancel(ctx), ev.SessionID) })
	}
	if ev.Feed != nil && ev.Feed.Kind == domain.FeedAgent && ev.Feed.Agent != "" {
		w.mu.Lock()
		w.track(ev.SessionID).own = ev.Feed.Agent
		w.mu.Unlock()
	}
	w.publish(ev.SessionID, ev.Feed)
	if ev.Call != nil {
		w.onCall(ctx, ev.SessionID, ev.Call)
	}
	w.onAgentActivity(ctx, ev.SessionID, ev.SessionID, ev)
	if ev.Kind == adapters.EventUserInput {
		w.onUserInput(ctx, ev.SessionID)
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
		w.reopenCompleted(ctx, ev.SessionID)
		w.d.ensureFanoutDriver(ctx, ev.SessionID) // p6-debts
		if !w.holdOverBudget(ctx, ev.SessionID, ev.SessionID) {
			w.clearAlerts(ctx, ev.SessionID)
			w.syncBudget(ctx, ev.SessionID, false) // after a raise
			w.mu.Lock()
			w.track(ev.SessionID).held = false // going on: nothing to resume
			w.mu.Unlock()
		}
	case ev.Kind == adapters.EventExecEnded && ev.Outcome == "failed":
		w.mu.Lock()
		held := w.track(ev.SessionID).held
		w.mu.Unlock()
		if !held { // a step interrupted by oh for its budget is no error
			w.raiseFailure(ctx, ev)
		}
	}
	if refreshPending {
		w.refreshPending(ctx, ev.SessionID)
	} else if ev.Kind == adapters.EventExecStarted {
		w.refreshAlerts(ctx, ev.SessionID)
	}
	w.persist(ctx, ev.SessionID, refreshUsage)
	if ev.Kind == adapters.EventExecEnded && ev.Outcome != "failed" {
		w.finishWorkflow(ctx, ev.SessionID)
	}
	if ev.Kind == adapters.EventExecEnded {
		w.d.ensureFanoutDriver(ctx, ev.SessionID) // p6-debts: the step is over, the driver may have ended with it
		w.flushDeferred(ctx, ev.SessionID)
		w.raiseBudget(ctx, ev.SessionID, true) // the step is over: a dismissed decision is asked again
		w.clearResume(ctx, ev.SessionID)
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
		w.d.ensureFanoutDriver(ctx, s.ID)
	}
	w.adoptChildren(ctx)
	w.accountChildren(ctx)
	w.applyAllRules(ctx, sessions)
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

// accountChildren reads the cost of every subagent session of the working
// oh sessions, running or ended, and records what the ledger does not hold
// yet, then checks the budgets: a subagent that ended while the daemon was
// not listening (reconnection, restart, sleep) would otherwise never be
// counted, and subagents carry most of the cost of a session. account
// records the difference with what it already holds: a subagent counted
// before is not counted twice.
func (w *watcher) accountChildren(ctx context.Context) {
	if w.d.opts.Usage == nil || w.d.opts.Sessions == nil {
		return
	}
	w.mu.Lock()
	links := make(map[string]string, len(w.children))
	for c, r := range w.children {
		links[c] = r
	}
	w.mu.Unlock()
	children := make([]string, 0, len(links))
	for c := range links {
		children = append(children, c)
	}
	sort.Strings(children)
	working := map[string]bool{} // root → not ended
	var roots []string
	for _, child := range children {
		root := links[child]
		live, seen := working[root]
		if !seen {
			sess, err := w.d.opts.Sessions.Get(ctx, root)
			live = err == nil && !isTerminal(sess.State)
			working[root] = live
			if live {
				roots = append(roots, root)
			}
		}
		if !live {
			continue
		}
		if res, err := w.usage(ctx, child); err == nil {
			w.account(ctx, root, child, res)
		}
	}
	for _, root := range roots {
		w.storeTotal(ctx, root)
		w.raiseBudget(ctx, root, false)
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
	if sess.State == domain.RunQueued && state == domain.RunIdle {
		state = domain.RunQueued // its first prompt waits for a slot
	}
	changed := sess.State != state
	if changed {
		now := time.Now()
		sess.StateChangedAt = &now
	}
	sess.State = state
	if withUsage {
		if res, err := w.usage(ctx, id); err == nil {
			w.account(ctx, id, id, res)
			res = w.sessionTotal(ctx, id, res)
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
	_, had := w.children[child]
	w.children[child] = root
	w.mu.Unlock()
	if !had {
		w.applyRulesTo(ctx, root, child) // locks hold in nested delegations too
		safego.Go(func() { w.applyChildEnv(child, root) })
	}
}

func (w *watcher) rootOf(id string) (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r, ok := w.children[id]
	return r, ok
}

// adoptNow links a subagent session to its oh session from the tool's list
// of sessions, ancestors first, when its creation event was not handled yet
// (event just emitted, stream lost, daemon restarted): its session rules
// are set at once. ok is false when id is no descendant of an oh session.
func (w *watcher) adoptNow(ctx context.Context, id string) (string, bool) {
	if r, ok := w.rootOf(id); ok {
		return r, true
	}
	cl, ok := w.ad.(adapters.ChildLister)
	if !ok {
		return "", false
	}
	parents, err := cl.Children(ctx, w.handle())
	if err != nil {
		return "", false
	}
	var chain []string
	for cur := id; len(chain) <= maxAdoptDepth; {
		p, ok := parents[cur]
		if !ok {
			break
		}
		chain = append(chain, cur)
		if _, linked := w.rootOf(p); linked || w.isKnown(ctx, p) {
			break
		}
		cur = p
	}
	for i := len(chain) - 1; i >= 0; i-- {
		w.adoptChild(ctx, chain[i], parents[chain[i]])
	}
	return w.rootOf(id)
}

// maxAdoptDepth bounds the delegation chain walked by adoptNow.
const maxAdoptDepth = 32

// adoptForHook returns the oh session of a tool session that asks about a
// held request (p6-debts): the session itself when oh knows it, else its
// root once adopted, retried for a few seconds (the tool may list a new
// session a moment after its first call); id when none.
func (d *Daemon) adoptForHook(ctx context.Context, id, group string) string {
	if s, err := d.opts.Sessions.Get(ctx, id); err == nil && s.GroupKey == group {
		return id
	}
	d.wmu.Lock()
	w := d.watchers[group]
	d.wmu.Unlock()
	if w == nil {
		return id
	}
	for i := 0; ; i++ {
		if r, ok := w.adoptNow(ctx, id); ok {
			return r
		}
		if i+1 >= adoptTries {
			return id
		}
		select {
		case <-ctx.Done():
			return id
		case <-time.After(adoptRetry):
		}
	}
}

// adoptTries and adoptRetry bound the wait of adoptForHook (under the
// timeout of the plugin for a held request).
var (
	adoptTries = 5
	adoptRetry = 600 * time.Millisecond
)

// onChildEvent reports the activity of a subagent on its root session: feed
// entries (with the subagent name) and the requests it waits for.
func (w *watcher) onChildEvent(ctx context.Context, root string, ev adapters.ToolEvent) {
	if ev.Call != nil {
		w.onCall(ctx, root, ev.Call)
	}
	w.onAgentActivity(ctx, root, ev.SessionID, ev)
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
	switch ev.Kind {
	case adapters.EventExecStarted:
		w.holdOverBudget(ctx, root, ev.SessionID)
		w.resumeChildGrant(root, ev.SessionID)
	case adapters.EventUsage:
		if w.childUsageDue(ev.SessionID) {
			w.accountChild(ctx, root, ev.SessionID)
			w.raiseBudget(ctx, root, false) // F-04: not at the end of the root step only
		}
	case adapters.EventExecEnded:
		w.accountChild(ctx, root, ev.SessionID) // subagent cost counts for the oh session
		w.raiseBudget(ctx, root, false)
		w.endChildGrant(root, ev.SessionID)
	}
}

// childUsageEvery spaces the usage reads of a running subagent session.
var childUsageEvery = time.Second

// childUsageDue reports whether the usage of a subagent session is read now
// (its usage events come after each model step).
func (w *watcher) childUsageDue(child string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.childUsageAt == nil {
		w.childUsageAt = map[string]time.Time{}
	}
	if time.Since(w.childUsageAt[child]) < childUsageEvery {
		return false
	}
	w.childUsageAt[child] = time.Now()
	return true
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
