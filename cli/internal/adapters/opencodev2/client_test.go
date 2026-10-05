package opencodev2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
)

type recorded struct {
	method, path, query, user, pass, body string
	hasAuth                               bool
}

func newTestServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, rec recorded)) (*Client, *[]recorded) {
	t.Helper()
	var calls []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		u, p, ok := r.BasicAuth()
		rec := recorded{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, user: u, pass: p, hasAuth: ok, body: string(body)}
		calls = append(calls, rec)
		handler(w, r, rec)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL+"/", "secret"), &calls
}

func TestClientAuthAndLocation(t *testing.T) {
	c, calls := newTestServer(t, func(w http.ResponseWriter, r *http.Request, _ recorded) {
		fmt.Fprint(w, `{"location":{"directory":"/p"},"data":[{"id":"build","name":"Build","mode":"primary","hidden":false},{"id":"title","mode":"primary","hidden":true}]}`)
	})
	agents, err := c.Agents(context.Background(), "/tmp/my proj")
	require.NoError(t, err)
	require.Len(t, agents, 2)
	assert.Equal(t, "build", agents[0].ID)
	assert.True(t, agents[1].Hidden)

	got := (*calls)[0]
	assert.Equal(t, "/api/agent", got.path)
	assert.True(t, got.hasAuth)
	assert.Equal(t, BasicUser, got.user)
	assert.Equal(t, "secret", got.pass)
	assert.Equal(t, "location%5Bdirectory%5D=%2Ftmp%2Fmy+proj", got.query)
}

func TestClientAPIError(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request, _ recorded) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"_tag":"SessionNotFoundError","sessionID":"ses_x","message":"Session not found: ses_x"}`)
	})
	_, err := c.GetSession(context.Background(), "ses_x")
	require.Error(t, err)
	assert.True(t, IsNotFound(err))
	assert.Contains(t, err.Error(), "SessionNotFoundError")

	c2, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request, _ recorded) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `plain text`)
	})
	_, err = c2.Info(context.Background())
	assert.True(t, IsUnauthorized(err))
	assert.Contains(t, err.Error(), "plain text")
}

func TestClientCreateSessionBody(t *testing.T) {
	c, calls := newTestServer(t, func(w http.ResponseWriter, r *http.Request, rec recorded) {
		fmt.Fprint(w, `{"data":{"id":"ses_abc","agent":"pinger","title":"t","cost":0.5,"tokens":{"input":3,"output":4,"reasoning":0,"cache":{"read":7,"write":1}},"location":{"directory":"/p"}}}`)
	})
	s, err := c.CreateSession(context.Background(), CreateSessionRequest{
		ID: "ses_abc", Title: "t", Agent: "pinger",
		Location:    &Location{Directory: "/p"},
		Permissions: []Rule{{Action: "skill", Resource: "x", Effect: "deny"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "ses_abc", s.ID)
	assert.Equal(t, int64(7), s.Tokens.Cache.Read)
	assert.InDelta(t, 0.5, s.Cost, 1e-9)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte((*calls)[0].body), &body))
	assert.Equal(t, "ses_abc", body["id"])
	assert.Equal(t, map[string]any{"directory": "/p"}, body["location"])
	assert.NotContains(t, body, "model")
}

func TestClientDecisionBodies(t *testing.T) {
	c, calls := newTestServer(t, func(w http.ResponseWriter, r *http.Request, _ recorded) {
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := context.Background()
	require.NoError(t, c.ReplyPermission(ctx, "ses_1", "per_1", "once", "ok by oh"))
	require.NoError(t, c.ReplyForm(ctx, "ses_1", "frm_1", map[string]any{"q0": "Blue"}))
	require.NoError(t, c.Prompt(ctx, "ses_1", "go"))
	require.NoError(t, c.SetEnvironment(ctx, "ses_1", map[string]string{"A": "1"}))

	assert.Equal(t, "/api/session/ses_1/permission/per_1/reply", (*calls)[0].path)
	assert.JSONEq(t, `{"decision":"once","message":"ok by oh"}`, (*calls)[0].body)
	assert.Equal(t, "/api/session/ses_1/form/frm_1/reply", (*calls)[1].path)
	assert.JSONEq(t, `{"answer":{"q0":"Blue"}}`, (*calls)[1].body)
	assert.JSONEq(t, `{"text":"go"}`, (*calls)[2].body)
	assert.Equal(t, http.MethodPut, (*calls)[3].method)
	assert.JSONEq(t, `{"variables":{"A":"1"}}`, (*calls)[3].body)
}

func TestClientPermissionsAndForms(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request, _ recorded) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/permission"):
			fmt.Fprint(w, `{"data":[{"id":"per_1","sessionID":"ses_1","action":"shell","resources":["echo X"],"save":["echo *"]}]}`)
		case strings.HasSuffix(r.URL.Path, "/form"):
			fmt.Fprint(w, `{"data":[{"id":"frm_1","sessionID":"ses_1","title":"Questions","metadata":{"kind":"question"},"fields":[{"key":"q0","title":"Color","type":"string","options":[{"value":"Red","label":"Red"}],"custom":true}]}]}`)
		}
	})
	perms, err := c.Permissions(context.Background(), "ses_1")
	require.NoError(t, err)
	require.Len(t, perms, 1)
	assert.Equal(t, []string{"echo X"}, perms[0].Resources)

	forms, err := c.Forms(context.Background(), "ses_1")
	require.NoError(t, err)
	require.Len(t, forms, 1)
	assert.Equal(t, "question", forms[0].Metadata["kind"])
	assert.True(t, forms[0].Fields[0].Custom)
	assert.Equal(t, "Red", forms[0].Fields[0].Options[0].Value)
}

func TestParseSSE(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"id":"evt_1","type":"server.connected","data":{}}`,
		``,
		`: heartbeat`,
		``,
		`data: {"id":"evt_2","created":1791209362803,"type":"session.renamed","location":{"directory":"/p"},"data":{"sessionID":"ses_1","title":"renamed"}}`,
		``,
		`data: not-json`,
		``,
		`event: ignored`,
		`data: {"id":"evt_3","type":"permission.asked","data":{"sessionID":"ses_1"}}`,
		``,
	}, "\n")
	out := make(chan Event, 10)
	err := parseSSE(context.Background(), strings.NewReader(stream), out)
	assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	close(out)
	var got []Event
	for e := range out {
		got = append(got, e)
	}
	require.Len(t, got, 3)
	assert.Equal(t, EventTypeConnected, got[0].Type)
	assert.Equal(t, "ses_1", got[1].SessionID())
	assert.Equal(t, "/p", got[1].Location.Directory)
	assert.Equal(t, int64(1791209362803), got[1].Time().UnixMilli())
	assert.Equal(t, "permission.asked", got[2].Type)
	assert.True(t, got[0].Time().IsZero())
}

func TestEventsStream(t *testing.T) {
	c, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request, _ recorded) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"evt_1\",\"type\":\"server.connected\",\"data\":{}}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	events, errc, err := c.Events(ctx)
	require.NoError(t, err)
	ev := <-events
	assert.Equal(t, EventTypeConnected, ev.Type)
	cancel()
	for range events {
	}
	assert.NoError(t, <-errc)
}

func TestEventKind(t *testing.T) {
	cases := map[string]adapters.EventKind{
		"server.connected": adapters.EventConnected, "session.execution.started": adapters.EventExecStarted,
		"session.execution.succeeded": adapters.EventExecEnded, "permission.asked": adapters.EventDecisionAsked,
		"form.created": adapters.EventDecisionAsked, "permission.replied": adapters.EventDecisionReplied,
		"session.usage.updated": adapters.EventUsage, "session.tool.called": adapters.EventActivity,
		"provider.updated": adapters.EventOther,
	}
	for in, want := range cases {
		got, _ := EventKind(in)
		assert.Equal(t, want, got, in)
	}
	_, outcome := EventKind("session.execution.failed")
	assert.Equal(t, "failed", outcome)
}
