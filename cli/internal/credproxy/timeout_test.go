package credproxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A-04: the connection to the providers has a bound on the handshake, on the
// idle connections and on the wait for the start of the answer, and none on
// the whole request.
func TestProxyTransportTimeouts(t *testing.T) {
	p := New()
	assert.Equal(t, 5*time.Minute, p.client.ResponseHeaderTimeout, "slow models take minutes before the first byte")
	assert.Equal(t, 10*time.Second, p.client.TLSHandshakeTimeout)
	assert.Equal(t, 90*time.Second, p.client.IdleConnTimeout)

	p.SetResponseHeaderTimeout(2 * time.Minute)
	assert.Equal(t, 2*time.Minute, p.client.ResponseHeaderTimeout)
	p.SetResponseHeaderTimeout(0)
	assert.Equal(t, DefaultResponseHeaderTimeout, p.client.ResponseHeaderTimeout, "0 = the default")
}

// A-04: a provider that never answers does not hold the request forever.
func TestProxyGivesUpOnAProviderThatNeverAnswers(t *testing.T) {
	var s seen
	gate := make(chan struct{})
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-gate:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(gate) })
	p := New()
	p.SetResponseHeaderTimeout(50 * time.Millisecond)
	require.NoError(t, p.Start("127.0.0.1:0"))
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	tok, _ := p.Issue(Grant{Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "k"}}})

	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(`{"model":"m"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode, string(body))
	assert.Contains(t, string(body), "did not answer in time")
}

// A-04: the bound is on the start of the answer only: a stream that runs
// longer than it is not cut.
func TestProxyStreamHasNoTotalTimeout(t *testing.T) {
	var s seen
	second := make(chan struct{})
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: one\n\n")
		w.(http.Flusher).Flush()
		<-second
		fmt.Fprint(w, "data: two\n\n")
	})
	p := New()
	p.SetResponseHeaderTimeout(30 * time.Millisecond)
	require.NoError(t, p.Start("127.0.0.1:0"))
	t.Cleanup(func() { _ = p.Close(context.Background()) })
	tok, _ := p.Issue(Grant{Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "k"}}})

	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(`{"model":"m","stream":true}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed below
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	r := bufio.NewReader(resp.Body)
	line, err := r.ReadString('\n')
	require.NoError(t, err)
	assert.Contains(t, line, "one")

	// Several times the header bound go by before the second chunk.
	<-time.After(150 * time.Millisecond)
	close(second)
	rest, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Contains(t, string(rest), "two", "the stream went on after the header timeout")
}
