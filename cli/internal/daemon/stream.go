package daemon

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Live stream: the watchers publish feed items and session changes;
// clients (TUI, `oh session follow`) read them from GET /v1/stream as NDJSON.
// Without ?session=, a client receives the changes of every session (badge,
// Sessions view); with it, also the feed of that session, starting with its
// recent backlog.
//
// Every entry gets a sequence number and is kept in a bounded window (a ring
// of streamWindow entries). A client reads the window at its own pace through
// a cursor, so a slow client loses nothing as long as it stays within the
// window; past it, it receives a resync event (see the package documentation
// for the protocol).

const (
	feedBacklog  = 100
	streamWindow = 1024
	streamPingEv = 15 * time.Second
)

// seqItem is a feed item of the per-session backlog with its sequence number.
type seqItem struct {
	seq  uint64
	item domain.FeedItem
}

// entry is one published event of the window.
type entry struct {
	session string // the session the entry belongs to
	ev      api.StreamEvent
}

// subscriber is one stream reader. cursor (guarded by hub.mu) is the last
// sequence number handed to it; wake tells it that the window grew.
type subscriber struct {
	session string
	cursor  uint64
	wake    chan struct{}
}

type hub struct {
	mu      sync.Mutex
	subs    map[*subscriber]struct{}
	backlog map[string][]seqItem
	ring    []entry // circular window of the last len(ring) entries
	head    int     // index of the oldest entry
	n       int     // entries in the window
	seq     uint64  // sequence number of the newest entry
	closed  chan struct{}
	once    sync.Once
}

func newHub() *hub { return newHubWindow(streamWindow) }

// newHubWindow makes a hub keeping the last window entries. The sequence
// starts at the clock (in microseconds, below 2^53 so that any JSON client reads it exactly), so that it keeps increasing across
// daemon restarts: a client resuming from a previous run is told to resync.
func newHubWindow(window int) *hub {
	return &hub{
		subs:    map[*subscriber]struct{}{},
		backlog: map[string][]seqItem{},
		ring:    make([]entry, max(window, 1)),
		seq:     uint64(time.Now().UnixMicro()),
		closed:  make(chan struct{}),
	}
}

func (h *hub) close() {
	if h != nil {
		h.once.Do(func() { close(h.closed) })
	}
}

// floor is the highest sequence number a client may resume from without
// missing an entry: the one just before the oldest entry of the window.
func (h *hub) floor() uint64 {
	if h.n == 0 {
		return h.seq
	}
	return h.ring[h.head].ev.Seq - 1
}

// matches tells whether an entry is delivered to a subscriber.
func (s *subscriber) matches(e entry) bool {
	if e.ev.Feed != nil {
		return s.session == e.session
	}
	return s.session == "" || s.session == e.session
}

// appendLocked numbers an entry, stores it in the window and wakes the
// readers it concerns. It never blocks.
func (h *hub) appendLocked(session string, ev api.StreamEvent) uint64 {
	h.seq++
	ev.Seq = h.seq
	e := entry{session: session, ev: ev}
	if h.n < len(h.ring) {
		h.ring[(h.head+h.n)%len(h.ring)] = e
		h.n++
	} else {
		h.ring[h.head] = e
		h.head = (h.head + 1) % len(h.ring)
	}
	for s := range h.subs {
		if s.matches(e) {
			select {
			case s.wake <- struct{}{}:
			default: // already woken: it reads the whole window
			}
		}
	}
	return h.seq
}

// catchUpLocked returns what a reader must still see after since, and moves
// its cursor to the newest entry. A reader that cannot continue from since
// (older than the window, or from another daemon run) gets a resync event,
// then the feed backlog of its session (each item with its own Seq, so that
// the client can skip what it already has).
func (h *hub) catchUpLocked(s *subscriber, since uint64, resume bool) []api.StreamEvent {
	var out []api.StreamEvent
	if resume && since >= h.floor() && since <= h.seq {
		for i := 0; i < h.n; i++ {
			if e := h.ring[(h.head+i)%len(h.ring)]; e.ev.Seq > since && s.matches(e) {
				out = append(out, e.ev)
			}
		}
	} else {
		if resume {
			out = append(out, api.StreamEvent{Resync: true, Seq: h.seq})
		}
		for _, b := range h.backlog[s.session] {
			it := b.item
			out = append(out, api.StreamEvent{Feed: &it, Seq: b.seq})
		}
	}
	s.cursor = h.seq
	return out
}

// attach registers a reader and returns its initial replay and cursor. Without
// resume it replays the backlog; with it, the entries after since.
func (h *hub) attach(session string, since uint64, resume bool) (*subscriber, []api.StreamEvent, uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := &subscriber{session: session, wake: make(chan struct{}, 1)}
	evs := h.catchUpLocked(s, since, resume)
	h.subs[s] = struct{}{}
	return s, evs, s.cursor
}

// next returns the entries a reader has not seen yet, and its new cursor.
func (h *hub) next(s *subscriber) (evs []api.StreamEvent, cursor uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	evs = h.catchUpLocked(s, s.cursor, true)
	return evs, s.cursor
}

func (h *hub) unsubscribe(s *subscriber) {
	h.mu.Lock()
	delete(h.subs, s)
	h.mu.Unlock()
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
	seq := h.appendLocked(it.SessionID, api.StreamEvent{Feed: &it})
	b := h.backlog[it.SessionID]
	b = append(b, seqItem{seq: seq, item: it})
	if len(b) > feedBacklog {
		b = b[len(b)-feedBacklog:]
	}
	h.backlog[it.SessionID] = b
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
	h.appendLocked(c.SessionID, api.StreamEvent{Change: &c})
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
	q := r.URL.Query()
	var since uint64
	resume := q.Has(api.StreamSinceParam)
	if resume {
		var err error
		if since, err = strconv.ParseUint(q.Get(api.StreamSinceParam), 10, 64); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid since")
			return
		}
	}
	sub, evs, cursor := d.feed.attach(q.Get("session"), since, resume)
	defer d.feed.unsubscribe(sub)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	for i := range evs {
		if enc.Encode(evs[i]) != nil {
			return
		}
	}
	// End of the initial replay (an empty entry for a client without Seq).
	if enc.Encode(api.StreamEvent{Seq: cursor}) != nil {
		return
	}
	fl.Flush()
	ping := time.NewTicker(streamPingEv)
	defer ping.Stop()
	for {
		pinged := false
		select {
		case <-r.Context().Done():
			return
		case <-d.feed.closed:
			return
		case <-ping.C:
			pinged = true
		case <-sub.wake:
		}
		evs, cursor = d.feed.next(sub)
		if len(evs) == 0 {
			if !pinged {
				continue
			}
			evs = []api.StreamEvent{{Seq: cursor}} // keep-alive, carries the cursor
		}
		for i := range evs {
			if enc.Encode(evs[i]) != nil {
				return
			}
		}
		fl.Flush()
	}
}
