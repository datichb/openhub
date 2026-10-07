//go:build integration

package opencodev2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// QB8 (S8): the session context capability writes and clears instruction
// entries on a real server.
func TestContractSessionContext(t *testing.T) {
	cfg := `{"agents":{"pinger":{"mode":"primary","description":"p"}}}`
	s := startLiveServer(t, cfg, "pinger")
	ctx := context.Background()
	ad := &Adapter{}
	require.True(t, ad.Capabilities().SessionContext)
	h := adapters.ServerHandle{URL: s.URL, Password: s.Password, PID: s.PID}
	sid := sessionspec.NewSessionID()
	require.NoError(t, ad.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: sid, EntryAgent: "pinger", Location: s.project}))

	entries := func() map[string]json.RawMessage {
		var page struct {
			Data []struct {
				Key   string          `json:"key"`
				Value json.RawMessage `json:"value"`
			} `json:"data"`
		}
		require.NoError(t, s.client.do(ctx, http.MethodGet, "/api/experimental/session/"+sid+"/instructions/entries", nil, nil, &page))
		out := map[string]json.RawMessage{}
		for _, e := range page.Data {
			out[e.Key] = e.Value
		}
		return out
	}
	require.NoError(t, ad.SetSessionContext(ctx, h, sid, "oh.checkpoints", map[string]any{"passed": []string{"cp-1"}}))
	got := entries()
	require.Contains(t, got, "oh.checkpoints")
	assert.JSONEq(t, `{"passed":["cp-1"]}`, string(got["oh.checkpoints"]))

	require.NoError(t, ad.ClearSessionContext(ctx, h, sid, "oh.checkpoints"))
	assert.NotContains(t, entries(), "oh.checkpoints")
	require.NoError(t, ad.ClearSessionContext(ctx, h, sid, "oh.checkpoints"), "already absent")

	assert.Error(t, ad.SetSessionContext(ctx, h, sid, "Bad Key", 1), "key format checked by the tool")
}

// QB8: a server without the experimental route (404 without error tag, or
// 405) makes the capability unsupported: the caller falls back.
func TestSessionContextUnsupportedRoute(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusMethodNotAllowed} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte("Not Found"))
		}))
		err := (&Adapter{}).SetSessionContext(context.Background(), adapters.ServerHandle{URL: srv.URL}, "ses_x", "oh.budget", 1)
		srv.Close()
		assert.True(t, errors.Is(err, adapters.ErrUnsupported), "%d: %v", status, err)
	}
}
