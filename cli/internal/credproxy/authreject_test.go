package credproxy

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A refused credential (401/403) is logged with its key, never the secret,
// recorded on the grant and reported once; a success ends the episode.
func TestProxyTracksCredentialRefusal(t *testing.T) {
	var status atomic.Int32
	status.Store(http.StatusForbidden)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(int(status.Load()))
		if status.Load() >= 300 {
			_, _ = io.WriteString(w, `{"message":"Authentication failed: Please make sure your API Key is valid."}`)
			return
		}
		_, _ = io.WriteString(w, `{"usage":{"inputTokens":1,"outputTokens":1}}`)
	}))
	t.Cleanup(up.Close)

	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	p := startProxy(t)
	got := make(chan AuthRejection, 4)
	p.OnAuthRejected = func(r AuthRejection) { got <- r }
	cred := CredentialRef{Kind: "bearer", Key: "openhub.provider.bedrock.token.p1", Scope: "project", ID: "fp1"}
	tok, err := p.Issue(Grant{SessionID: "g1", Provider: ProviderBedrock, Credential: cred,
		Upstream: Upstream{BaseURL: up.URL, Auth: BearerAuth{Token: "REAL-SECRET"}}})
	require.NoError(t, err)

	call := func() (int, string) {
		req, _ := http.NewRequest(http.MethodPost, p.BaseURL(ProviderBedrock)+"/model/m/converse", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	code, body := call()
	assert.Equal(t, http.StatusForbidden, code)
	assert.Contains(t, body, "API Key is valid", "the tool still gets the provider answer")

	select {
	case r := <-got:
		assert.Equal(t, "g1", r.Owner)
		assert.Equal(t, http.StatusForbidden, r.Status)
		assert.Equal(t, cred, r.Credential)
		assert.Contains(t, r.Message, "Authentication failed")
	case <-time.After(2 * time.Second):
		t.Fatal("refusal not reported")
	}
	rej, ok := p.AuthRejection(tok)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, rej.Status)

	call()
	select {
	case <-got:
		t.Fatal("reported once per episode")
	case <-time.After(100 * time.Millisecond):
	}
	assert.Contains(t, logs.String(), "openhub.provider.bedrock.token.p1")
	assert.Contains(t, logs.String(), "status=403")
	assert.NotContains(t, logs.String(), "REAL-SECRET")

	status.Store(http.StatusOK)
	code, _ = call()
	assert.Equal(t, http.StatusOK, code)
	_, ok = p.AuthRejection(tok)
	assert.False(t, ok, "a success ends the refusal")
}

func TestIsAuthFailure(t *testing.T) {
	assert.True(t, IsAuthFailure(401, ""))
	assert.True(t, IsAuthFailure(403, "Bearer Token has expired"))
	assert.False(t, IsAuthFailure(403, "You don't have access to the model with the specified model ID."))
	assert.False(t, IsAuthFailure(403, "User: x is not authorized to perform: bedrock:ListFoundationModels"))
	assert.False(t, IsAuthFailure(429, ""))
}

func TestUpstreamErrorMessage(t *testing.T) {
	assert.Equal(t, "bad key", UpstreamErrorMessage([]byte(`{"Message":"bad key"}`)))
	assert.Equal(t, "nope", UpstreamErrorMessage([]byte(`{"type":"error","error":{"type":"authentication_error","message":"nope"}}`)))
	assert.Equal(t, "plain text", UpstreamErrorMessage([]byte("plain\n text")))
	long := UpstreamErrorMessage([]byte(strings.Repeat("x", 500)))
	assert.LessOrEqual(t, len([]rune(long)), maxErrorExcerpt+1)
}

func TestCheckBedrockBearer(t *testing.T) {
	var auth, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.RequestURI()
		if auth != "Bearer good" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"message":"Bearer Token has expired"}`)
			return
		}
		_, _ = io.WriteString(w, `{"modelSummaries":[]}`)
	}))
	t.Cleanup(srv.Close)
	ctx := context.Background()
	c, err := CheckBedrockBearer(ctx, srv.Client(), srv.URL, "good")
	require.NoError(t, err)
	assert.True(t, c.Accepted())
	assert.Equal(t, "/foundation-models?byProvider=anthropic", path)
	c, err = CheckBedrockBearer(ctx, srv.Client(), srv.URL, "old")
	require.NoError(t, err)
	assert.False(t, c.Accepted())
	assert.Equal(t, "Bearer Token has expired", c.Message)

	u, err := BedrockControlURL("eu-west-1")
	require.NoError(t, err)
	assert.Equal(t, "https://bedrock.eu-west-1.amazonaws.com", u)
	_, err = BedrockControlURL("x.evil.com#")
	assert.Error(t, err)
}
