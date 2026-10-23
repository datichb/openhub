//go:build unix

package client

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/services/wiki"
)

// seen is a request received by the fake daemon.
type seen struct {
	Method, URI, Auth, Capability, ContentType string
	Body                                       string
}

// fake is a daemon of the user (private folder, socket 0600) that records the
// requests and answers with respond.
type fake struct {
	mu      sync.Mutex
	reqs    []seen
	respond func(w http.ResponseWriter, r *http.Request)
}

func (f *fake) last(t *testing.T) seen {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.NotEmpty(t, f.reqs)
	return f.reqs[len(f.reqs)-1]
}

func (f *fake) uris() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.reqs {
		out = append(out, r.URI)
	}
	return out
}

func (f *fake) first() seen {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reqs[0]
}

func (f *fake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reqs)
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, seen{r.Method, r.RequestURI, r.Header.Get("Authorization"), r.Header.Get(api.CapabilityHeader), r.Header.Get("Content-Type"), string(body)})
	f.mu.Unlock()
	if f.respond != nil {
		f.respond(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func serve(t *testing.T, respond func(w http.ResponseWriter, r *http.Request)) (*Client, *fake, api.Paths) {
	t.Helper()
	return serveAt(t, shortPaths(t), respond)
}

func serveAt(t *testing.T, p api.Paths, respond func(w http.ResponseWriter, r *http.Request)) (*Client, *fake, api.Paths) {
	t.Helper()
	require.NoError(t, os.Chmod(p.Dir, 0o700))
	l, err := net.Listen("unix", p.Socket())
	require.NoError(t, err)
	require.NoError(t, os.Chmod(p.Socket(), 0o600))
	f := &fake{respond: respond}
	srv := &http.Server{Handler: f, ReadHeaderTimeout: time.Second}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return NewClient(p), f, p
}

func reply(status int, v any) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
}

func TestClientRoutes(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		answer any
		call   func(c *Client) error
		method string
		uri    string
		body   string // "" = no body
	}{
		{"Health", api.Health{Version: "v"}, func(c *Client) error { _, err := c.Health(ctx); return err }, "GET", "/v1/health", ""},
		{"IssueGrant", api.GrantResponse{Token: "t"}, func(c *Client) error {
			_, err := c.IssueGrant(ctx, api.GrantRequest{Owner: "o", Provider: "p"})
			return err
		}, "POST", "/v1/grants", `{"owner":"o","provider":"p","source":{"kind":""}}`},
		{"PendingGrants", []api.PendingGrant{{Owner: "o"}}, func(c *Client) error { _, err := c.PendingGrants(ctx); return err }, "GET", "/v1/grants/pending", ""},
		{"ProvideSecret", nil, func(c *Client) error { return c.ProvideSecret(ctx, "tok", "sec") }, "POST", "/v1/grants/secret", `{"token":"tok","secret":"sec"}`},
		{"RevokeOwner", nil, func(c *Client) error { return c.RevokeOwner(ctx, "a/b c") }, "DELETE", "/v1/owners/a%2Fb%20c/grants", ""},
		{"Usage", api.UsageResponse{Requests: 3}, func(c *Client) error { _, err := c.Usage(ctx, "a&b"); return err }, "GET", "/v1/usage?token=a%26b", ""},
		{"ProxyListen", api.ListenResponse{URL: "http://h:1"}, func(c *Client) error { _, err := c.ProxyListen(ctx, "10.0.0.1"); return err }, "POST", "/v1/proxy/listeners", `{"host":"10.0.0.1"}`},
		{"IssueGatewayGrant", api.GatewayGrantResponse{Token: "g"}, func(c *Client) error {
			_, err := c.IssueGatewayGrant(ctx, api.GatewayGrantRequest{SessionID: "s", GroupKey: "g", Location: "l"})
			return err
		}, "POST", "/v1/gateway/grants", `{"session_id":"s","group_key":"g","location":"l","beads_allow":null}`},
		{"Touch", nil, func(c *Client) error { return c.Touch(ctx, "g 1") }, "POST", "/v1/servers/g%201/touch", ""},
		{"Shutdown", nil, func(c *Client) error { return c.Shutdown(ctx, false) }, "POST", "/v1/shutdown", ""},
		{"ShutdownForce", nil, func(c *Client) error { return c.Shutdown(ctx, true) }, "POST", "/v1/shutdown?force=true", ""},
		{"Heartbeat", nil, func(c *Client) error {
			return c.Heartbeat(ctx, api.HeartbeatRequest{ClientID: "c", Kind: api.ClientPresence})
		}, "POST", "/v1/clients/heartbeat", `{"client_id":"c","kind":"presence"}`},
		{"ClientGone", nil, func(c *Client) error { return c.ClientGone(ctx, "c/1") }, "DELETE", "/v1/clients/c%2F1", ""},
		{"SetPolicy", nil, func(c *Client) error { return c.SetPolicy(ctx, "g", api.PolicyStopNow) }, "POST", "/v1/groups/g/policy", `{"policy":"stop_now"}`},
		{"SessionDecided", nil, func(c *Client) error { return c.SessionDecided(ctx, "ses 1") }, "POST", "/v1/sessions/ses%201/decided", ""},
		{"AttachedClients", []api.AttachedClient{{ClientID: "c"}}, func(c *Client) error { _, err := c.AttachedClients(ctx, "s&1"); return err }, "GET", "/v1/clients?session=s%261", ""},
		{"WorkflowRefresh", nil, func(c *Client) error { return c.WorkflowRefresh(ctx, "s") }, "POST", "/v1/workflow/rules", `{"session_id":"s"}`},
		{"WorkflowStatus", checkpoint.Status{SessionID: "s"}, func(c *Client) error { _, err := c.WorkflowStatus(ctx, "t 1"); return err }, "GET", "/v1/workflow/status?tool_session=t+1", ""},
		{"WorkflowCheckpoint", checkpoint.Result{}, func(c *Client) error {
			_, err := c.WorkflowCheckpoint(ctx, "t", checkpoint.Call{ID: "cp-1"})
			return err
		}, "POST", "/v1/workflow/checkpoint", `{"tool_session":"t","id":"cp-1"}`},
		{"WorkflowOutput", nil, func(c *Client) error { return c.WorkflowOutput(ctx, "t", checkpoint.Output{}) }, "POST", "/v1/workflow/outputs", ""},
		{"WorkflowReport", api.WorkflowReport{ID: "rp-1"}, func(c *Client) error {
			_, err := c.WorkflowReport(ctx, "t", "title", "body")
			return err
		}, "POST", "/v1/workflow/report", `{"tool_session":"t","title":"title","body":"body"}`},
		{"WorkflowReports", []api.WorkflowReport{{ID: "rp-1"}}, func(c *Client) error { _, err := c.WorkflowReports(ctx, "t"); return err }, "GET", "/v1/workflow/reports?tool_session=t", ""},
		{"WorkflowReportGet", api.WorkflowReport{ID: "rp-1"}, func(c *Client) error {
			_, err := c.WorkflowReportGet(ctx, "t", "rp-1", "Intro")
			return err
		}, "GET", "/v1/workflow/report?id=rp-1&section=Intro&tool_session=t", ""},
		{"WorkflowReportGetWhole", api.WorkflowReport{ID: "rp-1"}, func(c *Client) error {
			_, err := c.WorkflowReportGet(ctx, "t", "rp-1", "")
			return err
		}, "GET", "/v1/workflow/report?id=rp-1&tool_session=t", ""},
		{"WikiIndex", api.WikiIndex{}, func(c *Client) error { _, err := c.WikiIndex(ctx, "t"); return err }, "GET", "/v1/wiki/index?tool_session=t", ""},
		{"WikiPage", api.WikiPage{Page: "a.md"}, func(c *Client) error { _, err := c.WikiPage(ctx, "t", "a.md", "S"); return err }, "GET", "/v1/wiki/page?page=a.md&section=S&tool_session=t", ""},
		{"WikiPropose", api.WikiProposal{ID: "wp-1"}, func(c *Client) error {
			_, err := c.WikiPropose(ctx, api.WikiProposeRequest{ToolSession: "t", Page: "a.md", Content: "c", Tag: "[verified]"})
			return err
		}, "POST", "/v1/wiki/propose", `{"tool_session":"t","page":"a.md","content":"c","tag":"[verified]"}`},
		{"WikiPending", []api.WikiProposal{}, func(c *Client) error { _, err := c.WikiPending(ctx, "t", false); return err }, "GET", "/v1/wiki/pending?tool_session=t", ""},
		{"WikiPendingSession", []api.WikiProposal{}, func(c *Client) error { _, err := c.WikiPending(ctx, "t", true); return err }, "GET", "/v1/wiki/pending?scope=session&tool_session=t", ""},
		{"WikiRefused", []api.WikiProposal{}, func(c *Client) error { _, err := c.WikiRefused(ctx, "t", true); return err }, "GET", "/v1/wiki/refused?scope=session&tool_session=t", ""},
		{"WikiApply", []wiki.Result{}, func(c *Client) error { _, err := c.WikiApply(ctx, "t", []string{"wp-1"}); return err }, "POST", "/v1/wiki/apply", `{"tool_session":"t","ids":["wp-1"]}`},
		{"WikiCheck", wiki.CheckReport{}, func(c *Client) error {
			_, err := c.WikiCheck(ctx, "t", wiki.CheckOptions{Paths: []string{"a", "b"}, Base: "main", Head: "HEAD", SinceSync: true})
			return err
		}, "GET", "/v1/wiki/check?base=main&head=HEAD&path=a&path=b&since_sync=1&tool_session=t", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			respond := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
			if tc.answer != nil {
				respond = reply(http.StatusOK, tc.answer)
			}
			c, f, _ := serve(t, respond)
			require.NoError(t, tc.call(c))
			got := f.last(t)
			assert.Equal(t, tc.method, got.Method)
			assert.Equal(t, tc.uri, got.URI)
			if tc.body != "" {
				assert.JSONEq(t, tc.body, got.Body)
				assert.Equal(t, "application/json", got.ContentType)
			}
			if tc.body == "" && tc.method != "POST" {
				assert.Empty(t, got.Body)
			}
		})
	}
}

func TestClientDecodesTheAnswer(t *testing.T) {
	c, _, _ := serve(t, reply(http.StatusOK, api.Health{Version: "5.1", PID: 42, Servers: 2, Restoring: true}))
	h, err := c.Health(context.Background())
	require.NoError(t, err)
	assert.Equal(t, api.Health{Version: "5.1", PID: 42, Servers: 2, Restoring: true}, h)

	c, _, _ = serve(t, reply(http.StatusOK, api.ListenResponse{URL: "http://10.0.0.1:9"}))
	url, err := c.ProxyListen(context.Background(), "10.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, "http://10.0.0.1:9", url)
}

func TestClientSendsItsCredentials(t *testing.T) {
	c, f, _ := serve(t, nil)
	ctx := context.Background()

	require.NoError(t, c.Touch(ctx, "g"))
	got := f.last(t)
	assert.Empty(t, got.Capability)
	assert.Empty(t, got.Auth)

	withCap := c.WithCapability("cap-1")
	withTok := withCap.WithGroupToken("ohw_tok")
	require.NoError(t, withTok.Touch(ctx, "g"))
	got = f.last(t)
	assert.Equal(t, "cap-1", got.Capability)
	assert.Equal(t, "Bearer ohw_tok", got.Auth)

	require.NoError(t, withCap.Touch(ctx, "g"))
	got = f.last(t)
	assert.Equal(t, "cap-1", got.Capability)
	assert.Empty(t, got.Auth, "WithGroupToken returns a copy")

	require.NoError(t, c.Touch(ctx, "g"))
	assert.Empty(t, f.last(t).Capability, "WithCapability returns a copy")
}

func TestClientAPIError(t *testing.T) {
	c, _, _ := serve(t, reply(http.StatusConflict, map[string]string{"error": "not approved"}))
	_, err := c.WorkflowStatus(context.Background(), "t")
	var ae *api.APIError
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, http.StatusConflict, ae.Status)
	assert.Equal(t, "not approved", ae.Message)
	assert.Equal(t, "GET", ae.Method)
	assert.Equal(t, "/workflow/status?tool_session=t", ae.Path)
	assert.Equal(t, "ohd GET /workflow/status?tool_session=t: 409 not approved", err.Error())

	// no message: the status line stands for it
	c, _, _ = serve(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", http.StatusNotFound) })
	err = c.Touch(context.Background(), "g")
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, http.StatusNotFound, ae.Status)
	assert.Equal(t, "404 Not Found", ae.Message)
}

func TestClientStream(t *testing.T) {
	c, f, _ := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		_ = enc.Encode(api.StreamEvent{})                                                    // keep-alive
		_ = enc.Encode(api.StreamEvent{Change: &domain.SessionChange{SessionID: "s1"}})      // change
		_ = enc.Encode(api.StreamEvent{Feed: &domain.FeedItem{SessionID: "s1", Text: "hi"}}) // feed
	})
	ch, err := c.Stream(context.Background(), "s 1")
	require.NoError(t, err)
	var got []api.StreamEvent
	for ev := range ch {
		got = append(got, ev)
	}
	require.Len(t, got, 2, "the keep-alive is not forwarded")
	require.NotNil(t, got[0].Change)
	assert.Equal(t, "s1", got[0].Change.SessionID)
	require.NotNil(t, got[1].Feed)
	assert.Equal(t, "hi", got[1].Feed.Text)
	assert.Equal(t, "/v1/stream?session=s+1", f.last(t).URI)

	_, err = c.Stream(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "/v1/stream", f.last(t).URI)
}

func TestClientStreamRefused(t *testing.T) {
	c, _, _ := serve(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	_, err := c.Stream(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

func TestClientStreamEndsWithItsContext(t *testing.T) {
	c, _, _ := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := c.Stream(ctx, "")
	require.NoError(t, err)
	cancel()
	select {
	case _, ok := <-ch:
		assert.False(t, ok)
	case <-time.After(5 * time.Second):
		t.Fatal("the channel stayed open after the context was cancelled")
	}
}

func TestKeepAlive(t *testing.T) {
	c, f, _ := serve(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.KeepAlive(ctx, api.HeartbeatRequest{ClientID: "cl-1", Kind: api.ClientAttach, Group: "g"}, 20*time.Millisecond)
		close(done)
	}()
	require.Eventually(t, func() bool { return f.count() >= 3 }, 5*time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("KeepAlive did not return")
	}
	first := f.first()
	assert.Equal(t, "/v1/clients/heartbeat", first.URI)
	var hb api.HeartbeatRequest
	require.NoError(t, json.Unmarshal([]byte(first.Body), &hb))
	assert.Equal(t, 0, hb.TTL-int(3*20*time.Millisecond/time.Second), "the ttl is three periods (rounded to the second)")
	last := f.last(t)
	assert.Equal(t, "DELETE", last.Method)
	assert.Equal(t, "/v1/clients/cl-1", last.URI, "the client is removed when the context ends")
}
