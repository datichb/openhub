package opencodev2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
)

// Event is one server event from /api/event.
type Event struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Created  float64         `json:"created,omitempty"` // unix milliseconds
	Location *Location       `json:"location,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

// SessionID extracts the session of an event: data.sessionID, or the
// sessionID of the object it carries (form.created: data.form.sessionID,
// permission requests…).
func (e Event) SessionID() string {
	if len(e.Data) == 0 {
		return ""
	}
	var d map[string]json.RawMessage
	if json.Unmarshal(e.Data, &d) != nil {
		return ""
	}
	var id string
	if raw, ok := d["sessionID"]; ok && json.Unmarshal(raw, &id) == nil && id != "" {
		return id
	}
	for _, k := range []string{"form", "request", "permission", "info", "session"} {
		raw, ok := d[k]
		if !ok {
			continue
		}
		var inner struct {
			SessionID string `json:"sessionID"`
		}
		if json.Unmarshal(raw, &inner) == nil && inner.SessionID != "" {
			return inner.SessionID
		}
	}
	return ""
}

// Time returns the event creation time (zero if unknown).
func (e Event) Time() time.Time {
	if e.Created == 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(e.Created))
}

// EventTypeConnected is the first event of every subscription.
const EventTypeConnected = "server.connected"

// Events subscribes to the server event stream. The events channel is closed
// when the stream ends (context cancelled, server gone, read error); the
// terminal error, if any, is sent on errc before it is closed.
//
// The stream is live-only: events emitted while disconnected are lost, so
// callers must resynchronize state from list endpoints after (re)connecting.
func (c *Client) Events(ctx context.Context) (events <-chan Event, errs <-chan error, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/event", http.NoBody)
	if err != nil {
		return nil, nil, err
	}
	c.authorize(req)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.stream.Do(req) //nolint:bodyclose // closed by the reader goroutine
	if err != nil {
		return nil, nil, fmt.Errorf("subscribing to events: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, nil, decodeAPIError(resp.StatusCode, data)
	}

	out := make(chan Event, 64)
	errc := make(chan error, 1)
	go func() {
		defer resp.Body.Close()
		defer close(out)
		defer close(errc)
		if err := parseSSE(ctx, resp.Body, out); err != nil && ctx.Err() == nil {
			errc <- err
		}
	}()
	return out, errc, nil
}

// parseSSE reads "data:" lines (one JSON event per SSE message) and ignores
// comments (": heartbeat") and other fields.
func parseSSE(ctx context.Context, r io.Reader, out chan<- Event) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var data strings.Builder
	flush := func() error {
		if data.Len() == 0 {
			return nil
		}
		raw := data.String()
		data.Reset()
		var ev Event
		if !decodeEvent(raw, &ev) {
			return nil // skip malformed message, keep the stream alive
		}
		select {
		case out <- ev:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return err
			}
		case strings.HasPrefix(line, ":"):
			// comment / heartbeat
		case strings.HasPrefix(line, "data:"):
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := flush(); err != nil {
		return err
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("reading event stream: %w", err)
	}
	return io.ErrUnexpectedEOF
}

func decodeEvent(raw string, ev *Event) bool {
	return json.Unmarshal([]byte(raw), ev) == nil
}

// EventKind maps an opencode V2 event type to its tool-agnostic kind.
func EventKind(t string) (kind adapters.EventKind, outcome string) {
	switch {
	case t == EventTypeConnected:
		return adapters.EventConnected, ""
	case t == "session.execution.started":
		return adapters.EventExecStarted, ""
	case strings.HasPrefix(t, "session.execution."):
		return adapters.EventExecEnded, strings.TrimPrefix(t, "session.execution.")
	case t == "permission.asked", t == "form.created":
		return adapters.EventDecisionAsked, ""
	case t == "permission.replied", t == "form.replied", t == "form.cancelled":
		return adapters.EventDecisionReplied, ""
	case t == "session.usage.updated":
		return adapters.EventUsage, ""
	case t == "session.created":
		return adapters.EventSessionCreated, ""
	case t == "session.inbox.enqueued":
		return adapters.EventUserInput, ""
	case strings.HasPrefix(t, "session."), strings.HasPrefix(t, "shell."):
		return adapters.EventActivity, ""
	}
	return adapters.EventOther, ""
}
