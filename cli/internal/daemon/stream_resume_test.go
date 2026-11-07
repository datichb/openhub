//go:build unix

package daemon

import (
	"context"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	dclient "github.com/datichb/openhub/cli/internal/daemon/client"
	"github.com/datichb/openhub/cli/internal/domain"
)

// streamPeer serves the stream of a hub on a Unix socket and lets a test cut
// the connection, then hold back the next one until it is released: the client
// is "away" while the test publishes.
type streamPeer struct {
	mu    sync.Mutex
	conns int
	cut   context.CancelFunc
	ended chan struct{}
	gate  chan struct{} // closed = connections are served
}

func newStreamPeer(t *testing.T, h *hub) (*dclient.Client, *streamPeer) {
	t.Helper()
	p := shortPaths(t)
	require.NoError(t, os.Chmod(p.Dir, 0o700))
	l, err := net.Listen("unix", p.Socket())
	require.NoError(t, err)
	require.NoError(t, os.Chmod(p.Socket(), 0o600))
	sp := &streamPeer{gate: make(chan struct{})}
	close(sp.gate)
	d := &Daemon{feed: h}
	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sp.mu.Lock()
		gate := sp.gate
		sp.mu.Unlock()
		select {
		case <-gate:
		case <-r.Context().Done():
			return
		}
		ctx, cut := context.WithCancel(r.Context())
		ended := make(chan struct{})
		sp.mu.Lock()
		sp.conns++
		sp.cut, sp.ended = cut, ended
		sp.mu.Unlock()
		defer close(ended)
		d.handleStream(w, r.WithContext(ctx))
	})}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return dclient.NewClient(p), sp
}

// drop cuts the current connection and holds the next one back until release.
func (sp *streamPeer) drop() (release func()) {
	sp.mu.Lock()
	cut, ended := sp.cut, sp.ended
	sp.gate = make(chan struct{})
	gate := sp.gate
	sp.mu.Unlock()
	cut()
	<-ended
	return func() { close(gate) }
}

func (sp *streamPeer) connections() int {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return sp.conns
}

func nextEvent(t *testing.T, ch <-chan api.StreamEvent) api.StreamEvent {
	t.Helper()
	select {
	case ev, ok := <-ch:
		require.True(t, ok, "the stream closed")
		return ev
	case <-time.After(10 * time.Second):
		t.Fatal("no event")
		return api.StreamEvent{}
	}
}

// A client that is away for a while, within the window, gets back every entry
// once and in order, without a resync.
func TestStreamResumeWithinTheWindowLosesNothing(t *testing.T) {
	h := newHubWindow(16)
	c, sp := newStreamPeer(t, h)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := c.Stream(ctx, "a")
	require.NoError(t, err)

	feedText(h, "a", 2)
	var texts []string
	for len(texts) < 2 {
		if ev := nextEvent(t, ch); ev.Feed != nil {
			texts = append(texts, ev.Feed.Text)
		}
	}
	release := sp.drop()
	h.publishFeed(domain.FeedItem{SessionID: "a", Kind: domain.FeedText, Text: "t2"})
	h.publishChange(domain.SessionChange{SessionID: "a", State: domain.RunIdle})
	h.publishFeed(domain.FeedItem{SessionID: "a", Kind: domain.FeedText, Text: "t3"})
	release()

	var changes int
	for len(texts) < 4 || changes < 1 {
		ev := nextEvent(t, ch)
		assert.False(t, ev.Resync, "nothing was lost: no resync")
		switch {
		case ev.Feed != nil:
			texts = append(texts, ev.Feed.Text)
		case ev.Change != nil:
			changes++
		}
	}
	assert.Equal(t, []string{"t0", "t1", "t2", "t3"}, texts)
	assert.Equal(t, 2, sp.connections())
}

// Away for longer than the window: the client is told to resync, and the
// backlog brings the entries back once, without repeating those it had.
func TestStreamResumeBeyondTheWindowResyncs(t *testing.T) {
	h := newHubWindow(16)
	c, sp := newStreamPeer(t, h)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := c.Stream(ctx, "a")
	require.NoError(t, err)

	feedText(h, "a", 2)
	for seen := 0; seen < 2; {
		if nextEvent(t, ch).Feed != nil {
			seen++
		}
	}
	release := sp.drop()
	for i := 2; i < 40; i++ { // 38 entries in a window of 16
		h.publishFeed(domain.FeedItem{SessionID: "a", Kind: domain.FeedText, Text: "x"})
	}
	release()

	ev := nextEvent(t, ch)
	require.True(t, ev.Resync, "past the window")
	n := 0
	for n < 38 {
		ev := nextEvent(t, ch)
		require.NotNil(t, ev.Feed)
		assert.False(t, ev.Resync)
		n++
	}
	select {
	case ev := <-ch:
		t.Fatalf("an entry is repeated: %+v", ev)
	default:
	}
}
