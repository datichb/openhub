//go:build unix

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
)

func feedEv(seq uint64, text string) api.StreamEvent {
	return api.StreamEvent{Seq: seq, Feed: &domain.FeedItem{SessionID: "s", Kind: domain.FeedText, Text: text}}
}

// script answers the n-th connection with the n-th batch of lines, then ends
// it (or holds it open until the client leaves when hold is set).
func script(batches [][]api.StreamEvent, hold bool) func(w http.ResponseWriter, r *http.Request) {
	n := 0
	return func(w http.ResponseWriter, r *http.Request) {
		if n >= len(batches) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		batch := batches[n]
		n++
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		for _, ev := range batch {
			_ = enc.Encode(ev)
		}
		w.(http.Flusher).Flush()
		if hold && n == len(batches) {
			<-r.Context().Done()
		}
	}
}

func collect(t *testing.T, ch <-chan api.StreamEvent, n int) []api.StreamEvent {
	t.Helper()
	var got []api.StreamEvent
	timeout := time.After(5 * time.Second)
	for len(got) < n {
		select {
		case ev, ok := <-ch:
			require.True(t, ok, "the stream closed after %d events", len(got))
			got = append(got, ev)
		case <-timeout:
			t.Fatalf("only %d events out of %d", len(got), n)
		}
	}
	return got
}

// The connection drops: the client resumes after the last number it saw, and
// the reader sees every entry once, in order.
func TestStreamResumesAfterTheLastSeq(t *testing.T) {
	c, f, _ := serve(t, script([][]api.StreamEvent{
		{feedEv(11, "a"), feedEv(12, "b"), {Seq: 12}},
		{feedEv(13, "c"), {Seq: 13}},
	}, true))
	c.streamWait = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := c.Stream(ctx, "s")
	require.NoError(t, err)
	got := collect(t, ch, 3)
	for i, text := range []string{"a", "b", "c"} {
		assert.Equal(t, text, got[i].Feed.Text)
	}
	assert.Equal(t, []string{"/v1/stream?session=s", "/v1/stream?session=s&since=12"}, f.uris())
}

// The cursor also moves with the keep-alives: entries of other sessions that
// this reader does not receive must not make the next resume too old.
func TestStreamResumesFromTheKeepAliveCursor(t *testing.T) {
	c, f, _ := serve(t, script([][]api.StreamEvent{
		{{Seq: 5}, {Seq: 40}},
		{feedEv(41, "x")},
	}, true))
	c.streamWait = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := c.Stream(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, "x", collect(t, ch, 1)[0].Feed.Text)
	assert.Equal(t, []string{"/v1/stream", "/v1/stream?since=40"}, f.uris())
}

// After a resync the daemon replays the backlog with the original numbers:
// what the reader already has is not delivered again.
func TestStreamResyncForwardedAndBacklogDeduplicated(t *testing.T) {
	c, _, _ := serve(t, script([][]api.StreamEvent{
		{feedEv(1, "a"), feedEv(2, "b"), {Seq: 2}},
		{{Resync: true, Seq: 9}, feedEv(1, "a"), feedEv(2, "b"), feedEv(5, "e"), {Seq: 9}, feedEv(10, "j")},
	}, true))
	c.streamWait = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := c.Stream(ctx, "s")
	require.NoError(t, err)
	got := collect(t, ch, 5)
	assert.Equal(t, "a", got[0].Feed.Text)
	assert.Equal(t, "b", got[1].Feed.Text)
	assert.True(t, got[2].Resync)
	assert.Equal(t, "e", got[3].Feed.Text, "the missed entry of the backlog")
	assert.Equal(t, "j", got[4].Feed.Text)
}

// A daemon without sequence numbers cannot be resumed (a replay would repeat
// the backlog): the stream ends with the connection, as before.
func TestStreamWithoutSeqIsNotResumed(t *testing.T) {
	c, f, _ := serve(t, script([][]api.StreamEvent{
		{{Feed: &domain.FeedItem{SessionID: "s", Text: "old"}}},
		{{Feed: &domain.FeedItem{SessionID: "s", Text: "again"}}},
	}, false))
	c.streamWait = time.Millisecond
	ch, err := c.Stream(context.Background(), "s")
	require.NoError(t, err)
	var got []api.StreamEvent
	for ev := range ch {
		got = append(got, ev)
	}
	require.Len(t, got, 1)
	assert.Equal(t, 1, f.count())
}

// When the daemon is gone (or refuses the resume), the stream ends.
func TestStreamEndsWhenTheResumeIsRefused(t *testing.T) {
	c, f, _ := serve(t, script([][]api.StreamEvent{{feedEv(3, "a"), {Seq: 3}}}, false))
	c.streamWait = time.Millisecond
	ch, err := c.Stream(context.Background(), "s")
	require.NoError(t, err)
	var got []api.StreamEvent
	for ev := range ch {
		got = append(got, ev)
	}
	require.Len(t, got, 1)
	assert.Equal(t, []string{"/v1/stream?session=s", "/v1/stream?session=s&since=3"}, f.uris())
}

// A reader that leaves while the stream waits to resume stops it.
func TestStreamStopsWhileWaitingToResume(t *testing.T) {
	c, _, _ := serve(t, script([][]api.StreamEvent{{feedEv(3, "a")}}, false))
	c.streamWait = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := c.Stream(ctx, "s")
	require.NoError(t, err)
	collect(t, ch, 1)
	cancel()
	select {
	case _, ok := <-ch:
		assert.False(t, ok)
	case <-time.After(5 * time.Second):
		t.Fatal("the stream did not stop")
	}
}
