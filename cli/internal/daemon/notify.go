package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// System notifications (P3-T13): the daemon is the only process alive when
// oh is closed, so it notifies — whether a TUI is open or not (the TUI adds
// its own toast). Triggers: a new decision (⏸ ? ! $ ✗, whoever raised it)
// and the end of a turn when nobody is attached to the session. Notes are
// grouped over a short window; texts never carry session content.

// NotifyFunc shows one desktop notification.
type NotifyFunc func(ctx context.Context, title, message string) error

const notifyWindow = 3 * time.Second

type note struct {
	kind      string // decision kind, or "done"
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
	timer   *time.Timer
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
	n.enqueueLocked(note{kind: "done", sessionID: sessionID})
}

func (n *notifier) enqueueLocked(nt note) {
	n.queue = append(n.queue, nt)
	if n.timer == nil {
		n.timer = time.AfterFunc(n.window, n.flush)
	}
}

func (n *notifier) flush() {
	n.mu.Lock()
	queue := n.queue
	n.queue, n.timer = nil, nil
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
	decisions, done := 0, 0
	sessions := map[string]bool{}
	for _, q := range queue {
		sessions[q.sessionID] = true
		if q.kind == "done" {
			done++
		} else {
			decisions++
		}
	}
	title = "oh"
	if len(queue) == 1 {
		q := queue[0]
		if t := i18n.T("tui.notify.title." + q.kind); t != "tui.notify.title."+q.kind {
			title = t
		}
		return title, n.describe(ctx, q.sessionID)
	}
	switch {
	case done == 0:
		return title, i18n.Tf("tui.notify.many_decisions", decisions, len(sessions))
	case decisions == 0:
		return title, i18n.Tf("tui.notify.many_done", done)
	}
	return title, i18n.Tf("tui.notify.mixed", decisions, done)
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
