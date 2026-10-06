package credproxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
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
	assert.False(t, p.Exhausted(tok))
	assert.Equal(t, http.StatusOK, call())
	assert.Eventually(t, func() bool { return p.Exhausted(tok) }, 2*time.Second, 20*time.Millisecond)
	assert.Equal(t, http.StatusTooManyRequests, call(), "110 tokens used ≥ budget 100")
	assert.False(t, p.Exhausted("ohs_unknown"))
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
	m, err := parsePath(ProviderBedrock, http.MethodPost, "model/eu.anthropic.claude-haiku-4-5-20251001-v1%3A0/converse-stream")
	require.NoError(t, err)
	assert.Equal(t, "eu.anthropic.claude-haiku-4-5-20251001-v1:0", m)
	// An inference profile ARN keeps its escaped slash, decoded once.
	m, err = parsePath(ProviderBedrock, http.MethodPost, "model/arn%3Aaws%3Abedrock%3Aeu-west-1%3A1%3Ainference-profile%2Feu.x/converse")
	require.NoError(t, err)
	assert.Equal(t, "arn:aws:bedrock:eu-west-1:1:inference-profile/eu.x", m)
	m, err = bodyModel([]byte(`{"messages":[{"model":"inner"}],"model":"gpt-5"}`))
	require.NoError(t, err)
	assert.Equal(t, "gpt-5", m)
	m, err = bodyModel(nil)
	require.NoError(t, err)
	assert.Empty(t, m)
	for _, b := range []string{`not json`, `[]`, `{"model":"a","model":"b"}`, `{"Model":"a"}`, `{"model":"a","MODEL":"b"}`, `{"model":1}`} {
		_, err := bodyModel([]byte(b))
		assert.ErrorIs(t, err, errModel, b)
	}
	assert.True(t, modelAllowed(nil, "anything"))
	assert.True(t, modelAllowed([]string{"eu.anthropic.*"}, "eu.anthropic.claude-haiku-4-5-20251001-v1:0"))
	assert.False(t, modelAllowed([]string{"eu.anthropic.*"}, "us.anthropic.x"))
}

func TestProxyMountServesOtherServices(t *testing.T) {
	p := startProxy(t)
	p.Mount("oh-gateway", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "gw:"+r.URL.Path)
	}))
	resp, err := http.Post(p.URL()+"/oh-gateway/beads/v1/exec", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "gw:/beads/v1/exec", string(b))

	resp, err = http.Post(p.URL()+"/amazon-bedrock/model/m/converse", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "providers still need a token")
}

func TestPathAllowList(t *testing.T) {
	ok := []struct{ provider, method, path string }{
		{ProviderBedrock, http.MethodPost, "model/m/converse"},
		{ProviderBedrock, http.MethodPost, "model/m/invoke-with-response-stream"},
		{ProviderAnthropic, http.MethodPost, "messages"},
		{ProviderAnthropic, http.MethodGet, "models/claude"},
		{ProviderOpenAI, http.MethodPost, "chat/completions"},
		{ProviderOpenAI, http.MethodPost, "responses"},
		{ProviderOpenRouter, http.MethodGet, "models"},
	}
	for _, c := range ok {
		_, err := parsePath(c.provider, c.method, c.path)
		assert.NoError(t, err, c.path)
	}
	bad := []struct{ provider, method, path string }{
		{ProviderBedrock, http.MethodGet, "model/m/converse"},         // method
		{ProviderBedrock, http.MethodPost, "model/m/../x/converse"},   // dot segment
		{ProviderBedrock, http.MethodPost, "model/m/%2E%2E/converse"}, // escaped dot segment
		{ProviderBedrock, http.MethodPost, "model//converse"},         // empty segment
		{ProviderBedrock, http.MethodPost, "model/m%252Fx/converse"},  // double escaping
		{ProviderBedrock, http.MethodPost, "guardrail/g/version/1/apply"},
		{ProviderAnthropic, http.MethodPost, "messages/batches"},
		{ProviderAnthropic, http.MethodPost, "messages/"},
		{ProviderAnthropic, http.MethodGet, "models/a%2Fb"},
		{ProviderOpenAI, http.MethodPost, "files"},
		{ProviderOpenAI, http.MethodPost, "chat%2Fcompletions"},
		{ProviderOpenAI, http.MethodPost, ""},
		{"unknown", http.MethodPost, "messages"},
	}
	for _, c := range bad {
		_, err := parsePath(c.provider, c.method, c.path)
		assert.ErrorIs(t, err, errPath, c.path)
	}
}

func TestProxyRefusesUnlistedPathsAndAmbiguousModels(t *testing.T) {
	up, hits := countingUpstream(t)
	p := startProxy(t)
	tok, err := p.Issue(Grant{Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "real"}}, AllowedModels: []string{"gpt-5"}})
	require.NoError(t, err)
	do := func(method, path, body string) int {
		req, _ := http.NewRequest(method, p.BaseURL(ProviderOpenAI)+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		return resp.StatusCode
	}
	assert.Equal(t, http.StatusNotFound, do(http.MethodPost, "/files", `{"model":"gpt-5"}`))
	assert.Equal(t, http.StatusNotFound, do(http.MethodPost, "/chat/../files", `{"model":"gpt-5"}`))
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPost, "/chat/completions", `{"model":"gpt-5","model":"gpt-4o"}`))
	assert.Equal(t, http.StatusBadRequest, do(http.MethodPost, "/chat/completions", `{"model":"gpt-5","Model":"gpt-4o"}`))
	assert.Equal(t, http.StatusOK, do(http.MethodPost, "/chat/completions", `{"model":"gpt-5"}`))
	assert.Equal(t, int32(1), hits.Load(), "only the allowed request reached the provider")
}

func TestProxyBodyTooLarge(t *testing.T) {
	up, hits := countingUpstream(t)
	p := startProxy(t)
	tok, err := p.Issue(Grant{Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "real"}}})
	require.NoError(t, err)
	body := `{"model":"m","x":"` + strings.Repeat("a", MaxRequestBytes) + `"}`
	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	assert.Equal(t, int32(0), hits.Load())
}

type failingAuth struct{}

func (failingAuth) Apply(*http.Request, []byte) error { return errors.New("credentials expired") }

func TestProxyRefusesWhenCredentialsFail(t *testing.T) {
	up, hits := countingUpstream(t)
	p := startProxy(t)
	tok, err := p.Issue(Grant{Provider: ProviderBedrock, Upstream: Upstream{BaseURL: up.URL, Auth: failingAuth{}}})
	require.NoError(t, err)
	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderBedrock)+"/model/m/converse", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	assert.Equal(t, int32(0), hits.Load(), "nothing sent without credentials")
}

func TestProxyHoldWaitsForRestoredGrants(t *testing.T) {
	up, hits := countingUpstream(t)
	p := startProxy(t)
	p.Hold()
	tok := NewToken()
	done := make(chan int, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(`{"model":"m"}`))
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			done <- 0
			return
		}
		resp.Body.Close()
		done <- resp.StatusCode
	}()
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, p.IssueWithToken(tok, Grant{Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "real"}}}))
	p.Release()
	select {
	case code := <-done:
		assert.Equal(t, http.StatusOK, code)
	case <-time.After(3 * time.Second):
		t.Fatal("request not released")
	}
	assert.Equal(t, int32(1), hits.Load())
}

func countingUpstream(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(up.Close)
	return up, &hits
}

// M12: the stored hash of a token is not a credential.
func TestProxyRefusesTokenHash(t *testing.T) {
	up, hits := countingUpstream(t)
	p := startProxy(t)
	tok, err := p.Issue(Grant{Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "real"}}})
	require.NoError(t, err)
	assert.True(t, p.Has(TokenHash(tok)))
	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(`{"model":"m"}`))
	req.Header.Set("Authorization", "Bearer "+TokenHash(tok))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, int32(0), hits.Load())
}
