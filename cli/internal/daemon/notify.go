package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
)

// System notifications: the daemon is the only process alive when
// oh is closed, so it notifies — whether a TUI is open or not (the TUI adds
// its own toast). Triggers: a new decision (⏸ ? ! $ ✗ ⊘, whoever raised it)
// and the end of a turn when nobody is attached to the session. Notes are
// grouped over a short window; texts never carry session content.

// NotifyFunc shows one desktop notification.
type NotifyFunc func(ctx context.Context, title, message string) error

const notifyWindow = 3 * time.Second

// reminderMaxAge is how old a decision may be to be recalled to the user when
// the daemon starts.
const reminderMaxAge = 24 * time.Hour

// Kinds of note that are not decision kinds.
const (
	kindDone     = "done"     // the end of a step, nobody attached
	kindSleep    = "sleep"    // a session went to sleep with a decision open
	kindReminder = "reminder" // a decision still open when the daemon started
)

type note struct {
	kind      string // decision kind, "done", "sleep" or "reminder"
	sessionID string
}

type notifier struct {
	d       *Daemon
	send    NotifyFunc
	window  time.Duration
	started time.Time

	mu      sync.Mutex
	seen    map[string]bool // decision ids already notified (or open at start)
	queue   []note
	pending bool // a flush is scheduled
	primed  bool
	scanReq chan struct{}
}

func newNotifier(d *Daemon, send NotifyFunc) *notifier {
	window := d.opts.NotifyWindow
	if window <= 0 {
		window = notifyWindow
	}
	return &notifier{d: d, send: send, window: window, started: time.Now(), seen: map[string]bool{}, scanReq: make(chan struct{}, 1)}
}

// run scans the open decisions when asked (debounced) until ctx ends.
func (n *notifier) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-n.scanReq:
			n.scan(ctx)
		}
	}
}

// requestScan asks for a scan of the open decisions (non-blocking).
func (n *notifier) requestScan() {
	if n == nil {
		return
	}
	select {
	case n.scanReq <- struct{}{}:
	default:
	}
}

// scan queues a note for every decision opened since the last scan. The
// decisions already open when the daemon started are not notified again.
func (n *notifier) scan(ctx context.Context) {
	store := n.d.opts.Decisions
	if store == nil {
		return
	}
	open, err := store.ListOpen(ctx, domain.DecisionFilter{})
	if err != nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	still := map[string]bool{}
	for _, d := range open {
		still[d.ID] = true
		if n.seen[d.ID] {
			continue
		}
		n.seen[d.ID] = true
		if !n.primed && d.CreatedAt.Before(n.started.Add(-time.Minute)) {
			// Open before the daemon started: the user may have missed it, they are
			// reminded once (not for an old one that was left on purpose).
			if time.Since(d.CreatedAt) < reminderMaxAge {
				n.enqueueLocked(note{kind: kindReminder, sessionID: d.SessionID})
			}
			continue
		}
		n.enqueueLocked(note{kind: string(d.Kind), sessionID: d.SessionID})
	}
	for id := range n.seen {
		if !still[id] {
			delete(n.seen, id)
		}
	}
	n.primed = true
}

// turnDone queues an end-of-turn note.
func (n *notifier) turnDone(sessionID string) {
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enqueueLocked(note{kind: kindDone, sessionID: sessionID})
}

// sleeping queues the note of the sessions that went to sleep with a decision
// waiting: the request died with the server, it stays to be taken up.
func (n *notifier) sleeping(sessionIDs []string) {
	if n == nil || len(sessionIDs) == 0 {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, id := range sessionIDs {
		n.enqueueLocked(note{kind: kindSleep, sessionID: id})
	}
}

func (n *notifier) enqueueLocked(nt note) {
	n.queue = append(n.queue, nt)
	if !n.pending {
		n.pending = true
		// The notification command may panic: the wait and flush run under safego.
		safego.Go(func() {
			time.Sleep(n.window)
			n.flush()
		})
	}
}

func (n *notifier) flush() {
	n.mu.Lock()
	queue := n.queue
	n.queue, n.pending = nil, false
	n.mu.Unlock()
	if len(queue) == 0 {
		return
	}
	title, msg := n.render(context.Background(), queue)
	if err := n.send(context.Background(), title, msg); err != nil {
		slog.Debug("ohd: notification failed", "error", err)
	}
}

// render builds the text of a group of notes: one note names its project
// and agent; several are counted.
func (n *notifier) render(ctx context.Context, queue []note) (title, msg string) {
	decisions, done, slept := 0, 0, 0
	sessions := map[string]bool{}
	for _, q := range queue {
		sessions[q.sessionID] = true
		switch q.kind {
		case kindDone:
			done++
		case kindSleep:
			slept++
		default:
			decisions++
		}
	}
	title = "oh"
	if slept > 0 && slept == len(queue) && len(queue) > 1 {
		return title, i18n.Tf("v54n.notify.many_sleeping", slept)
	}
	if len(queue) == 1 {
		q := queue[0]
		key := "tui.notify.title." + q.kind
		switch q.kind {
		case kindSleep, kindReminder:
			key = "v54n.notify.title." + q.kind
		}
		if t := i18n.T(key); t != key {
			title = t
		}
		return title, n.describe(ctx, q.sessionID)
	}
	switch {
	case done == 0:
		return title, i18n.Tf("tui.notify.many_decisions", decisions+slept, len(sessions))
	case decisions+slept == 0:
		return title, i18n.Tf("tui.notify.many_done", done)
	}
	return title, i18n.Tf("tui.notify.mixed", decisions+slept, done)
}

// describe names a session without its content: project · agent.
func (n *notifier) describe(ctx context.Context, sessionID string) string {
	if n.d.opts.Sessions == nil {
		return sessionID
	}
	s, err := n.d.opts.Sessions.Get(ctx, sessionID)
	if err != nil {
		return sessionID
	}
	project := s.ProjectID
	if n.d.opts.ProjectName != nil {
		if name := n.d.opts.ProjectName(ctx, s.ProjectID); name != "" {
			project = name
		}
	}
	if s.EntryAgent == "" {
		return project
	}
	return fmt.Sprintf("%s · %s", project, s.EntryAgent)
}
