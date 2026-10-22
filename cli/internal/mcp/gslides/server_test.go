package gslides

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/mcp/protocol"
)

// seen is one request received by the fake Slides API.
type seen struct {
	Method, Path, RawQuery, Token string
}

type fakeSlides struct {
	srv  *httptest.Server
	mu   sync.Mutex
	reqs []seen
}

// toServer sends every request, whatever its host, to the fake server: the
// handlers call the real API host.
type toServer struct{ target *url.URL }

func (r toServer) RoundTrip(req *http.Request) (*http.Response, error) {
	c := req.Clone(req.Context())
	c.URL.Scheme, c.URL.Host = r.target.Scheme, r.target.Host
	return http.DefaultTransport.RoundTrip(c)
}

// newFakeSlides starts a fake API, points the handlers at it and sets the token.
func newFakeSlides(t *testing.T, handler http.HandlerFunc) *fakeSlides {
	t.Helper()
	f := &fakeSlides{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		f.mu.Lock()
		f.reqs = append(f.reqs, seen{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Authorization")})
		f.mu.Unlock()
		if handler != nil {
			handler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Design system"}`))
	}))
	t.Cleanup(f.srv.Close)
	target, err := url.Parse(f.srv.URL)
	require.NoError(t, err)
	prev := httpClient
	httpClient = &http.Client{Transport: toServer{target}}
	t.Cleanup(func() { httpClient = prev })
	t.Setenv("GOOGLE_ACCESS_TOKEN", "ya29.secret")
	return f
}

func (f *fakeSlides) requests() []seen {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]seen(nil), f.reqs...)
}

func params(t *testing.T, kv map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(kv)
	require.NoError(t, err)
	return b
}

// Every tool asks Google Slides for the expected path, with the bearer token,
// and returns the body as text (A-09).
func TestToolsRequestTheExpectedPath(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		call func(json.RawMessage) (string, error)
		args map[string]any
		path string
	}{
		{"get_presentation", func(a json.RawMessage) (string, error) { return textOf(handleGetPresentation(ctx, a)) },
			map[string]any{"presentation_id": "1AbC_-xyz"}, "/v1/presentations/1AbC_-xyz"},
		{"get_slide", func(a json.RawMessage) (string, error) { return textOf(handleGetSlide(ctx, a)) },
			map[string]any{"presentation_id": "1AbC", "slide_id": "g1a2b3_0_0"}, "/v1/presentations/1AbC/pages/g1a2b3_0_0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeSlides(t, nil)
			got, err := tc.call(params(t, tc.args))
			require.NoError(t, err)
			assert.JSONEq(t, `{"name":"Design system"}`, got)
			reqs := f.requests()
			require.Len(t, reqs, 1)
			assert.Equal(t, "GET", reqs[0].Method)
			assert.Equal(t, tc.path, reqs[0].Path)
			assert.Equal(t, "Bearer ya29.secret", reqs[0].Token)
		})
	}
}

// textOf is the text of the single block a tool answers with.
func textOf(res *protocol.ToolResult, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if len(res.Content) != 1 || res.Content[0].Type != "text" {
		return "", assert.AnError
	}
	return res.Content[0].Text, nil
}

// A slide id keeps to its path segment: the ":" object ids may hold is legal.
func TestSlideIDWithColonIsAccepted(t *testing.T) {
	f := newFakeSlides(t, nil)
	_, err := handleGetSlide(context.Background(), params(t, map[string]any{"presentation_id": "p", "slide_id": "SLIDES_API1:2-3"}))
	require.NoError(t, err)
	reqs := f.requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "/v1/presentations/p/pages/SLIDES_API1:2-3", reqs[0].Path)
	assert.Empty(t, reqs[0].RawQuery)
}

// An identifier that is not one reaches no resource: ".." would climb one
// segment of the path (url.PathEscape leaves it as it is), a slash or a query
// separator would leave the segment. No request is sent.
func TestTrappedIdentifiersSendNoRequest(t *testing.T) {
	ctx := context.Background()
	trapped := []string{
		"", ".", "..", "...", "../x", "a/b", "a/../b", "a?x=1", "a#y", "a%2F..", "%2e%2e", "a b", " a", "a\n",
		"a\\b", "a.b", "a\x00", "é", "http://evil.example/x", strings.Repeat("a", 129),
	}
	f := newFakeSlides(t, nil)
	for _, id := range trapped {
		_, err := handleGetPresentation(ctx, params(t, map[string]any{"presentation_id": id}))
		require.Error(t, err, "get_presentation %q", id)
		assert.Contains(t, err.Error(), "invalid presentation id", "%q", id)

		_, err = handleGetSlide(ctx, params(t, map[string]any{"presentation_id": id, "slide_id": "g1"}))
		require.Error(t, err, "get_slide presentation %q", id)
		assert.Contains(t, err.Error(), "invalid presentation id", "%q", id)
	}
	for _, id := range append([]string{":a"}, trapped...) {
		_, err := handleGetSlide(ctx, params(t, map[string]any{"presentation_id": "p", "slide_id": id}))
		require.Error(t, err, "get_slide slide %q", id)
		assert.Contains(t, err.Error(), "invalid slide id", "%q", id)
	}
	assert.Empty(t, f.requests(), "no request reached Google Slides")
}

// Without a token no request leaves; an unreadable argument is refused.
func TestMissingTokenAndBadArguments(t *testing.T) {
	ctx := context.Background()
	f := newFakeSlides(t, nil)

	t.Setenv("GOOGLE_ACCESS_TOKEN", "")
	_, err := handleGetPresentation(ctx, params(t, map[string]any{"presentation_id": "p"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GOOGLE_ACCESS_TOKEN")

	t.Setenv("GOOGLE_ACCESS_TOKEN", "ya29.secret")
	_, err = handleGetPresentation(ctx, json.RawMessage(`{"presentation_id": 12}`))
	require.Error(t, err)
	assert.Empty(t, f.requests(), "no request reached Google")
}

// Google's refusal reaches the agent with its status and message, and a body
// larger than the limit is cut, not loaded whole.
func TestAPIErrorsAndLargeBodies(t *testing.T) {
	ctx := context.Background()
	t.Run("an error status carries the body", func(t *testing.T) {
		newFakeSlides(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"Request had invalid authentication credentials"}}`))
		})
		_, err := handleGetPresentation(ctx, params(t, map[string]any{"presentation_id": "p"}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
		assert.Contains(t, err.Error(), "invalid authentication credentials")
	})
	t.Run("a body over the limit is truncated", func(t *testing.T) {
		newFakeSlides(t, func(w http.ResponseWriter, _ *http.Request) {
			chunk := strings.Repeat("x", 1<<20)
			for range maxResponseSize/(1<<20) + 2 {
				if _, err := w.Write([]byte(chunk)); err != nil {
					return
				}
			}
		})
		got, err := textOf(handleGetPresentation(ctx, params(t, map[string]any{"presentation_id": "p"})))
		require.NoError(t, err)
		assert.Len(t, got, maxResponseSize)
	})
	t.Run("an unreachable server is reported", func(t *testing.T) {
		f := newFakeSlides(t, nil)
		f.srv.Close()
		_, err := handleGetPresentation(ctx, params(t, map[string]any{"presentation_id": "p"}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "request failed")
	})
}
