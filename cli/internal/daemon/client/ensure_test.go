//go:build unix

package client

import (
	"context"
	"net/http"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon/api"
)

// Windows option A (forced here with OH_DAEMON_INPROCESS=1): Ensure starts
// the daemon inside the process instead of spawning it, once.
func TestEnsureInProcess(t *testing.T) {
	t.Setenv("OH_DAEMON_INPROCESS", "1")
	require.True(t, InProcessMode())
	p := shortPaths(t)
	var starts atomic.Int32
	var served *fake
	opts := EnsureOptions{Version: "t", InProcess: func() error {
		starts.Add(1)
		_, served, _ = serveAt(t, p, reply(http.StatusOK, api.Health{Version: "t", PID: os.Getpid()}))
		return nil
	}}
	_, h, err := Ensure(context.Background(), p, opts)
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), h.PID)
	_, _, err = Ensure(context.Background(), p, opts)
	require.NoError(t, err)
	assert.Equal(t, int32(1), starts.Load(), "started once")
	assert.Equal(t, "/v1/health", served.last(t).URI)

	_, _, err = Ensure(context.Background(), api.Paths{Dir: t.TempDir()}, EnsureOptions{})
	assert.ErrorIs(t, err, api.ErrUnsupported, "no in-process starter")
}

func TestEnsureKeepsARunningDaemon(t *testing.T) {
	t.Setenv("OH_DAEMON_INPROCESS", "")
	c, f, p := serve(t, reply(http.StatusOK, api.Health{Version: "5.1.0"}))
	ec, h, err := Ensure(context.Background(), p, EnsureOptions{Version: "5.1.0", Capability: "cap"})
	require.NoError(t, err)
	assert.Equal(t, "5.1.0", h.Version)
	require.NoError(t, ec.Touch(context.Background(), "g"))
	assert.Equal(t, "cap", f.last(t).Capability, "the returned client sends the capability")
	_ = c

	// another version with live servers: kept, not replaced
	c2, f2, p2 := serve(t, reply(http.StatusOK, api.Health{Version: "5.0.0", Servers: 2}))
	_ = c2
	_, h, err = Ensure(context.Background(), p2, EnsureOptions{Version: "5.1.0"})
	require.NoError(t, err)
	assert.Equal(t, "5.0.0", h.Version)
	assert.NotContains(t, f2.uris(), "/v1/shutdown")
}

// A daemon of another version without server is asked to stop before a new
// one is spawned.
func TestEnsureReplacesAnIdleDaemonOfAnotherVersion(t *testing.T) {
	t.Setenv("OH_DAEMON_INPROCESS", "")
	var stopped atomic.Bool
	_, f, p := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if stopped.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if r.URL.Path == "/v1/shutdown" {
			stopped.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		reply(http.StatusOK, api.Health{Version: "5.0.0"})(w, r)
	})
	_, _, err := Ensure(context.Background(), p, EnsureOptions{Version: "5.1.0", Executable: "/nonexistent/oh"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "starting ohd")
	assert.Contains(t, f.uris(), "/v1/shutdown")
}
