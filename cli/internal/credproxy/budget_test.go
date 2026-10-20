package credproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// call posts a body through the proxy and returns the status and the body.
func call(t *testing.T, p *Proxy, tok, provider, path, body string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, p.BaseURL(provider)+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

func streamOf(t *testing.T, frames ...string) string {
	t.Helper()
	var b []byte
	for _, f := range frames {
		b = append(b, eventFrame([]byte(f))...)
	}
	return string(b)
}

func invokeStreamOf(t *testing.T, chunks ...string) string {
	t.Helper()
	var b []byte
	for _, c := range chunks {
		b = append(b, chunkFrame(t, c)...)
	}
	return string(b)
}

// The cache is counted for each usage format (S-27): the input count of
// Anthropic and Bedrock leaves the cache out, the one of OpenAI-style APIs
// holds it (it moves to the cache counters).
func TestProxyCountsCacheTokens(t *testing.T) {
	const sse, es = "text/event-stream", "application/vnd.amazon.eventstream"
	cases := []struct {
		name, provider, path, ctype, response string
		want                                  Usage
	}{
		{
			name: "anthropic", provider: ProviderAnthropic, path: "/messages",
			response: `{"id":"msg_1","usage":{"input_tokens":10,"cache_creation_input_tokens":2000,"cache_read_input_tokens":30000,"output_tokens":40}}`,
			want:     Usage{InputTokens: 10, OutputTokens: 40, CacheReadTokens: 30000, CacheWriteTokens: 2000},
		},
		{
			name: "anthropic stream", provider: ProviderAnthropic, path: "/messages", ctype: sse,
			response: "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":10,\"cache_creation_input_tokens\":2000,\"cache_read_input_tokens\":30000,\"output_tokens\":1}}}\n\n" +
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":55}}\n\n" +
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
			want: Usage{InputTokens: 10, OutputTokens: 55, CacheReadTokens: 30000, CacheWriteTokens: 2000},
		},
		{
			name: "bedrock converse", provider: ProviderBedrock, path: "/model/m/converse",
			response: `{"usage":{"inputTokens":8,"outputTokens":21,"totalTokens":29,"cacheReadInputTokens":5000,"cacheWriteInputTokens":700}}`,
			want:     Usage{InputTokens: 8, OutputTokens: 21, CacheReadTokens: 5000, CacheWriteTokens: 700},
		},
		{
			name: "bedrock converse stream", provider: ProviderBedrock, path: "/model/m/converse-stream", ctype: es,
			response: streamOf(t, `{"delta":{"text":"hi"}}`,
				`{"metrics":{"latencyMs":3},"usage":{"inputTokens":8,"outputTokens":21,"totalTokens":29,"cacheReadInputTokens":5000,"cacheWriteInputTokens":700}}`),
			want: Usage{InputTokens: 8, OutputTokens: 21, CacheReadTokens: 5000, CacheWriteTokens: 700},
		},
		{
			name: "bedrock invoke", provider: ProviderBedrock, path: "/model/m/invoke",
			response: `{"type":"message","usage":{"input_tokens":12,"output_tokens":9,"cache_read_input_tokens":4000,"cache_creation_input_tokens":300}}`,
			want:     Usage{InputTokens: 12, OutputTokens: 9, CacheReadTokens: 4000, CacheWriteTokens: 300},
		},
		{
			name: "bedrock invoke stream", provider: ProviderBedrock, path: "/model/m/invoke-with-response-stream", ctype: es,
			response: invokeStreamOf(t,
				`{"type":"message_start","message":{"usage":{"input_tokens":12,"cache_read_input_tokens":4000,"cache_creation_input_tokens":300,"output_tokens":1}}}`,
				`{"type":"message_delta","usage":{"output_tokens":9}}`,
				`{"type":"message_stop","amazon-bedrock-invocationMetrics":{"inputTokenCount":12,"outputTokenCount":9,"cacheReadInputTokenCount":4000,"cacheWriteInputTokenCount":300}}`),
			want: Usage{InputTokens: 12, OutputTokens: 9, CacheReadTokens: 4000, CacheWriteTokens: 300},
		},
		{
			name: "openai chat", provider: ProviderOpenAI, path: "/chat/completions",
			response: `{"usage":{"prompt_tokens":1000,"completion_tokens":20,"prompt_tokens_details":{"cached_tokens":800}}}`,
			want:     Usage{InputTokens: 200, OutputTokens: 20, CacheReadTokens: 800},
		},
		{
			name: "openai chat stream", provider: ProviderOpenAI, path: "/chat/completions", ctype: sse,
			response: "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n" +
				"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1000,\"completion_tokens\":20,\"prompt_tokens_details\":{\"cached_tokens\":800}}}\n\n" +
				"data: [DONE]\n\n",
			want: Usage{InputTokens: 200, OutputTokens: 20, CacheReadTokens: 800},
		},
		{
			name: "openai responses", provider: ProviderOpenAI, path: "/responses",
			response: `{"usage":{"input_tokens":1500,"input_tokens_details":{"cached_tokens":1200},"output_tokens":33,"output_tokens_details":{"reasoning_tokens":5}}}`,
			want:     Usage{InputTokens: 300, OutputTokens: 33, CacheReadTokens: 1200},
		},
		{
			name: "openrouter", provider: ProviderOpenRouter, path: "/chat/completions",
			response: `{"usage":{"prompt_tokens":900,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":500,"cache_write_tokens":100}}}`,
			want:     Usage{InputTokens: 300, OutputTokens: 10, CacheReadTokens: 500, CacheWriteTokens: 100},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var s seen
			up := upstream(t, &s, func(w http.ResponseWriter, _ *http.Request) {
				if tc.ctype != "" {
					w.Header().Set("Content-Type", tc.ctype)
				}
				_, _ = io.WriteString(w, tc.response)
			})
			p := startProxy(t)
			var mu sync.Mutex
			var reported Usage
			p.OnUsage = func(_ string, d Usage) {
				mu.Lock()
				defer mu.Unlock()
				reported.InputTokens += d.InputTokens
				reported.OutputTokens += d.OutputTokens
				reported.CacheReadTokens += d.CacheReadTokens
				reported.CacheWriteTokens += d.CacheWriteTokens
				reported.Estimated += d.Estimated
			}
			tok, err := p.Issue(Grant{SessionID: "g1", Provider: tc.provider, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "k"}}})
			require.NoError(t, err)
			status, _, _ := call(t, p, tok, tc.provider, tc.path, `{"model":"m","stream":true}`)
			require.Equal(t, http.StatusOK, status)

			want := tc.want
			want.Requests = 1
			require.Eventually(t, func() bool { u, _ := p.Usage(tok); return u == want }, 2*time.Second, 5*time.Millisecond)
			mu.Lock()
			defer mu.Unlock()
			want.Requests = 0
			assert.Equal(t, want, reported, "the ledger receives the cache too")
		})
	}
}

// The token budget counts the cache.
func TestProxyBudgetCountsCache(t *testing.T) {
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":98}}`)
	})
	p := startProxy(t)
	tok, _ := p.Issue(Grant{Provider: ProviderAnthropic, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "k"}}, MaxTokens: 100})
	status, _, _ := call(t, p, tok, ProviderAnthropic, "/messages", `{"model":"m"}`)
	assert.Equal(t, http.StatusOK, status)
	require.Eventually(t, func() bool { return p.Exhausted(tok) }, 2*time.Second, 5*time.Millisecond)
	status, _, _ = call(t, p, tok, ProviderAnthropic, "/messages", `{"model":"m"}`)
	assert.Equal(t, http.StatusTooManyRequests, status)
}

// A stream cut before its usage message is counted with what was seen; a
// stream cut before anything is estimated from the request, high (S-27).
func TestProxyCountsCutStream(t *testing.T) {
	const start = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":120,\"cache_read_input_tokens\":3000,\"output_tokens\":1}}}\n\n"
	text := strings.Repeat("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"word\"}}\n\n", 40)
	cut := func(t *testing.T, body string) (*Proxy, string) {
		t.Helper()
		var s seen
		up := upstream(t, &s, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, body)
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler) // the connection drops mid-stream
		})
		p := startProxy(t)
		tok, err := p.Issue(Grant{Provider: ProviderAnthropic, Upstream: Upstream{BaseURL: up.URL, Auth: HeaderAuth{Name: "x-api-key", Value: "k"}}})
		require.NoError(t, err)
		req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderAnthropic)+"/messages", strings.NewReader(`{"model":"m","stream":true,"max_tokens":4096}`))
		req.Header.Set("x-api-key", tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		require.Eventually(t, func() bool { u, _ := p.Usage(tok); return u.InputTokens > 0 || u.Estimated > 0 }, 3*time.Second, 5*time.Millisecond)
		return p, tok
	}

	t.Run("what was seen", func(t *testing.T) {
		p, tok := cut(t, start+text)
		u, _ := p.Usage(tok)
		assert.Equal(t, int64(120), u.InputTokens, "input read from message_start")
		assert.Equal(t, int64(3000), u.CacheReadTokens)
		assert.Greater(t, u.OutputTokens, int64(1), "the output received is more than the 1 token of message_start")
		assert.LessOrEqual(t, u.OutputTokens, int64(4096), "capped by the output the request allowed")
		assert.Equal(t, int64(1), u.Estimated, "the estimate is flagged")
	})
	t.Run("nothing seen", func(t *testing.T) {
		p, tok := cut(t, "event: ping\ndata: {}\n\n")
		u, _ := p.Usage(tok)
		assert.Greater(t, u.InputTokens, int64(0), "input estimated from the size of the request")
		assert.Equal(t, int64(1), u.Estimated)
	})
	t.Run("complete stream is not estimated", func(t *testing.T) {
		var s seen
		up := upstream(t, &s, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, start+text+"event: message_delta\ndata: {\"usage\":{\"output_tokens\":42}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		})
		p := startProxy(t)
		tok, _ := p.Issue(Grant{Provider: ProviderAnthropic, Upstream: Upstream{BaseURL: up.URL, Auth: HeaderAuth{Name: "x-api-key", Value: "k"}}})
		req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderAnthropic)+"/messages", strings.NewReader(`{"model":"m","stream":true}`))
		req.Header.Set("x-api-key", tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		require.Eventually(t, func() bool { u, _ := p.Usage(tok); return u.OutputTokens == 42 }, 2*time.Second, 5*time.Millisecond)
		u, _ := p.Usage(tok)
		assert.Zero(t, u.Estimated)
	})
}

// A client leaving right after the end of a stream is not a cut stream.
func TestUsageReaderClientLeavesAfterTheEnd(t *testing.T) {
	g := &grantState{}
	var got Usage
	u := &usageReader{rc: io.NopCloser(strings.NewReader("data: {\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n")),
		g: g, stream: true, estimate: true, reqBytes: 500, onReport: func(us Usage) { got = us }}
	buf := make([]byte, 1024)
	_, err := u.Read(buf) // all the body, not the EOF
	require.NoError(t, err)
	require.NoError(t, u.Close())
	assert.Equal(t, Usage{InputTokens: 10, OutputTokens: 5}, got)
}

// The reservation: two concurrent requests that would overrun the token
// budget together are not both admitted; the second one is told to retry,
// and goes through once the first one is over and counted.
func TestProxyReservesTheTokenBudget(t *testing.T) {
	entered, release := make(chan struct{}, 4), make(chan struct{})
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, _ *http.Request) {
		entered <- struct{}{}
		<-release
		fmt.Fprint(w, `{"usage":{"input_tokens":50,"output_tokens":50}}`)
	})
	p := startProxy(t)
	tok, _ := p.Issue(Grant{Provider: ProviderAnthropic, Upstream: Upstream{BaseURL: up.URL, Auth: HeaderAuth{Name: "x-api-key", Value: "k"}}, MaxTokens: 100000})
	body := `{"model":"m","max_tokens":60000}`

	first := make(chan int, 1)
	go func() {
		status, _, _ := call(t, p, tok, ProviderAnthropic, "/messages", body)
		first <- status
	}()
	<-entered // the first request is relayed: its reservation is held

	status, hdr, msg := call(t, p, tok, ProviderAnthropic, "/messages", body)
	assert.Equal(t, http.StatusTooManyRequests, status, msg)
	secs, err := strconv.Atoi(hdr.Get("Retry-After"))
	require.NoError(t, err)
	assert.Positive(t, secs)

	// A smaller request still fits next to it.
	small := make(chan int, 1)
	go func() {
		status, _, _ := call(t, p, tok, ProviderAnthropic, "/messages", `{"model":"m","max_tokens":1000}`)
		small <- status
	}()
	<-entered

	close(release)
	assert.Equal(t, http.StatusOK, <-first)
	assert.Equal(t, http.StatusOK, <-small)

	// Both are over and counted: the reservation is released.
	require.Eventually(t, func() bool {
		g := p.byToken[RefHash(tok)]
		g.mu.Lock()
		defer g.mu.Unlock()
		return g.reserved == 0 && g.usage.Tokens() == 200
	}, 2*time.Second, 5*time.Millisecond)
	status, _, msg = call(t, p, tok, ProviderAnthropic, "/messages", body)
	assert.Equal(t, http.StatusOK, status, msg)
}

// A request alone is never refused for its estimate (soft cap).
func TestProxyAdmitsALoneRequestOverTheEstimate(t *testing.T) {
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"usage":{"input_tokens":5,"output_tokens":5}}`)
	})
	p := startProxy(t)
	tok, _ := p.Issue(Grant{Provider: ProviderAnthropic, Upstream: Upstream{BaseURL: up.URL, Auth: HeaderAuth{Name: "x-api-key", Value: "k"}}, MaxTokens: 1000})
	status, _, msg := call(t, p, tok, ProviderAnthropic, "/messages", `{"model":"m","max_tokens":64000}`)
	assert.Equal(t, http.StatusOK, status, msg)
}

// The Gate sees every model call (not the model list), can refuse it, and
// is told the usage when the request is over, once, whatever its outcome.
func TestProxyGate(t *testing.T) {
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") != "" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{"usage":{"input_tokens":3,"output_tokens":4,"cache_read_input_tokens":5}}`)
	})
	p := startProxy(t)
	var mu sync.Mutex
	var admitted []Admission
	var released []Usage
	var verdict error
	p.Gate = func(_ context.Context, owner string, a Admission) (func(Usage), error) {
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, "g1", owner)
		if verdict != nil {
			return nil, verdict
		}
		admitted = append(admitted, a)
		return func(u Usage) {
			mu.Lock()
			defer mu.Unlock()
			released = append(released, u)
		}, nil
	}
	tok, _ := p.Issue(Grant{SessionID: "g1", Provider: ProviderAnthropic, Upstream: Upstream{BaseURL: up.URL, Auth: HeaderAuth{Name: "x-api-key", Value: "k"}}})
	count := func() (int, int) { mu.Lock(); defer mu.Unlock(); return len(admitted), len(released) }

	status, _, _ := call(t, p, tok, ProviderAnthropic, "/messages", `{"model":"claude-x","max_tokens":777}`)
	assert.Equal(t, http.StatusOK, status)
	require.Eventually(t, func() bool { a, r := count(); return a == 1 && r == 1 }, 2*time.Second, 5*time.Millisecond)
	mu.Lock()
	assert.Equal(t, "claude-x", admitted[0].Model)
	assert.Equal(t, int64(777), admitted[0].MaxOutputTokens)
	assert.Positive(t, admitted[0].InputTokens)
	assert.Equal(t, Usage{InputTokens: 3, OutputTokens: 4, CacheReadTokens: 5}, released[0])
	mu.Unlock()

	// No max_tokens: the default, a failure releases with no usage.
	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderAnthropic)+"/messages?fail=1", strings.NewReader(`{"model":"claude-x"}`))
	req.Header.Set("x-api-key", tok)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Eventually(t, func() bool { a, r := count(); return a == 2 && r == 2 }, 2*time.Second, 5*time.Millisecond)
	mu.Lock()
	assert.Equal(t, int64(DefaultMaxOutputTokens), admitted[1].MaxOutputTokens)
	assert.Equal(t, Usage{}, released[1])
	mu.Unlock()

	// Not a model call: not gated.
	req, _ = http.NewRequest(http.MethodPost, p.BaseURL(ProviderAnthropic)+"/messages/count_tokens", strings.NewReader(`{"model":"claude-x"}`))
	req.Header.Set("x-api-key", tok)
	resp, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	a, _ := count()
	assert.Equal(t, 2, a)

	// Refusals: spent budget (402), momentary (429 + Retry-After).
	mu.Lock()
	verdict = &Refusal{Message: "budget spent"}
	mu.Unlock()
	status, _, msg := call(t, p, tok, ProviderAnthropic, "/messages", `{"model":"claude-x"}`)
	assert.Equal(t, http.StatusPaymentRequired, status)
	assert.Contains(t, msg, "budget spent")
	mu.Lock()
	verdict = &Refusal{Message: "reserved", RetryAfter: 1500 * time.Millisecond}
	mu.Unlock()
	status, hdr, _ := call(t, p, tok, ProviderAnthropic, "/messages", `{"model":"claude-x"}`)
	assert.Equal(t, http.StatusTooManyRequests, status)
	assert.Equal(t, "2", hdr.Get("Retry-After"))
	a, _ = count()
	assert.Equal(t, 2, a, "a refused request is not relayed")
	assert.Equal(t, int64(3), func() int64 { u, _ := p.Usage(tok); return u.Requests }(), "nor counted (2 model calls and the token count)")
}
