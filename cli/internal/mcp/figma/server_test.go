package figma

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

// seen is one request received by the fake Figma.
type seen struct {
	Method, Path, RawQuery, Token string
}

type fakeFigma struct {
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

// newFakeFigma starts a fake API, points the handlers at it and sets the token.
func newFakeFigma(t *testing.T, handler http.HandlerFunc) *fakeFigma {
	t.Helper()
	f := &fakeFigma{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		f.mu.Lock()
		f.reqs = append(f.reqs, seen{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("X-Figma-Token")})
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
	t.Setenv("FIGMA_TOKEN", "figd_secret")
	return f
}

func (f *fakeFigma) requests() []seen {
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

// Every tool asks Figma for the expected path, with the token, and returns
// the body as text (A-09).
func TestToolsRequestTheExpectedPath(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name  string
		call  func(json.RawMessage) (string, error)
		args  map[string]any
		path  string
		query string
	}{
		{"get_file", func(a json.RawMessage) (string, error) { return textOf(handleGetFile(ctx, a)) },
			map[string]any{"file_key": "AbC123"}, "/v1/files/AbC123", ""},
		{"get_node", func(a json.RawMessage) (string, error) { return textOf(handleGetNode(ctx, a)) },
			map[string]any{"file_key": "AbC123", "node_id": "12:34"}, "/v1/files/AbC123/nodes", "ids=12%3A34"},
		{"get_styles", func(a json.RawMessage) (string, error) { return textOf(handleGetStyles(ctx, a)) },
			map[string]any{"file_key": "AbC123"}, "/v1/files/AbC123/styles", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeFigma(t, nil)
			got, err := tc.call(params(t, tc.args))
			require.NoError(t, err)
			assert.JSONEq(t, `{"name":"Design system"}`, got)
			reqs := f.requests()
			require.Len(t, reqs, 1)
			assert.Equal(t, "GET", reqs[0].Method)
			assert.Equal(t, tc.path, reqs[0].Path)
			assert.Equal(t, tc.query, reqs[0].RawQuery)
			assert.Equal(t, "figd_secret", reqs[0].Token)
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

// A node id cannot add a parameter to the query.
func TestNodeIDStaysInItsParameter(t *testing.T) {
	f := newFakeFigma(t, nil)
	_, err := handleGetNode(context.Background(), params(t, map[string]any{"file_key": "k", "node_id": "1:2&ids=3"}))
	require.NoError(t, err)

	reqs := f.requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "ids=1%3A2%26ids%3D3", reqs[0].RawQuery)
}

// A file key that is not a key reaches no resource: ".." would climb one
// segment of the path (url.PathEscape leaves it as it is), a slash or a
// query separator would leave the segment. No request is sent.
func TestTrappedFileKeysSendNoRequest(t *testing.T) {
	ctx := context.Background()
	trapped := []string{
		"", ".", "..", "...", "../styles", "a/b", "a/../b", "a?x=1", "a#y", "a%2F..", "%2e%2e", "a b", " a", "a\n",
		"a\\b", "a.b", "a\x00", "é", "a:b", "http://evil.example/x", strings.Repeat("a", 129),
	}
	tools := map[string]func(json.RawMessage) error{
		"get_file":   func(a json.RawMessage) error { _, err := handleGetFile(ctx, a); return err },
		"get_node":   func(a json.RawMessage) error { _, err := handleGetNode(ctx, a); return err },
		"get_styles": func(a json.RawMessage) error { _, err := handleGetStyles(ctx, a); return err },
	}
	f := newFakeFigma(t, nil)
	for _, key := range trapped {
		for name, call := range tools {
			err := call(params(t, map[string]any{"file_key": key, "node_id": "1:2"}))
			require.Error(t, err, "%s %q", name, key)
			assert.Contains(t, err.Error(), "invalid file key", "%s %q", name, key)
		}
	}
	assert.Empty(t, f.requests(), "no request reached Figma")
}

// The keys Figma gives (letters, digits, "-" and "_") go through.
func TestValidFileKeysAreAccepted(t *testing.T) {
	f := newFakeFigma(t, nil)
	for _, key := range []string{"k", "AbC123", "a-b_c", "0", strings.Repeat("a", 128)} {
		_, err := handleGetFile(context.Background(), params(t, map[string]any{"file_key": key}))
		require.NoError(t, err, "%q", key)
	}
	assert.Len(t, f.requests(), 5)
}

// Without a token no request leaves; an unreadable argument is refused.
func TestMissingTokenAndBadArguments(t *testing.T) {
	ctx := context.Background()
	f := newFakeFigma(t, nil)

	t.Setenv("FIGMA_TOKEN", "")
	_, err := handleGetFile(ctx, params(t, map[string]any{"file_key": "k"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FIGMA_TOKEN")

	t.Setenv("FIGMA_TOKEN", "figd_secret")
	_, err = handleGetFile(ctx, json.RawMessage(`{"file_key": 12}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid params")
	assert.Empty(t, f.requests(), "no request reached Figma")
}

// Figma's refusal reaches the agent with its status and message, and a body
// larger than the limit is cut, not loaded whole.
func TestAPIErrorsAndLargeBodies(t *testing.T) {
	ctx := context.Background()
	t.Run("an error status carries the body", func(t *testing.T) {
		newFakeFigma(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"err":"Invalid token"}`))
		})
		_, err := handleGetStyles(ctx, params(t, map[string]any{"file_key": "k"}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
		assert.Contains(t, err.Error(), "Invalid token")
	})
	t.Run("a body over the limit is truncated", func(t *testing.T) {
		newFakeFigma(t, func(w http.ResponseWriter, _ *http.Request) {
			chunk := strings.Repeat("x", 1<<20)
			for range maxResponseSize/(1<<20) + 2 {
				if _, err := w.Write([]byte(chunk)); err != nil {
					return
				}
			}
		})
		got, err := textOf(handleGetFile(ctx, params(t, map[string]any{"file_key": "k"})))
		require.NoError(t, err)
		assert.Len(t, got, maxResponseSize)
	})
	t.Run("an unreachable server is reported", func(t *testing.T) {
		f := newFakeFigma(t, nil)
		f.srv.Close()
		_, err := handleGetFile(ctx, params(t, map[string]any{"file_key": "k"}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "figma API request failed")
	})
}

type hostRecorder struct{ hosts []string }

func (h *hostRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	h.hosts = append(h.hosts, req.URL.Scheme+"://"+req.URL.Host)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`)), Header: http.Header{}, Request: req}, nil
}

// The server always calls the official API, whatever URL is recorded for Figma
// (the environment of the server has none, and the setting is ignored).
func TestServerAlwaysCallsTheOfficialAPI(t *testing.T) {
	rec := &hostRecorder{}
	prev := httpClient
	httpClient = &http.Client{Transport: rec}
	t.Cleanup(func() { httpClient = prev })
	t.Setenv("FIGMA_TOKEN", "figd_secret")
	t.Setenv("FIGMA_URL", "https://figma.internal.example")

	_, err := handleGetFile(context.Background(), params(t, map[string]any{"file_key": "AbC123"}))

	require.NoError(t, err)
	assert.Equal(t, []string{"https://api.figma.com"}, rec.hosts)
}
