package daemon

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
)

type memStore struct {
	mu     sync.Mutex
	m      map[string]string
	setErr error
	getErr error
}

func (s *memStore) Get(_ context.Context, k string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[k], s.getErr
}

func (s *memStore) Set(_ context.Context, k, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.setErr != nil {
		return s.setErr
	}
	s.m[k] = v
	return nil
}

func TestLoadCapability(t *testing.T) {
	ctx := context.Background()
	p := Paths{Dir: t.TempDir()}

	// Secret store usable: created there once, then reused.
	ks := &memStore{m: map[string]string{}}
	c1, src, err := LoadCapability(ctx, ks, p)
	require.NoError(t, err)
	assert.Equal(t, CapabilityKeychain, src)
	c2, _, err := LoadCapability(ctx, ks, p)
	require.NoError(t, err)
	assert.Equal(t, c1, c2)
	_, err = os.Stat(p.CapabilityFile())
	assert.True(t, os.IsNotExist(err))

	// No usable store: 0600 file, stable across calls.
	q := Paths{Dir: t.TempDir()}
	f1, src, err := LoadCapability(ctx, &memStore{m: map[string]string{}, getErr: errors.New("locked")}, q)
	require.NoError(t, err)
	assert.Equal(t, CapabilityFile, src)
	st, err := os.Stat(q.CapabilityFile())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	f2, _, err := LoadCapability(ctx, nil, q)
	require.NoError(t, err)
	assert.Equal(t, f1, f2)

	// The store becomes usable: the file is moved into it.
	ks2 := &memStore{m: map[string]string{}}
	f3, src, err := LoadCapability(ctx, ks2, q)
	require.NoError(t, err)
	assert.Equal(t, CapabilityKeychain, src)
	assert.Equal(t, f1, f3)
	_, err = os.Stat(q.CapabilityFile())
	assert.True(t, os.IsNotExist(err))
}

// M12: issuing tokens, revoking, opening listeners and stopping the daemon
// need the capability; the tool-facing routes do not.
func TestPrivilegedRoutesNeedTheCapability(t *testing.T) {
	p, servers, grants := hardeningEnv(t)
	ctx := context.Background()
	runDaemon(t, Options{Paths: p, Servers: servers, Grants: grants, Tick: time.Hour, IdleAfter: time.Hour, Capability: "ohc_secret"})
	bare := NewClient(p)
	req := GrantRequest{Owner: "g", Provider: credproxy.ProviderBedrock, Region: "eu-west-1",
		Source: domain.CredentialSource{Kind: domain.CredentialBearer}, Secret: "real"}

	var apiErr *APIError
	_, err := bare.IssueGrant(ctx, req)
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
	_, err = bare.WithCapability("ohc_wrong").IssueGrant(ctx, req)
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.Status)
	for _, call := range []func() error{
		func() error { return bare.RevokeOwner(ctx, "g") },
		func() error { _, err := bare.ProxyListen(ctx, "127.0.0.1"); return err },
		func() error { _, err := bare.IssueGatewayGrant(ctx, GatewayGrantRequest{SessionID: "s"}); return err },
		func() error { _, err := bare.PendingGrants(ctx); return err },
		func() error { return bare.Shutdown(ctx, true) },
	} {
		err := call()
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, http.StatusForbidden, apiErr.Status)
	}

	cli := bare.WithCapability("ohc_secret")
	g, err := cli.IssueGrant(ctx, req)
	require.NoError(t, err)
	_, err = bare.Usage(ctx, g.Token)
	assert.NoError(t, err, "read-only routes stay open")
	_, err = bare.Health(ctx)
	assert.NoError(t, err)
}

func TestPeerUIDOfOwnProcess(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ohp-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	l, err := net.Listen("unix", filepath.Join(dir, "s"))
	require.NoError(t, err)
	defer l.Close()
	pl := peerListener{Listener: l}
	got := make(chan net.Conn, 1)
	go func() {
		c, err := pl.Accept()
		if err == nil {
			got <- c
		}
	}()
	c, err := net.Dial("unix", filepath.Join(dir, "s"))
	require.NoError(t, err)
	defer c.Close()
	select {
	case sc := <-got:
		defer sc.Close()
		uid, err := peerUID(sc)
		if errors.Is(err, errUnsupportedPeer) {
			t.Skip("peer credentials not supported here")
		}
		require.NoError(t, err)
		assert.Equal(t, os.Getuid(), uid)
	case <-time.After(3 * time.Second):
		t.Fatal("connection of the same user refused")
	}
}
