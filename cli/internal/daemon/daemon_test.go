package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

type mapSecrets map[string]string

func (m mapSecrets) Get(_ context.Context, k string) (string, error) {
	if v, ok := m[k]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}

// shortPaths: Unix socket paths are limited to ~104 bytes on macOS.
func shortPaths(t *testing.T) Paths {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "ohd-")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(dir) })
	return Paths{Dir: dir}
}

type env struct {
	paths   Paths
	store   *sqlite.Store
	grants  domain.GrantStore
	servers domain.ServerStore
}

func newEnv(t *testing.T) *env {
	t.Helper()
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return &env{paths: p, store: st, grants: sqlite.NewGrantStore(st), servers: sqlite.NewServerStore(st)}
}

func (e *env) start(t *testing.T, secrets SecretGetter, idle time.Duration) (*Client, chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			Paths: e.paths, Version: "test", Grants: e.grants, Servers: e.servers, Secrets: secrets,
			IdleAfter: idle, Tick: 50 * time.Millisecond,
			SigV4: func(context.Context, string, string) (credproxy.Auth, error) {
				return credproxy.BearerAuth{Token: "signed"}, nil
			},
		})
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})
	c := NewClient(e.paths)
	require.Eventually(t, func() bool { h, err := c.Health(context.Background()); return err == nil && !h.Restoring }, 5*time.Second, 20*time.Millisecond)
	return c, done
}

func TestDaemonGrantsAndRestore(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	secrets := mapSecrets{"openhub.provider.bedrock.token": "real"}
	c, done := e.start(t, secrets, time.Hour)

	h, err := c.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, "test", h.Version)
	assert.NotEmpty(t, h.ProxyURL)

	// A second daemon refuses to start.
	err = Run(ctx, Options{Paths: e.paths})
	assert.ErrorIs(t, err, ErrAlreadyRunning)

	resp, err := c.IssueGrant(ctx, GrantRequest{
		Owner: "g1", Provider: credproxy.ProviderBedrock, Region: "eu-west-1",
		Source: domain.CredentialSource{Kind: domain.CredentialBearer, KeychainKey: "openhub.provider.bedrock.token"},
		Secret: "real",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Token, "ohs_")
	assert.Equal(t, h.ProxyURL+"/amazon-bedrock", resp.BaseURL)
	sig, err := c.IssueGrant(ctx, GrantRequest{Owner: "g2", Provider: credproxy.ProviderBedrock, Region: "eu-west-1",
		Source: domain.CredentialSource{Kind: domain.CredentialSigV4, Profile: "work"}})
	require.NoError(t, err)

	_, err = c.IssueGrant(ctx, GrantRequest{Owner: "g3", Provider: credproxy.ProviderAnthropic, Source: domain.CredentialSource{Kind: domain.CredentialAPIKey}})
	assert.Error(t, err, "api key grant without secret")

	_, err = c.Usage(ctx, resp.Token)
	require.NoError(t, err)
	stored, err := e.grants.ListActive(ctx)
	require.NoError(t, err)
	for _, g := range stored {
		assert.NotContains(t, g.TokenHash, "ohs_", "M12: only token hashes are stored")
	}
	_, err = c.Usage(ctx, credproxy.TokenHash(resp.Token))
	require.NoError(t, err, "usage by token hash (server registry)")

	// Restart: grants restored from the store, proxy port kept.
	require.NoError(t, c.Shutdown(ctx, true))
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not stop")
	}
	c2, _ := e.start(t, secrets, time.Hour)
	h2, err := c2.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, h.ProxyURL, h2.ProxyURL, "stable proxy port")
	assert.Equal(t, 2, h2.Grants)
	assert.Equal(t, 0, h2.PendingGrants)
	_, err = c2.Usage(ctx, resp.Token)
	assert.NoError(t, err, "bearer grant restored from the secret store")
	_, err = c2.Usage(ctx, sig.Token)
	assert.NoError(t, err, "sigv4 grant restored from the profile")

	require.NoError(t, c2.RevokeOwner(ctx, "g1"))
	_, err = c2.Usage(ctx, resp.Token)
	assert.Error(t, err)
}

func TestDaemonPendingSecret(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	require.NoError(t, e.grants.Insert(ctx, &domain.ProxyGrant{
		TokenHash: credproxy.NewToken(), Owner: "g1", Provider: credproxy.ProviderAnthropic,
		Source: domain.CredentialSource{Kind: domain.CredentialAPIKey, KeychainKey: "k"},
	}))
	c, _ := e.start(t, nil, time.Hour) // no secret store (e.g. passphrase file store)
	h, err := c.Health(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, h.PendingGrants)

	pending, err := c.PendingGrants(ctx)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.NoError(t, c.ProvideSecret(ctx, pending[0].Token, "sk-real"))
	h, _ = c.Health(ctx)
	assert.Equal(t, 0, h.PendingGrants)
	_, err = c.Usage(ctx, pending[0].Token)
	assert.NoError(t, err)
}

func TestDaemonSupervisionAndIdleExit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	require.NoError(t, e.servers.Upsert(ctx, &domain.Server{GroupKey: "dead", Adapter: "x", PID: 999999, Status: domain.ServerReady, CreatedAt: time.Now().Add(-time.Hour)}))
	require.NoError(t, e.grants.Insert(ctx, &domain.ProxyGrant{TokenHash: credproxy.NewToken(), Owner: "dead", Provider: credproxy.ProviderBedrock,
		Source: domain.CredentialSource{Kind: domain.CredentialSigV4}}))
	// A live server keeps the daemon busy: use our own process.
	require.NoError(t, e.servers.Upsert(ctx, &domain.Server{GroupKey: "live", Adapter: "x", PID: os.Getpid(), Status: domain.ServerReady}))

	c, done := e.start(t, nil, 300*time.Millisecond)
	require.Eventually(t, func() bool {
		s, err := e.servers.Get(ctx, "dead")
		return err == nil && s.Status == domain.ServerStopped
	}, 3*time.Second, 50*time.Millisecond)
	active, _ := e.grants.ListActive(ctx)
	assert.Empty(t, active, "grants of dead servers revoked")

	h, _ := c.Health(ctx)
	assert.Equal(t, 1, h.Servers)
	assert.Error(t, c.Shutdown(ctx, false), "refuses to stop with live sessions")

	select {
	case <-done:
		t.Fatal("daemon stopped while a server is live")
	case <-time.After(700 * time.Millisecond):
	}
	require.NoError(t, e.servers.SetStatus(ctx, "live", domain.ServerStopped))
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not stop when idle")
	}
	_, err := os.Stat(e.paths.Socket())
	assert.True(t, os.IsNotExist(err), "socket removed")
}

// TestHelperDaemon is executed in a child process by TestEnsureSpawnsDaemon.
func TestHelperDaemon(t *testing.T) {
	dir := os.Getenv("OHD_TEST_HELPER_DIR")
	if dir == "" {
		t.Skip("helper process only")
	}
	err := Run(context.Background(), Options{Paths: Paths{Dir: dir}, Version: "helper", IdleAfter: 2 * time.Second, Tick: 100 * time.Millisecond})
	if err != nil && !errors.Is(err, ErrAlreadyRunning) {
		t.Fatal(err)
	}
}

func TestEnsureSpawnsDaemon(t *testing.T) {
	p := shortPaths(t)
	ctx := context.Background()
	opts := EnsureOptions{
		Executable: os.Args[0],
		Args:       []string{"-test.run=^TestHelperDaemon$"},
		Env:        append(os.Environ(), "OHD_TEST_HELPER_DIR="+p.Dir),
		Version:    "helper",
	}
	c, h, err := Ensure(ctx, p, opts)
	require.NoError(t, err)
	assert.Equal(t, "helper", h.Version)
	pid := h.PID

	// Second Ensure reuses the running daemon.
	_, h2, err := Ensure(ctx, p, opts)
	require.NoError(t, err)
	assert.Equal(t, pid, h2.PID)

	require.NoError(t, c.Shutdown(ctx, false))
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err != nil }, 5*time.Second, 50*time.Millisecond)
	_, err = c.Health(ctx)
	assert.ErrorIs(t, err, ErrNotRunning)
}
