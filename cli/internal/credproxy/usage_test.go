package credproxy

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithStreamUsage(t *testing.T) {
	got := withStreamUsage(ProviderOpenAI, "chat/completions", []byte(`{"model":"m","stream":true,"messages":[]}`))
	var m map[string]any
	require.NoError(t, json.Unmarshal(got, &m))
	assert.Equal(t, map[string]any{"include_usage": true}, m["stream_options"])
	assert.Equal(t, "m", m["model"])

	kept := []byte(`{"stream":true,"stream_options":{"include_usage":true}}`)
	assert.Equal(t, kept, withStreamUsage(ProviderOpenRouter, "chat/completions", kept))
	got = withStreamUsage(ProviderOpenRouter, "chat/completions", []byte(`{"stream":true,"stream_options":{"x":1}}`))
	assert.Contains(t, string(got), `"include_usage":true`)
	assert.Contains(t, string(got), `"x":1`)
	for _, b := range []string{`{"stream":false}`, `{"model":"m"}`, `not json`} {
		assert.Equal(t, b, string(withStreamUsage(ProviderOpenAI, "chat/completions", []byte(b))))
	}
	assert.Equal(t, `{"stream":true}`, string(withStreamUsage(ProviderAnthropic, "messages", []byte(`{"stream":true}`))))
	assert.Equal(t, `{"stream":true}`, string(withStreamUsage(ProviderOpenAI, "responses", []byte(`{"stream":true}`))))
}

// The OpenAI stream only carries usage when asked: the proxy asks, counts it,
// and reports it to the ledger.
func TestProxyCountsOpenAIStreamUsage(t *testing.T) {
	var sawOptions bool
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			StreamOptions struct {
				IncludeUsage bool `json:"include_usage"`
			} `json:"stream_options"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		sawOptions = req.StreamOptions.IncludeUsage
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		for i := 0; i < 3; i++ {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"x%d\"}}]}\n\n", i)
			fl.Flush()
		}
		if sawOptions {
			fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":42,\"completion_tokens\":7}}\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(up.Close)
	p := startProxy(t)
	var mu sync.Mutex
	var reported Usage
	p.OnUsage = func(owner string, d Usage) {
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, "g1", owner)
		reported.Requests += d.Requests
		reported.InputTokens += d.InputTokens
		reported.OutputTokens += d.OutputTokens
	}
	tok, err := p.Issue(Grant{SessionID: "g1", Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "real"}}})
	require.NoError(t, err)
	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(`{"model":"gpt","stream":true}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	out, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.True(t, sawOptions)
	assert.Contains(t, string(out), "[DONE]")
	u, _ := p.Usage(tok)
	assert.Equal(t, Usage{Requests: 1, InputTokens: 42, OutputTokens: 7}, u)
	mu.Lock()
	assert.Equal(t, Usage{Requests: 1, InputTokens: 42, OutputTokens: 7}, reported)
	mu.Unlock()

	// A new token of the group continues the counters (restart, re-issue).
	tok2, err := p.Issue(Grant{SessionID: "g1", Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "real"}}})
	require.NoError(t, err)
	p.SetUsage(TokenHash(tok2), u)
	u2, _ := p.Usage(tok2)
	assert.Equal(t, u, u2)
}

// eventFrame encodes an AWS event-stream frame (no headers).
func eventFrame(payload []byte) []byte {
	total := 16 + len(payload)
	var b bytes.Buffer
	prelude := make([]byte, 8)
	binary.BigEndian.PutUint32(prelude[0:4], uint32(total))
	binary.BigEndian.PutUint32(prelude[4:8], 0)
	b.Write(prelude)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(prelude))
	b.Write(payload)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(b.Bytes()))
	return b.Bytes()
}

func chunkFrame(t *testing.T, chunk string) []byte {
	payload, err := json.Marshal(map[string]string{"bytes": base64.StdEncoding.EncodeToString([]byte(chunk))})
	require.NoError(t, err)
	return eventFrame(payload)
}

// Bedrock InvokeModel with response stream hides the usage in base64 chunks
// inside binary frames; the response arrives in arbitrary pieces.
func TestProxyCountsBedrockInvokeStreamUsage(t *testing.T) {
	var stream []byte
	for _, c := range []string{
		`{"type":"message_start","message":{"usage":{"input_tokens":120,"output_tokens":1}}}`,
		`{"type":"content_block_delta","delta":{"text":"hello"}}`,
		`{"type":"message_delta","usage":{"output_tokens":33}}`,
		`{"type":"message_stop","amazon-bedrock-invocationMetrics":{"inputTokenCount":120,"outputTokenCount":33}}`,
	} {
		stream = append(stream, chunkFrame(t, c)...)
	}
	assert.NotContains(t, string(stream), "input_tokens", "hidden in base64")
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
		fl := w.(http.Flusher)
		for i := 0; i < len(stream); i += 7 { // frames split across writes
			end := min(i+7, len(stream))
			_, _ = w.Write(stream[i:end])
			fl.Flush()
		}
	}))
	t.Cleanup(up.Close)
	p := startProxy(t)
	tok, err := p.Issue(Grant{Provider: ProviderBedrock, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "real"}}})
	require.NoError(t, err)
	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderBedrock)+"/model/eu.anthropic.claude-haiku-4-5-20251001-v1%3A0/invoke-with-response-stream", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	got, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, stream, got, "relayed unchanged")
	u, _ := p.Usage(tok)
	assert.Equal(t, int64(120), u.InputTokens)
	assert.Equal(t, int64(33), u.OutputTokens)
}

// Converse streams carry plain JSON in frames: counted as before.
func TestProxyCountsBedrockConverseStreamUsage(t *testing.T) {
	stream := append(eventFrame([]byte(`{"delta":{"text":"hi"}}`)), eventFrame([]byte(`{"usage":{"inputTokens":9,"outputTokens":4,"totalTokens":13}}`))...)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(stream) }))
	t.Cleanup(up.Close)
	p := startProxy(t)
	tok, err := p.Issue(Grant{Provider: ProviderBedrock, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "real"}}})
	require.NoError(t, err)
	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderBedrock)+"/model/m/converse-stream", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	u, _ := p.Usage(tok)
	assert.Equal(t, int64(9), u.InputTokens)
	assert.Equal(t, int64(4), u.OutputTokens)
}
