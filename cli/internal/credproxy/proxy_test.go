package credproxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type seen struct {
	mu      sync.Mutex
	path    string
	rawPath string
	query   string
	headers http.Header
	body    string
}

func startProxy(t *testing.T) *Proxy {
	t.Helper()
	p := New()
	require.NoError(t, p.Start("127.0.0.1:0"))
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	return p
}

func upstream(t *testing.T, s *seen, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.path, s.rawPath, s.query, s.headers, s.body = r.URL.Path, r.URL.RawPath, r.URL.RawQuery, r.Header.Clone(), string(b)
		s.mu.Unlock()
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProxySwapsCredentialAndKeepsPath(t *testing.T) {
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"output":{},"usage":{"inputTokens":12,"outputTokens":34,"totalTokens":46}}`)
	})
	p := startProxy(t)
	tok, err := p.Issue(Grant{SessionID: "ses_1", Provider: ProviderBedrock, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "REAL-SECRET"}}})
	require.NoError(t, err)

	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderBedrock)+"/model/eu.anthropic.claude-haiku-4-5-20251001-v1%3A0/converse?x=1", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Amz-Date", "20260101T000000Z")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Equal(t, "Bearer REAL-SECRET", s.headers.Get("Authorization"))
	assert.Empty(t, s.headers.Get("X-Amz-Date"), "tool-side signing headers stripped")
	assert.Equal(t, "/model/eu.anthropic.claude-haiku-4-5-20251001-v1:0/converse", s.path)
	assert.Equal(t, "/model/eu.anthropic.claude-haiku-4-5-20251001-v1%3A0/converse", s.rawPath)
	assert.Equal(t, "x=1", s.query)
	assert.Equal(t, `{"messages":[]}`, s.body)

	u, ok := p.Usage(tok)
	require.True(t, ok)
	assert.Equal(t, Usage{Requests: 1, InputTokens: 12, OutputTokens: 34}, u)
}

func TestProxyRejectsUnknownToken(t *testing.T) {
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) { t.Error("upstream must not be called") })
	p := startProxy(t)
	tok, _ := p.Issue(Grant{Provider: ProviderBedrock, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "x"}}})

	for _, c := range []struct{ path, token string }{
		{"/amazon-bedrock/model/m/converse", ""},
		{"/amazon-bedrock/model/m/converse", "ohs_wrong"},
		{"/anthropic/messages", tok}, // right token, wrong provider route
	} {
		req, _ := http.NewRequest(http.MethodPost, p.URL()+c.path, strings.NewReader("{}"))
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, c.path)
	}

	p.Revoke(tok)
	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderBedrock)+"/model/m/converse", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "revoked")
}

func TestProxyAnthropicHeaderAndModelAllowList(t *testing.T) {
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "{}") })
	p := startProxy(t)
	tok, _ := p.Issue(Grant{Provider: ProviderAnthropic, Upstream: Upstream{BaseURL: up.URL + "/v1", Auth: HeaderAuth{Name: "x-api-key", Value: "sk-real"}}, AllowedModels: []string{"claude-haiku-*"}})

	call := func(model string) int {
		req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderAnthropic)+"/messages", strings.NewReader(`{"model":"`+model+`"}`))
		req.Header.Set("x-api-key", tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		return resp.StatusCode
	}
	assert.Equal(t, http.StatusForbidden, call("claude-opus-4-6"))
	assert.Equal(t, http.StatusOK, call("claude-haiku-4-5"))
	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Equal(t, "sk-real", s.headers.Get("x-api-key"))
	assert.Equal(t, "/v1/messages", s.path)
}

func TestProxyStreamsWithoutBuffering(t *testing.T) {
	var s seen
	release := make(chan struct{})
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: message_start\ndata: {\"message\":{\"usage\":{\"input_tokens\":7,\"output_tokens\":1}}}\n\n")
		w.(http.Flusher).Flush()
		<-release
		fmt.Fprint(w, "event: message_delta\ndata: {\"usage\":{\"output_tokens\":21}}\n\n")
	})
	p := startProxy(t)
	tok, _ := p.Issue(Grant{Provider: ProviderAnthropic, Upstream: Upstream{BaseURL: up.URL, Auth: HeaderAuth{Name: "x-api-key", Value: "k"}}})

	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderAnthropic)+"/messages", strings.NewReader(`{"model":"m","stream":true}`))
	req.Header.Set("x-api-key", tok)
	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed below
	require.NoError(t, err)
	defer resp.Body.Close()

	first := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(resp.Body).ReadString('\n')
		first <- line
	}()
	select {
	case line := <-first:
		assert.Contains(t, line, "message_start", "first event delivered before the upstream finished")
	case <-time.After(3 * time.Second):
		t.Fatal("stream was buffered by the proxy")
	}
	close(release)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	require.Eventually(t, func() bool {
		u, _ := p.Usage(tok)
		return u.OutputTokens == 21
	}, 2*time.Second, 20*time.Millisecond)
	u, _ := p.Usage(tok)
	assert.Equal(t, int64(7), u.InputTokens)
}

func TestProxyBudget(t *testing.T) {
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"usage":{"prompt_tokens":60,"completion_tokens":50}}`)
	})
	p := startProxy(t)
	tok, _ := p.Issue(Grant{Provider: ProviderOpenRouter, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "k"}}, MaxTokens: 100})
	call := func() int {
		req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenRouter)+"/chat/completions", strings.NewReader(`{"model":"x"}`))
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.StatusCode
	}
	assert.Equal(t, http.StatusOK, call())
	assert.Equal(t, http.StatusTooManyRequests, call(), "110 tokens used ≥ budget 100")
}

func TestProxyUpstreamDown(t *testing.T) {
	p := startProxy(t)
	tok, _ := p.Issue(Grant{Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: "http://127.0.0.1:1", Auth: BearerAuth{Token: "k"}}})
	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestIssueValidationAndRevokeSession(t *testing.T) {
	p := New()
	_, err := p.Issue(Grant{})
	assert.Error(t, err)
	a, _ := p.Issue(Grant{SessionID: "s1", Provider: "p", Upstream: Upstream{BaseURL: "http://x", Auth: BearerAuth{}}})
	b, _ := p.Issue(Grant{SessionID: "s2", Provider: "p", Upstream: Upstream{BaseURL: "http://x", Auth: BearerAuth{}}})
	assert.True(t, strings.HasPrefix(a, "ohs_"))
	p.RevokeSession("s1")
	_, okA := p.Usage(a)
	_, okB := p.Usage(b)
	assert.False(t, okA)
	assert.True(t, okB)
}

func TestModelExtraction(t *testing.T) {
	assert.Equal(t, "eu.anthropic.claude-haiku-4-5-20251001-v1:0", requestModel("model/eu.anthropic.claude-haiku-4-5-20251001-v1%3A0/converse-stream", nil))
	assert.Equal(t, "gpt-5", requestModel("chat/completions", []byte(`{"model":"gpt-5"}`)))
	assert.Equal(t, "", requestModel("chat/completions", []byte(`not json`)))
	assert.True(t, modelAllowed(nil, "anything"))
	assert.True(t, modelAllowed([]string{"eu.anthropic.*"}, "eu.anthropic.claude-haiku-4-5-20251001-v1:0"))
	assert.False(t, modelAllowed([]string{"eu.anthropic.*"}, "us.anthropic.x"))
}
