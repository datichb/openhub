package daemon

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func hardeningEnv(t *testing.T) (Paths, *sqlite.ServerStore, *sqlite.GrantStore) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return p, sqlite.NewServerStore(st), sqlite.NewGrantStore(st)
}

func runDaemon(t *testing.T, opts Options) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, opts) }()
	t.Cleanup(func() { cancel(); <-done })
	c := NewClient(opts.Paths)
	require.Eventually(t, func() bool { h, err := c.Health(context.Background()); return err == nil && !h.Restoring }, 5*time.Second, 20*time.Millisecond)
}

// E14-B1: restarting an old group must not lose its proxy token while starting.
func TestRestartingOldGroupKeepsItsGrant(t *testing.T) {
	p, servers, grants := hardeningEnv(t)
	ctx := context.Background()
	old := time.Now().Add(-time.Hour)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g", Adapter: "x", PID: 999999, Status: domain.ServerSleeping, CreatedAt: old}))
	// A client restarts the group: status starting, no PID yet, new start time.
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g", Adapter: "x", Status: domain.ServerStarting, CreatedAt: time.Now()}))
	tok := credproxy.NewToken()
	require.NoError(t, grants.Insert(ctx, &domain.ProxyGrant{TokenHash: tok, Owner: "g", Provider: credproxy.ProviderBedrock, Source: domain.CredentialSource{Kind: domain.CredentialSigV4}}))

	runDaemon(t, Options{Paths: p, Servers: servers, Grants: grants, Tick: 30 * time.Millisecond, IdleAfter: time.Hour,
		SigV4: func(context.Context, string, string) (credproxy.Auth, error) {
			return credproxy.BearerAuth{Token: "s"}, nil
		}})
	time.Sleep(300 * time.Millisecond)
	s, err := servers.Get(ctx, "g")
	require.NoError(t, err)
	assert.Equal(t, domain.ServerStarting, s.Status)
	active, _ := grants.ListActive(ctx)
	assert.Len(t, active, 1, "the token of a starting server is kept")
}

// E14-M1: a live PID that does not answer as our server is marked stopped and never signalled.
func TestReusedPIDIsNotSignalled(t *testing.T) {
	p, servers, grants := hardeningEnv(t)
	ctx := context.Background()
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g", Adapter: "fake", PID: os.Getpid(), Status: domain.ServerReady, CreatedAt: time.Now().Add(-time.Hour)}))
	ad := &stopCountingAdapter{fakeAdapter: newFake()}
	ad.failAll = true
	runDaemon(t, Options{Paths: p, Servers: servers, Grants: grants, Tick: 30 * time.Millisecond, IdleAfter: time.Hour,
		Adapter: func(string) adapters.ToolAdapter { return ad }})
	require.Eventually(t, func() bool {
		s, _ := servers.Get(ctx, "g")
		return s != nil && s.Status == domain.ServerStopped
	}, 3*time.Second, 30*time.Millisecond)
	assert.Equal(t, int32(0), ad.stops.Load(), "no signal sent to a process that is not our server")
}

// Restoration: orphan grants (no server, a stopped or sleeping group, or a
// token the server does not hold) are revoked; the others are restored, with
// one AWS signer per profile/region.
func TestRestoreRevokesOrphansAndCachesSigners(t *testing.T) {
	p, servers, grants := hardeningEnv(t)
	ctx := context.Background()
	old := time.Now().Add(-time.Hour)
	held, stale, gone, asleep, fresh := credproxy.NewToken(), credproxy.NewToken(), credproxy.NewToken(), credproxy.NewToken(), credproxy.NewToken()
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "live", Adapter: "x", PID: os.Getpid(), ProxyTokenHash: held, Status: domain.ServerReady, CreatedAt: old}))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "zz", Adapter: "x", Status: domain.ServerSleeping, CreatedAt: old}))
	sig := domain.CredentialSource{Kind: domain.CredentialSigV4, Profile: "work"}
	for _, g := range []domain.ProxyGrant{
		{TokenHash: held, Owner: "live", CreatedAt: old},
		{TokenHash: stale, Owner: "live", CreatedAt: old},  // replaced by a newer token
		{TokenHash: gone, Owner: "nobody", CreatedAt: old}, // no server row
		{TokenHash: asleep, Owner: "zz", CreatedAt: old},   // sleeping group
		{TokenHash: fresh, Owner: "starting"},              // a client is starting its server
	} {
		g.Provider, g.Region, g.Source = credproxy.ProviderBedrock, "eu-west-1", sig
		require.NoError(t, grants.Insert(ctx, &g))
	}
	var loads atomic.Int32
	runDaemon(t, Options{Paths: p, Servers: servers, Grants: grants, Tick: time.Hour, IdleAfter: time.Hour,
		SigV4: func(context.Context, string, string) (credproxy.Auth, error) {
			loads.Add(1)
			return credproxy.BearerAuth{Token: "s"}, nil
		}})
	active, err := grants.ListActive(ctx)
	require.NoError(t, err)
	var toks []string
	for _, g := range active {
		toks = append(toks, g.TokenHash)
	}
	assert.ElementsMatch(t, []string{credproxy.TokenHash(held), credproxy.TokenHash(fresh)}, toks, "kept, and stored as hashes only")
	assert.Equal(t, int32(1), loads.Load(), "one signer for the profile/region")
	c := NewClient(p)
	_, err = c.Usage(ctx, held)
	assert.NoError(t, err)
	_, err = c.Usage(ctx, stale)
	assert.Error(t, err)
}

// The socket answers before slow grants are restored (AWS SSO, IMDS…), and
// grant routes wait for the restoration.
func TestSocketAnswersBeforeSlowRestore(t *testing.T) {
	p, servers, grants := hardeningEnv(t)
	ctx := context.Background()
	tok := credproxy.NewToken()
	require.NoError(t, grants.Insert(ctx, &domain.ProxyGrant{TokenHash: tok, Owner: "g", Provider: credproxy.ProviderBedrock,
		Source: domain.CredentialSource{Kind: domain.CredentialSigV4}}))
	release := make(chan struct{})
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Servers: servers, Grants: grants, Tick: time.Hour, IdleAfter: time.Hour,
			SigV4: func(context.Context, string, string) (credproxy.Auth, error) {
				<-release
				return credproxy.BearerAuth{Token: "s"}, nil
			}})
	}()
	t.Cleanup(func() { cancel(); <-done })
	c := NewClient(p)
	require.Eventually(t, func() bool { h, err := c.Health(ctx); return err == nil && h.Restoring }, 5*time.Second, 20*time.Millisecond)
	got := make(chan error, 1)
	go func() { _, err := c.Usage(ctx, tok); got <- err }()
	select {
	case <-got:
		t.Fatal("usage answered before the restoration")
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-got:
		assert.NoError(t, err, "restored grant")
	case <-time.After(5 * time.Second):
		t.Fatal("usage never answered")
	}
}
