package credproxy

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// relayed sends a request through the proxy of a provider whose upstream is
// a test server and returns what the upstream received.
func relayed(t *testing.T, provider, auth, method, path string, headers map[string]string) *seen {
	t.Helper()
	var s seen
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "{}") })
	p := startProxy(t)
	var up2 Upstream
	switch provider {
	case ProviderAnthropic:
		up2 = Upstream{BaseURL: up.URL + "/v1", Auth: HeaderAuth{Name: "x-api-key", Value: "REAL-KEY"}}
	default:
		up2 = Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "REAL-KEY"}}
	}
	tok, err := p.Issue(Grant{SessionID: "ses_1", Provider: provider, Upstream: up2})
	require.NoError(t, err)

	var body io.Reader = http.NoBody
	if method == http.MethodPost {
		body = strings.NewReader(`{"model":"m","max_tokens":10}`)
	}
	req, err := http.NewRequest(method, p.BaseURL(provider)+path, body)
	require.NoError(t, err)
	if auth == "x-api-key" {
		req.Header.Set("x-api-key", tok)
	} else {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return &s
}

// headerNames lists what the upstream received, without the headers the
// transports add on their own and the real credential.
func headerNames(h http.Header) []string {
	var out []string
	for k := range h {
		switch k {
		case "Content-Length", "Accept-Encoding", "Authorization", "X-Api-Key":
			continue
		}
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// What the real clients send is relayed, what the agent adds is not (S-28).
func TestRelayHeadersOfEachProvider(t *testing.T) {
	forged := map[string]string{
		"OpenAI-Organization":                       "org-other",
		"OpenAI-Project":                            "proj-other",
		"OpenAI-Beta":                               "assistants=v2",
		"X-Provider-API-Keys":                       `{"openai":"sk-evil"}`,
		"X-Amzn-Bedrock-GuardrailIdentifier":        "other",
		"X-Amzn-Bedrock-Service-Tier":               "priority",
		"X-Amzn-Bedrock-PerformanceConfig-Latency":  "optimized",
		"X-Amz-Bedrock-Region":                      "us-west-2",
		"Anthropic-Dangerous-Direct-Browser-Access": "true",
		"X-Forwarded-For":                           "203.0.113.9",
		"Cookie":                                    "session=1",
		"X-Custom-Option":                           "1",
		"Content-Encoding":                          "gzip",
	}
	cases := []struct {
		provider, auth, method, path string
		sent                         map[string]string // what the real client sends
		kept                         []string          // what must reach the provider
		notKept                      []string          // what the provider must not see from the forged set
	}{
		{
			provider: ProviderAnthropic, auth: "x-api-key", method: http.MethodPost, path: "/messages?beta=true",
			sent: map[string]string{
				"Content-Type": "application/json", "Accept": "application/json", "User-Agent": "opencode/2.0.20",
				"anthropic-version": "2023-06-01", "anthropic-beta": "interleaved-thinking-2025-05-14",
				"X-Stainless-Lang": "js", "X-Stainless-Retry-Count": "0", "X-Stainless-OS": "MacOS",
			},
			kept: []string{"Accept", "Anthropic-Beta", "Anthropic-Version", "Content-Type", "User-Agent", "X-Stainless-Lang", "X-Stainless-Os", "X-Stainless-Retry-Count"},
		},
		{
			provider: ProviderBedrock, auth: "bearer", method: http.MethodPost, path: "/model/eu.anthropic.claude-haiku-4-5-20251001-v1%3A0/converse-stream",
			sent: map[string]string{
				"Content-Type": "application/json", "Accept": "application/vnd.amazon.eventstream", "User-Agent": "opencode/2.0.20",
				"X-Amzn-Bedrock-Accept": "application/json",
				// signed by the tool: replaced by the credential of oh
				"X-Amz-Date": "20260101T000000Z", "X-Amz-Security-Token": "tool", "X-Amz-Content-Sha256": "abc",
			},
			kept: []string{"Accept", "Content-Type", "User-Agent", "X-Amzn-Bedrock-Accept"},
		},
		{
			provider: ProviderOpenAI, auth: "bearer", method: http.MethodPost, path: "/responses",
			sent: map[string]string{
				"Content-Type": "application/json", "Accept": "text/event-stream", "User-Agent": "opencode/2.0.20",
				"X-Session-Id": "ses_1", "X-Session-Affinity": "ses_1", "X-Opencode-Session": "ses_1", "X-Parent-Session-Id": "ses_0",
			},
			kept: []string{"Accept", "Content-Type", "User-Agent", "X-Session-Affinity", "X-Session-Id"},
		},
		{
			provider: ProviderOpenRouter, auth: "bearer", method: http.MethodPost, path: "/chat/completions",
			sent: map[string]string{
				"Content-Type": "application/json", "Accept": "text/event-stream", "User-Agent": "opencode/2.0.20",
				"HTTP-Referer": "https://opencode.ai/", "X-Title": "opencode", "X-Session-Id": "ses_1",
			},
			kept: []string{"Accept", "Content-Type", "Http-Referer", "User-Agent", "X-Session-Id", "X-Title"},
		},
	}
	for _, c := range cases {
		t.Run(c.provider, func(t *testing.T) {
			all := map[string]string{}
			for k, v := range c.sent {
				all[k] = v
			}
			for k, v := range forged {
				all[k] = v
			}
			s := relayed(t, c.provider, c.auth, c.method, c.path, all)
			s.mu.Lock()
			defer s.mu.Unlock()
			assert.Equal(t, c.kept, headerNames(s.headers), "only the headers of the provider reach it")
			for k, v := range c.sent {
				if slices.Contains(c.kept, http.CanonicalHeaderKey(k)) {
					assert.Equal(t, v, s.headers.Get(k), k)
				}
			}
			assert.Empty(t, s.headers.Get("Cookie"))
			assert.Empty(t, s.headers.Get("OpenAI-Organization"))
			assert.Empty(t, s.headers.Get("X-Provider-API-Keys"))
			if c.auth == "x-api-key" {
				assert.Equal(t, "REAL-KEY", s.headers.Get("x-api-key"))
			} else {
				assert.Equal(t, "Bearer REAL-KEY", s.headers.Get("Authorization"))
			}
		})
	}
}

// A header that is not in the list is not relayed, a header that is, is.
func TestRelayHeaderUnknownDropped(t *testing.T) {
	s := relayed(t, ProviderAnthropic, "x-api-key", http.MethodPost, "/messages", map[string]string{
		"X-Unknown": "1", "anthropic-beta": "context-1m-2025-08-07",
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Empty(t, s.headers.Get("X-Unknown"))
	assert.Equal(t, "context-1m-2025-08-07", s.headers.Get("anthropic-beta"))
}

// The beta header of Anthropic is not relayed to another provider, nor the
// session headers to Anthropic or Bedrock.
func TestRelayHeadersAreScopedToTheProvider(t *testing.T) {
	assert.NotContains(t, RelayHeaders(ProviderOpenAI), "Anthropic-Beta")
	assert.NotContains(t, RelayHeaders(ProviderBedrock), "Anthropic-Beta")
	assert.NotContains(t, RelayHeaders(ProviderAnthropic), "X-Session-Id")
	assert.NotContains(t, RelayHeaders(ProviderBedrock), "X-Session-Id")
	assert.Equal(t, []string{"Accept", "Content-Type", "User-Agent"}, RelayHeaders("unknown"))
}

// No header that carries a credential or picks the account, the project, the
// tier or the region is ever on a list.
func TestRelayHeadersNeverCarryCredentialsOrAccount(t *testing.T) {
	forbidden := []string{
		"Authorization", "Proxy-Authorization", "X-Api-Key", "Cookie", "Accept-Encoding", "Content-Encoding",
		"Openai-Organization", "Openai-Project", "Openai-Beta", "X-Provider-Api-Keys",
		"X-Amz-Security-Token", "X-Amz-Date", "X-Amz-Content-Sha256", "X-Amz-Target",
		"X-Amzn-Bedrock-Guardrailidentifier", "X-Amzn-Bedrock-Service-Tier", "X-Amzn-Bedrock-Performanceconfig-Latency",
		"X-Forwarded-For", "X-Goog-User-Project", "Anthropic-Dangerous-Direct-Browser-Access",
	}
	for _, p := range []string{ProviderAnthropic, ProviderBedrock, ProviderOpenAI, ProviderOpenRouter} {
		got := RelayHeaders(p)
		for _, h := range forbidden {
			assert.NotContains(t, got, h, p)
		}
		assert.True(t, slices.IsSorted(got))
	}
}

func TestRelayQuery(t *testing.T) {
	cases := []struct {
		name, provider, method, path, query, want string
	}{
		{"anthropic beta kept", ProviderAnthropic, http.MethodPost, "/messages", "beta=true", "beta=true"},
		{"anthropic beta kept on count_tokens", ProviderAnthropic, http.MethodPost, "/messages/count_tokens", "beta=true", "beta=true"},
		{"anthropic unknown parameter removed", ProviderAnthropic, http.MethodPost, "/messages", "beta=true&debug=1&region=us-west-2", "beta=true"},
		{"anthropic beta with another value removed", ProviderAnthropic, http.MethodPost, "/messages", "beta=false", ""},
		{"anthropic parameter repeated kept once", ProviderAnthropic, http.MethodPost, "/messages", "beta=true&beta=true", "beta=true"},
		{"anthropic escaped key", ProviderAnthropic, http.MethodPost, "/messages", "%62eta=true&x=1", "%62eta=true"},
		{"anthropic list pagination kept", ProviderAnthropic, http.MethodGet, "/models", "limit=5&after_id=abc&beta=true", "limit=5&after_id=abc"},
		{"anthropic model get takes none", ProviderAnthropic, http.MethodGet, "/models/claude-x", "limit=5", ""},
		{"bedrock takes none", ProviderBedrock, http.MethodPost, "/model/m/converse", "x=1&trace=ENABLED", ""},
		{"openai takes none", ProviderOpenAI, http.MethodPost, "/responses", "api-version=2025-01-01", ""},
		{"openrouter takes none", ProviderOpenRouter, http.MethodPost, "/chat/completions", "beta=true", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := relayed(t, c.provider, "x-api-key", c.method, c.path+"?"+c.query, nil)
			s.mu.Lock()
			defer s.mu.Unlock()
			assert.Equal(t, c.want, s.query)
		})
	}
}

func TestFilterQuery(t *testing.T) {
	allowed := map[string][]string{"beta": {"true"}, "limit": nil}
	for _, c := range []struct {
		raw, kept string
		dropped   []string
	}{
		{"", "", nil},
		{"beta=true", "beta=true", nil},
		{"limit=%35", "limit=%35", nil},
		{"limit", "limit", nil},
		{"x=1&beta=true&y", "beta=true", []string{"x", "y"}},
		{"beta=true;limit=1", "", []string{"beta"}},
		{"beta=%zz", "", []string{"beta"}},
		{"limit=1&limit=2", "limit=1", []string{"limit"}},
		{"&&", "", nil},
	} {
		kept, dropped := filterQuery(allowed, c.raw)
		assert.Equal(t, c.kept, kept, c.raw)
		assert.Equal(t, c.dropped, dropped, c.raw)
	}
}

// What is dropped is journaled at debug level, by name and never by value.
func TestRelayDroppedAreLoggedWithoutValues(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	relayed(t, ProviderOpenAI, "bearer", http.MethodPost, "/responses?org=secret-org-value", map[string]string{
		"OpenAI-Organization": "org-secret-header-value",
		"Content-Type":        "application/json",
	})
	logs := buf.String()
	assert.Contains(t, logs, "not relayed")
	assert.Contains(t, logs, "Openai-Organization")
	assert.Contains(t, logs, "org")
	assert.NotContains(t, logs, "org-secret-header-value")
	assert.NotContains(t, logs, "secret-org-value")
	assert.NotContains(t, logs, "REAL-KEY")
}

func TestLogDroppedBoundsTheNames(t *testing.T) {
	var many []string
	for i := range 50 {
		many = append(many, fmt.Sprintf("X-%d-%s", i, strings.Repeat("a", 200)))
	}
	got := cap20(many)
	require.Len(t, got, maxLoggedDropped+1)
	assert.Equal(t, "…", got[maxLoggedDropped])
	assert.LessOrEqual(t, len([]rune(got[0])), maxDroppedName+1)
}
