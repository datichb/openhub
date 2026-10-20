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

// longStream is a model stream: it sends a first event, then waits for the
// proxy to cut it (the request context of the upstream ends) or for end to
// be closed. The tests close end in a Cleanup registered after the proxy's,
// so that a failing test does not wait for the proxy to drain the stream.
func longStream(t *testing.T, cut chan<- struct{}, started chan<- struct{}) (*httptest.Server, chan struct{}) {
	t.Helper()
	end := make(chan struct{})
	var s seen
	return upstream(t, &s, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: one\n\n")
		w.(http.Flusher).Flush()
		close(started)
		select {
		case <-r.Context().Done():
			close(cut)
		case <-end:
		}
	}), end
}

// S-28: revoking a token cuts the streams already running with it. Without
// it, a revoked session would go on spending, for minutes, the credential
// the proxy no longer holds.
func TestRevokeCutsTheStreamsInFlight(t *testing.T) {
	for _, revoke := range []struct {
		name string
		do   func(p *Proxy, tok string)
	}{
		{"token", func(p *Proxy, tok string) { p.Revoke(tok) }},
		{"token hash", func(p *Proxy, tok string) { p.Revoke(TokenHash(tok)) }},
		{"session", func(p *Proxy, _ string) { p.RevokeSession("ses_1") }},
	} {
		t.Run(revoke.name, func(t *testing.T) {
			cut, started := make(chan struct{}), make(chan struct{})
			up, end := longStream(t, cut, started)
			p := startProxy(t)
			t.Cleanup(func() { close(end) })
			tok, err := p.Issue(Grant{SessionID: "ses_1", Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "k"}}})
			require.NoError(t, err)

			req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(`{"model":"m","stream":true}`))
			req.Header.Set("Authorization", "Bearer "+tok)
			resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed below
			require.NoError(t, err)
			defer resp.Body.Close()
			r := bufio.NewReader(resp.Body)
			line, err := r.ReadString('\n')
			require.NoError(t, err)
			require.Contains(t, line, "one")
			<-started

			revoke.do(p, tok)

			select {
			case <-cut:
			case <-time.After(10 * time.Second):
				t.Fatal("the upstream request was not cut when the token was revoked")
			}
			// The client does not get a clean end of stream either.
			_, err = io.ReadAll(r)
			assert.Error(t, err, "the stream is aborted, not finished")
			assert.False(t, p.Has(tok))
		})
	}
}

// Revoking a token does not touch the streams of the others.
func TestRevokeKeepsTheOtherStreams(t *testing.T) {
	cut, started := make(chan struct{}), make(chan struct{})
	up, end := longStream(t, cut, started)
	p := startProxy(t)
	t.Cleanup(func() { close(end) })
	keep, _ := p.Issue(Grant{SessionID: "ses_keep", Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "k"}}})
	gone, _ := p.Issue(Grant{SessionID: "ses_gone", Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "k"}}})

	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(`{"model":"m","stream":true}`))
	req.Header.Set("Authorization", "Bearer "+keep)
	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed below
	require.NoError(t, err)
	defer resp.Body.Close()
	<-started

	p.Revoke(gone)
	p.RevokeSession("ses_gone")

	select {
	case <-cut:
		t.Fatal("the stream of another token was cut")
	case <-time.After(150 * time.Millisecond):
	}
	assert.True(t, p.Has(keep))
}

// A token issued again under the same hash (grants restored) is revoked as
// one: the stream running on the state it replaced is cut too.
func TestRevokeCutsStreamsOfAReplacedGrant(t *testing.T) {
	cut, started := make(chan struct{}), make(chan struct{})
	up, end := longStream(t, cut, started)
	p := startProxy(t)
	t.Cleanup(func() { close(end) })
	tok := NewToken()
	grant := Grant{SessionID: "ses_1", Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "k"}}}
	require.NoError(t, p.IssueWithToken(tok, grant))

	req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(`{"model":"m","stream":true}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed below
	require.NoError(t, err)
	defer resp.Body.Close()
	<-started

	require.NoError(t, p.IssueWithToken(tok, grant)) // the credential is replaced
	p.Revoke(tok)

	select {
	case <-cut:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream of the replaced grant was not cut")
	}
}

// A request waiting for a provider that does not answer is released by the
// revocation too.
func TestRevokeReleasesARequestWaitingForTheProvider(t *testing.T) {
	var s seen
	arrived, cut, end := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	up := upstream(t, &s, func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(arrived) })
		select {
		case <-r.Context().Done():
			close(cut)
		case <-end:
		}
	})
	p := startProxy(t)
	t.Cleanup(func() { close(end) })
	tok, _ := p.Issue(Grant{SessionID: "ses_1", Provider: ProviderOpenAI, Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "k"}}})
	done := make(chan int, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderOpenAI)+"/chat/completions", strings.NewReader(`{"model":"m"}`))
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			done <- -1
			return
		}
		resp.Body.Close()
		done <- resp.StatusCode
	}()
	<-arrived
	p.Revoke(tok)
	select {
	case <-cut:
	case <-time.After(10 * time.Second):
		t.Fatal("the request to the provider was not cut")
	}
	assert.Equal(t, http.StatusBadGateway, <-done)
}
