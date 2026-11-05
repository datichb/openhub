package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notifypolicy"
	"github.com/datichb/openhub/cli/internal/safego"
)

// System notifications: the daemon is the only process alive when
// oh is closed, so it notifies — whether a TUI is open or not (the TUI adds
// its own toast). Triggers: a new decision (⏸ ? ! $ ✗ ⊘, whoever raised it),
// the end of a turn when nobody is attached to the session, a session put to
// sleep with a decision waiting, a workflow that finished, an alert that only
// the user can fix. What reaches the user is theirs to choose
// (notifypolicy: [notifications] of hub.toml, read again at each notification):
// per kind a system notification, a toast only, or nothing; a quiet period; the
// sound. Notes are grouped over a short window, one notification per family so
// that one does not replace another in the notification centre. Texts never
// carry session content.

// Notification is one desktop notification.
type Notification struct {
	Title, Message string
	// Group replaces the previous notification of the same group (one group
	// per family: a budget does not replace a decision).
	Group string
	Sound bool
}

// NotifyFunc shows one desktop notification.
type NotifyFunc func(ctx context.Context, n Notification) error

const notifyWindow = 3 * time.Second

// reminderMaxAge is how old a decision may be to be recalled to the user when
// the daemon starts.
const reminderMaxAge = 24 * time.Hour

// Windows of the memory of what was notified.
const (
	errorEvery  = 10 * time.Minute // one notification per session failing in a row
	alertEvery  = 24 * time.Hour   // one notification per alert key
	budgetEvery = 24 * time.Hour   // one per session and allowance
	recentDone  = 30 * time.Second // a step that ended a workflow is not announced twice
)

// Families of notes: one notification (one group) each.
const (
	classDecision = "decision"
	classBudget   = "budget"
	classError    = "error"
	classAlert    = "alert" // + ":" + key: one notification per key
	classDone     = "done"
	classSleep    = "sleep"
	classComplete = "complete"
	classRemind   = "reminder"
	classDirect   = "direct" // a text already written (memory cap, remote session)
)

type note struct {
	kind      notifypolicy.Kind
	class     string
	sessionID string
	title     string // a title written by whoever raised the alert
	message   string
	reminder  bool
}

type notifier struct {
	d       *Daemon
	send    NotifyFunc
	policy  func() notifypolicy.Settings
	window  time.Duration
	started time.Time
	now     func() time.Time

	mu       sync.Mutex
	seen     map[string]bool      // decision ids already notified (or open at start)
	said     map[string]time.Time // dedup keys → when they were notified
	complete map[string]time.Time // sessions whose workflow just ended
	queue    []note
	pending  bool // a flush is scheduled
	primed   bool
	scanReq  chan struct{}
}

func newNotifier(d *Daemon, send NotifyFunc) *notifier {
	window := d.opts.NotifyWindow
	if window <= 0 {
		window = notifyWindow
	}
	policy := d.opts.NotifyPolicy
	if policy == nil {
		policy = func() notifypolicy.Settings { return notifypolicy.Settings{} }
	}
	return &notifier{d: d, send: send, policy: policy, window: window, started: time.Now(), now: time.Now,
		seen: map[string]bool{}, said: map[string]time.Time{}, complete: map[string]time.Time{}, scanReq: make(chan struct{}, 1)}
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

// once reports whether key was not notified within ttl, and remembers it.
// Must be called under n.mu.
func (n *notifier) once(key string, ttl time.Duration) bool {
	now := n.now()
	if at, ok := n.said[key]; ok && now.Sub(at) < ttl {
		return false
	}
	n.said[key] = now
	for k, at := range n.said { // the memory does not grow for ever
		if now.Sub(at) > 48*time.Hour {
			delete(n.said, k)
		}
	}
	return true
}

// scan queues a note for every decision opened since the last scan. The
// decisions already open when the daemon started are recalled once, together,
// when they are recent enough to matter (the user may have missed them).
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
		nt, ok := n.noteOf(d)
		if !ok {
			continue
		}
		if !n.primed && d.CreatedAt.Before(n.started.Add(-time.Minute)) {
			if time.Since(d.CreatedAt) < reminderMaxAge {
				nt.reminder, nt.class = true, classRemind
				n.enqueueLocked(nt)
			}
			continue
		}
		n.enqueueLocked(nt)
	}
	for id := range n.seen {
		if !still[id] {
			delete(n.seen, id)
		}
	}
	n.primed = true
}

// noteOf is the note of a new decision, if it is to be notified once more:
// a dismissed budget asked again after a step, a failure repeated, the same
// credential refused for several sessions are told once.
func (n *notifier) noteOf(d domain.Decision) (note, bool) {
	nt := note{kind: notifypolicy.Kind(d.Kind), class: classDecision, sessionID: d.SessionID}
	switch d.Kind {
	case domain.DecisionBudget:
		nt.class = classBudget
		return nt, n.once("budget:"+d.ID, budgetEvery)
	case domain.DecisionError:
		if isOhAlert(d) {
			key := d.ToolRef
			if key == "" {
				key = d.ID
			}
			nt.kind, nt.class = notifypolicy.KindCredential, classAlert+":"+key
			nt.title = d.Payload.Title
			nt.message = shorten(firstLine(d.Payload.Message), 140)
			return nt, n.once("alert:"+key, alertEvery)
		}
		nt.class = classError
		return nt, n.once("error:"+d.SessionID, errorEvery)
	}
	return nt, true
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

func shorten(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:limit-1])) + "…"
}

// turnDone queues an end-of-turn note, unless that turn ended the workflow
// (the session is announced as finished).
func (n *notifier) turnDone(sessionID string) {
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if at, ok := n.complete[sessionID]; ok && n.now().Sub(at) < recentDone {
		return
	}
	n.enqueueLocked(note{kind: notifypolicy.KindStepDone, class: classDone, sessionID: sessionID})
}

// sessionComplete queues the note of a workflow that finished.
func (n *notifier) sessionComplete(sessionID string) {
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.complete[sessionID] = n.now()
	n.enqueueLocked(note{kind: notifypolicy.KindSessionComplete, class: classComplete, sessionID: sessionID})
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
		n.enqueueLocked(note{kind: notifypolicy.KindSleep, class: classSleep, sessionID: id})
	}
}

// direct queues a text written by its raiser (the memory cap).
func (n *notifier) direct(kind notifypolicy.Kind, title, message string) {
	if n == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.enqueueLocked(note{kind: kind, class: classDirect + ":" + string(kind), title: title, message: message})
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
	ctx := context.Background()
	p := n.policy()
	if !p.SystemOn() || p.Quiet(n.now()) {
		return // off, or quiet hours: the toasts of the TUI stay
	}
	var classes []string
	by := map[string][]note{}
	for _, q := range queue {
		if !n.wanted(ctx, p, q) {
			continue
		}
		if _, ok := by[q.class]; !ok {
			classes = append(classes, q.class)
		}
		by[q.class] = append(by[q.class], q)
	}
	for _, class := range classes {
		title, msg := n.render(ctx, by[class])
		out := Notification{Title: title, Message: msg, Group: "oh-" + strings.SplitN(class, ":", 2)[0], Sound: p.SoundOn()}
		if err := n.send(ctx, out); err != nil {
			slog.Debug("ohd: notification failed", "error", err)
		}
	}
}

// wanted reports whether the user wants a system notification for a note: the
// kind is set to « system », and the session is not one a terminal is attached
// to (they are looking at it) nor one whose TUI is open, unless they asked.
func (n *notifier) wanted(ctx context.Context, p notifypolicy.Settings, q note) bool {
	if !p.SystemFor(q.kind) {
		return false
	}
	if q.sessionID == "" || n.d == nil || n.d.opts.Sessions == nil {
		return true
	}
	s, err := n.d.opts.Sessions.Get(ctx, q.sessionID)
	if err != nil || s.GroupKey == "" {
		return true
	}
	attached, presence := n.d.clientState(s.GroupKey)
	if attached && !p.WhenAttached {
		return false
	}
	return !presence || p.TUIFocusedOn()
}

// render builds the text of a group of notes of one family: one note names its
// project and agent; several are counted.
func (n *notifier) render(ctx context.Context, queue []note) (title, msg string) {
	first := queue[0]
	title = "oh"
	if first.title != "" && (len(queue) == 1 || strings.HasPrefix(first.class, classAlert)) {
		// An alert says its cause: « Clé Bedrock refusée par AWS ».
		msg = first.message
		if msg == "" {
			msg = n.describe(ctx, first.sessionID)
		}
		return first.title, msg
	}
	sessions := map[string]bool{}
	for _, q := range queue {
		sessions[q.sessionID] = true
	}
	if len(queue) == 1 {
		key := "tui.notify.title." + string(first.kind)
		switch {
		case first.reminder:
			key = "v54n.notify.title.reminder"
		case first.kind == notifypolicy.KindSleep || first.kind == notifypolicy.KindSessionComplete || first.kind == notifypolicy.KindCredential:
			key = "v54n.notify.title." + string(first.kind)
		case first.kind == notifypolicy.KindStepDone:
			key = "tui.notify.title.done"
		}
		if t := i18n.T(key); t != key {
			title = t
		}
		if first.message != "" {
			return title, first.message
		}
		return title, n.describe(ctx, first.sessionID)
	}
	switch first.class {
	case classDone:
		return title, i18n.Tf("tui.notify.many_done", len(queue))
	case classSleep:
		return title, i18n.Tf("v54n.notify.many_sleeping", len(queue))
	case classComplete:
		return title, i18n.Tf("v54n.notify.many_complete", len(queue))
	case classBudget:
		return i18n.T("tui.notify.title.budget"), i18n.Tf("v54n.notify.many_budget", len(queue))
	case classError:
		return i18n.T("tui.notify.title.error"), i18n.Tf("v54n.notify.many_errors", len(queue))
	}
	return title, i18n.Tf("tui.notify.many_decisions", len(queue), len(sessions))
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
