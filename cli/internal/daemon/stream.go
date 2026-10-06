package daemon

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Live stream (P3-T10): the watchers publish feed items and session changes;
// clients (TUI, `oh session follow`) read them from GET /v1/stream as NDJSON.
// Without ?session=, a client receives the changes of every session (badge,
// Sessions view); with it, also the feed of that session, starting with its
// recent backlog.

const (
	feedBacklog  = 100
	subBuffer    = 256
	streamPingEv = 15 * time.Second
)

// StreamEvent is one line of GET /v1/stream (exactly one field is set; an
// empty object is a keep-alive).
type StreamEvent struct {
	Feed   *domain.FeedItem      `json:"feed,omitempty"`
	Change *domain.SessionChange `json:"change,omitempty"`
}

type subscriber struct {
	session string
	ch      chan StreamEvent
}

type hub struct {
	mu      sync.Mutex
	subs    map[*subscriber]struct{}
	backlog map[string][]domain.FeedItem
	closed  chan struct{}
	once    sync.Once
}

func newHub() *hub {
	return &hub{subs: map[*subscriber]struct{}{}, backlog: map[string][]domain.FeedItem{}, closed: make(chan struct{})}
}

func (h *hub) close() {
	if h != nil {
		h.once.Do(func() { close(h.closed) })
	}
}

func (h *hub) subscribe(session string) (*subscriber, []domain.FeedItem) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := &subscriber{session: session, ch: make(chan StreamEvent, subBuffer)}
	h.subs[s] = struct{}{}
	return s, append([]domain.FeedItem(nil), h.backlog[session]...)
}

func (h *hub) unsubscribe(s *subscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
}

// send never blocks: a slow client loses events (it reloads on changes).
func send(s *subscriber, ev StreamEvent) {
	select {
	case s.ch <- ev:
	default:
	}
}

func (h *hub) publishFeed(it domain.FeedItem) {
	if h == nil {
		return
	}
	if it.Time.IsZero() {
		it.Time = time.Now()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.backlog[it.SessionID] = append(h.backlog[it.SessionID], it)
	if b := h.backlog[it.SessionID]; len(b) > feedBacklog {
		h.backlog[it.SessionID] = b[len(b)-feedBacklog:]
	}
	for s := range h.subs {
		if s.session == it.SessionID {
			send(s, StreamEvent{Feed: &it})
		}
	}
}

func (h *hub) publishChange(c domain.SessionChange) {
	if h == nil {
		return
	}
	if c.Time.IsZero() {
		c.Time = time.Now()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		if s.session == "" || s.session == c.SessionID {
			send(s, StreamEvent{Change: &c})
		}
	}
}

// forget drops the backlog of sessions that ended.
func (h *hub) forget(session string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	delete(h.backlog, session)
	h.mu.Unlock()
}

func (d *Daemon) handleStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	sub, backlog := d.feed.subscribe(r.URL.Query().Get("session"))
	defer d.feed.unsubscribe(sub)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	for i := range backlog {
		if enc.Encode(StreamEvent{Feed: &backlog[i]}) != nil {
			return
		}
	}
	_ = enc.Encode(StreamEvent{})
	fl.Flush()
	ping := time.NewTicker(streamPingEv)
	defer ping.Stop()
	for {
		var ev StreamEvent
		select {
		case <-r.Context().Done():
			return
		case <-d.feed.closed:
			return
		case <-ping.C:
		case ev = <-sub.ch:
		}
		if enc.Encode(ev) != nil {
			return
		}
		fl.Flush()
	}
}
