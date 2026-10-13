package credproxy

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a log sink safe for the handlers of the proxy.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) take() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.b.String()
	s.b.Reset()
	return out
}

// A 429 of the provider is logged (warning for the first of a row, debug
// after, a success ends the row), without the secret; the tool still gets
// the provider answer.
func TestProxyLogsRateLimits(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusTooManyRequests)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if status.Load() == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "17")
			w.Header().Set("Request-Id", "req_42")
			w.Header().Set("Anthropic-Ratelimit-Tokens-Reset", "2026-10-13T12:00:30Z")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"rate_limit_error","message":"Number of request tokens has exceeded your per-minute rate limit"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"usage":{"input_tokens":1,"output_tokens":1}}`)
	}))
	t.Cleanup(up.Close)

	var logs syncBuffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	p := startProxy(t)
	secret := "sk-" + "ant-" + "NOT-A-REAL-KEY"
	tok, err := p.Issue(Grant{SessionID: "ses_a", Provider: ProviderAnthropic,
		Upstream: Upstream{BaseURL: up.URL, Auth: HeaderAuth{Name: "x-api-key", Value: secret}}})
	require.NoError(t, err)
	call := func() (int, string) {
		req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderAnthropic)+"/messages", strings.NewReader(`{"model":"claude-x"}`))
		req.Header.Set("x-api-key", tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	code, body := call()
	assert.Equal(t, http.StatusTooManyRequests, code)
	assert.Contains(t, body, "rate_limit_error", "the tool gets the provider answer")
	first := logs.take()
	for _, want := range []string{"level=WARN", `msg="credproxy: upstream rate limit"`, "provider=anthropic", "owner=ses_a", "status=429",
		"in_a_row=1", "model=claude-x", "retry_after=17", "request_id=req_42", "ratelimit_tokens_reset=2026-10-13T12:00:30Z",
		"per-minute rate limit"} {
		assert.Contains(t, first, want)
	}
	assert.NotContains(t, first, "NOT-A-REAL-KEY")

	call()
	again := logs.take()
	assert.Contains(t, again, "level=DEBUG")
	assert.Contains(t, again, "in_a_row=2")
	assert.NotContains(t, again, "level=WARN")

	status.Store(http.StatusOK)
	code, _ = call()
	assert.Equal(t, http.StatusOK, code)
	assert.NotContains(t, logs.take(), "rate limit")
	status.Store(http.StatusTooManyRequests)
	call()
	assert.Contains(t, logs.take(), "level=WARN", "a success ended the row: a new warning")
}
