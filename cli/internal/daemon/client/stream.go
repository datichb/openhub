package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/safego"
)

// streamRetry is the pause before the stream is resumed after it ended.
const streamRetry = time.Second

// openStream opens GET /v1/stream, resuming after since when resume is set.
func (c *Client) openStream(ctx context.Context, sessionID string, since uint64, resume bool) (*http.Response, error) {
	q := url.Values{}
	if sessionID != "" {
		q.Set("session", sessionID)
	}
	if resume {
		q.Set(api.StreamSinceParam, strconv.FormatUint(since, 10))
	}
	path := "http://ohd" + api.Prefix + api.RouteStream
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := c.stream.Do(req) //nolint:bodyclose // closed by the reader goroutine
	if err != nil {
		return nil, notRunningOr(err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("ohd stream: %s", resp.Status)
	}
	return resp, nil
}

// Stream subscribes to the live stream of the daemon: session changes, plus
// the feed of one session when sessionID is set (its recent backlog first).
//
// The client follows the sequence numbers of the entries. When the connection
// drops while the daemon still answers, it resumes after the last one it saw
// (?since=), so nothing is lost nor repeated; an entry already seen (the
// backlog replayed after a resync) is skipped. A resync event is forwarded: the
// entries between two connections may be missing, and the reader reloads what
// it shows. A daemon without sequence numbers (older) cannot be resumed: the
// channel is closed when its stream ends.
//
// The channel is also closed when ctx is cancelled or the daemon is gone.
func (c *Client) Stream(ctx context.Context, sessionID string) (<-chan api.StreamEvent, error) {
	resp, err := c.openStream(ctx, sessionID, 0, false)
	if err != nil {
		return nil, err
	}
	out := make(chan api.StreamEvent, 64)
	safego.Go(func() {
		defer close(out)
		var st streamState
		for {
			c.readStream(ctx, resp, out, &st)
			if !st.numbered || ctx.Err() != nil {
				return
			}
			wait := c.streamWait
			if wait <= 0 {
				wait = streamRetry
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			var err error
			if resp, err = c.openStream(ctx, sessionID, st.last, true); err != nil { //nolint:bodyclose // closed by readStream
				return // the daemon is gone (or refuses us): the stream ends
			}
		}
	})
	return out, nil
}

// streamState is what a reader knows of the daemon: the last sequence number
// it passed on, and whether the daemon numbers its entries at all.
type streamState struct {
	last     uint64
	numbered bool
}

// readStream forwards the entries of one connection to out and returns when it
// ends (or ctx is cancelled).
func (c *Client) readStream(ctx context.Context, resp *http.Response, out chan<- api.StreamEvent, st *streamState) {
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	for {
		var ev api.StreamEvent
		if err := dec.Decode(&ev); err != nil {
			return
		}
		if ev.Seq > 0 {
			st.numbered = true
		}
		switch {
		case ev.Resync:
			// Forwarded as it is; the cursor moves with the entries that follow.
		case ev.Feed == nil && ev.Change == nil:
			st.last = max(st.last, ev.Seq) // keep-alive or end of the replay: the cursor
			continue
		case ev.Seq > 0 && ev.Seq <= st.last:
			continue // already delivered (backlog replayed after a resync)
		default:
			st.last = max(st.last, ev.Seq)
		}
		select {
		case out <- ev:
		case <-ctx.Done():
			return
		}
	}
}
