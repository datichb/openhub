package credproxy

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gateSpy records the usage the gate is told at the end of each request.
type gateSpy struct {
	mu   sync.Mutex
	done []Usage
}

func (g *gateSpy) gate(_ context.Context, _ string, _ Admission) (func(Usage), error) {
	return func(u Usage) {
		g.mu.Lock()
		defer g.mu.Unlock()
		g.done = append(g.done, u)
	}, nil
}

func (g *gateSpy) last(t *testing.T) Usage {
	t.Helper()
	require.Eventually(t, func() bool { g.mu.Lock(); defer g.mu.Unlock(); return len(g.done) > 0 }, 3*time.Second, 5*time.Millisecond)
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.done[len(g.done)-1]
}

// The daemon counts the dollars of a response cut before its end, which the
// tool never records: the proxy tells it so, whether the counters were read
// or estimated. A complete response is not flagged.
func TestProxyFlagsACutResponseToTheGate(t *testing.T) {
	const start = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":120,\"output_tokens\":1}}}\n\n"
	const stop = "event: message_delta\ndata: {\"usage\":{\"output_tokens\":42}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	text := strings.Repeat("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"word\"}}\n\n", 40)
	cases := []struct {
		name, body string
		cut        bool
		cutCount   int64
	}{
		{"cut after the start", start + text, true, 1},
		{"cut before anything", "event: ping\ndata: {}\n\n", true, 1},
		{"complete", start + text + stop, false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var s seen
			up := upstream(t, &s, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, c.body)
				if c.cut {
					w.(http.Flusher).Flush()
					panic(http.ErrAbortHandler)
				}
			})
			p := startProxy(t)
			spy := &gateSpy{}
			p.Gate = spy.gate
			tok, err := p.Issue(Grant{SessionID: "g1", Provider: ProviderAnthropic, Upstream: Upstream{BaseURL: up.URL, Auth: HeaderAuth{Name: "x-api-key", Value: "k"}}})
			require.NoError(t, err)
			req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderAnthropic)+"/messages", strings.NewReader(`{"model":"m","stream":true,"max_tokens":4096}`))
			req.Header.Set("x-api-key", tok)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			got := spy.last(t)
			assert.Equal(t, c.cutCount, got.Cut)
			assert.Positive(t, got.InputTokens, "the tokens the proxy counted come with it")
			u, _ := p.Usage(tok)
			assert.Equal(t, c.cutCount, u.Cut)
		})
	}
}

// A request that failed upstream is no cut response: nothing is estimated.
func TestProxyDoesNotFlagAFailedRequest(t *testing.T) {
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", http.StatusInternalServerError) })
	p := startProxy(t)
	spy := &gateSpy{}
	p.Gate = spy.gate
	tok, _ := p.Issue(Grant{SessionID: "g1", Provider: ProviderAnthropic, Upstream: Upstream{BaseURL: up.URL, Auth: HeaderAuth{Name: "x-api-key", Value: "k"}}})
	status, _, _ := call(t, p, tok, ProviderAnthropic, "/messages", `{"model":"m","max_tokens":10}`)
	require.Equal(t, http.StatusInternalServerError, status)
	assert.Zero(t, spy.last(t).Cut)
}
