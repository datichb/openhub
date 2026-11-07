package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
)

func feedText(h *hub, session string, n int) {
	for i := range n {
		h.publishFeed(domain.FeedItem{SessionID: session, Kind: domain.FeedText, Text: fmt.Sprintf("t%d", i)})
	}
}

func seqs(evs []api.StreamEvent) []uint64 {
	out := make([]uint64, 0, len(evs))
	for _, ev := range evs {
		out = append(out, ev.Seq)
	}
	return out
}

func TestHubNumbersEntriesAndFiltersThem(t *testing.T) {
	h := newHubWindow(16)
	all, _, _ := h.attach("", 0, false)
	one, _, _ := h.attach("a", 0, false)
	start := h.seq

	h.publishFeed(domain.FeedItem{SessionID: "a", Kind: domain.FeedText, Text: "x"})
	h.publishChange(domain.SessionChange{SessionID: "a"})
	h.publishChange(domain.SessionChange{SessionID: "b"})
	h.publishFeed(domain.FeedItem{SessionID: "b", Kind: domain.FeedText, Text: "y"})

	evs, cursor := h.next(all)
	assert.Equal(t, []uint64{start + 2, start + 3}, seqs(evs), "all sessions: changes only")
	assert.Equal(t, start+4, cursor)
	evs, _ = h.next(one)
	assert.Equal(t, []uint64{start + 1, start + 2}, seqs(evs), "one session: its feed and its changes")
	require.NotNil(t, evs[0].Feed)
	require.NotNil(t, evs[1].Change)

	evs, _ = h.next(one)
	assert.Empty(t, evs, "nothing is delivered twice")
}

// A slow client (it does not read while 8 entries arrive in a window of 8)
// loses nothing: it reads them all, in order, without resync.
func TestHubSlowReaderLosesNothingWithinTheWindow(t *testing.T) {
	h := newHubWindow(8)
	s, _, _ := h.attach("a", 0, false)
	start := h.seq
	feedText(h, "a", 8)
	evs, cursor := h.next(s)
	require.Len(t, evs, 8)
	for i, ev := range evs {
		assert.False(t, ev.Resync)
		assert.Equal(t, start+uint64(i)+1, ev.Seq)
		assert.Equal(t, fmt.Sprintf("t%d", i), ev.Feed.Text)
	}
	assert.Equal(t, start+8, cursor)
}

// Past the window, the reader is told to resync, then gets the backlog of its
// session with the original sequence numbers.
func TestHubSlowReaderGetsResyncBeyondTheWindow(t *testing.T) {
	h := newHubWindow(8)
	s, _, _ := h.attach("a", 0, false)
	start := h.seq
	feedText(h, "a", 9)
	evs, cursor := h.next(s)
	require.Len(t, evs, 10)
	assert.True(t, evs[0].Resync)
	assert.Equal(t, start+9, evs[0].Seq)
	for i, ev := range evs[1:] {
		assert.Equal(t, start+uint64(i)+1, ev.Seq, "the backlog keeps the original numbers")
	}
	assert.Equal(t, start+9, cursor)
	more, _ := h.next(s)
	assert.Empty(t, more)
}

func TestHubResume(t *testing.T) {
	h := newHubWindow(8)
	start := h.seq
	feedText(h, "a", 5) // start+1 .. start+5

	_, evs, cursor := h.attach("a", start+2, true)
	assert.Equal(t, []uint64{start + 3, start + 4, start + 5}, seqs(evs))
	assert.Equal(t, start+5, cursor)

	_, evs, _ = h.attach("a", start+5, true)
	assert.Empty(t, evs, "up to date")

	_, evs, _ = h.attach("a", start, true)
	assert.Len(t, evs, 5, "the oldest entry is still in the window")
	assert.False(t, evs[0].Resync)

	feedText(h, "a", 4) // start+1 is out of the window
	_, evs, _ = h.attach("a", start, true)
	require.NotEmpty(t, evs)
	assert.True(t, evs[0].Resync, "just before the window: entries are lost")
	_, evs, _ = h.attach("a", start+1, true)
	assert.False(t, evs[0].Resync)

	_, evs, _ = h.attach("a", h.seq+1, true)
	assert.True(t, evs[0].Resync, "a number from another run (restart)")

	// Without since: the backlog, no resync.
	_, evs, _ = h.attach("a", 0, false)
	assert.Len(t, evs, 9)
	assert.False(t, evs[0].Resync)
}

// A client resuming from a previous run of the daemon is told to resync,
// whether the new run is ahead of its number or behind it.
func TestHubResumeAcrossARestart(t *testing.T) {
	old := newHub()
	feedText(old, "a", 3)
	resume := old.seq
	for _, base := range []uint64{resume + 5000, resume - 5000} {
		fresh := newHubWindow(8)
		fresh.seq = base
		_, evs, _ := fresh.attach("a", resume, true)
		require.NotEmpty(t, evs, "base %d", base)
		assert.True(t, evs[0].Resync, "base %d", base)
	}
}

func TestHubForgetKeepsTheWindow(t *testing.T) {
	h := newHubWindow(8)
	feedText(h, "a", 3)
	h.forget("a")
	_, evs, _ := h.attach("a", 0, false)
	assert.Empty(t, evs)
}

func TestHubWakesReaders(t *testing.T) {
	h := newHubWindow(8)
	s, _, _ := h.attach("a", 0, false)
	feedText(h, "b", 1)
	select {
	case <-s.wake:
		t.Fatal("woken by another session")
	default:
	}
	feedText(h, "a", 2)
	select {
	case <-s.wake:
	default:
		t.Fatal("not woken")
	}
}

func streamServer(t *testing.T, h *hub) *httptest.Server {
	t.Helper()
	d := &Daemon{feed: h}
	srv := httptest.NewServer(http.HandlerFunc(d.handleStream))
	t.Cleanup(srv.Close)
	return srv
}

func readEvents(t *testing.T, srv *httptest.Server, query string, want int) []api.StreamEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/stream"+query, http.NoBody)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	sc := bufio.NewScanner(resp.Body)
	var out []api.StreamEvent
	for len(out) < want && sc.Scan() {
		var ev api.StreamEvent
		require.NoError(t, json.Unmarshal(sc.Bytes(), &ev))
		out = append(out, ev)
	}
	return out
}

// The HTTP side: since resumes, the end of the replay carries the cursor, a
// bad since is refused.
func TestHandleStreamSince(t *testing.T) {
	h := newHubWindow(8)
	start := h.seq
	feedText(h, "a", 4)
	srv := streamServer(t, h)

	evs := readEvents(t, srv, fmt.Sprintf("?session=a&since=%d", start+2), 3)
	require.Len(t, evs, 3)
	assert.Equal(t, start+3, evs[0].Seq)
	assert.Equal(t, "t2", evs[0].Feed.Text)
	assert.Equal(t, start+4, evs[1].Seq)
	assert.Nil(t, evs[2].Feed, "end of the replay")
	assert.Equal(t, start+4, evs[2].Seq)

	evs = readEvents(t, srv, "?session=a", 5)
	require.Len(t, evs, 5)
	assert.Equal(t, start+4, evs[4].Seq)

	evs = readEvents(t, srv, fmt.Sprintf("?session=a&since=%d", start-1), 6)
	require.Len(t, evs, 6)
	assert.True(t, evs[0].Resync)

	resp, err := srv.Client().Get(srv.URL + "/v1/stream?since=abc")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// A live entry published after the replay reaches the connected client.
func TestHandleStreamLive(t *testing.T) {
	h := newHubWindow(8)
	srv := streamServer(t, h)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/v1/stream", http.NoBody)
	require.NoError(t, err)
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	require.True(t, sc.Scan()) // end of the replay
	h.publishChange(domain.SessionChange{SessionID: "z", State: domain.RunIdle})
	require.True(t, sc.Scan())
	var ev api.StreamEvent
	require.NoError(t, json.Unmarshal(sc.Bytes(), &ev))
	require.NotNil(t, ev.Change)
	assert.Equal(t, "z", ev.Change.SessionID)
	assert.NotZero(t, ev.Seq)
}
