package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// The daemon refuses to stop while tool servers are alive (their sessions go
// through its credential proxy), and the refusal says what it counted: the
// servers, not « sessions ».
func TestShutdownRefusalCountsTheLiveServers(t *testing.T) {
	ctx := context.Background()
	st := sqlitetest.New(t)
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	servers := sqlite.NewServerStore(st)
	for _, g := range []string{"g1", "g2"} {
		require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: g, Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	}
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g3", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerSleeping}))
	d := &Daemon{opts: Options{Servers: servers}}

	rec := httptest.NewRecorder()
	d.handleShutdown(rec, httptest.NewRequest(http.MethodPost, "/shutdown", nil))
	require.Equal(t, http.StatusConflict, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "tool servers alive: 2 (use force)", body["error"], "two live servers; a sleeping one is not counted")
	assert.NotContains(t, body["error"], "sessions")
}
