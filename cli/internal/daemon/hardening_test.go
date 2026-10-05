package daemon

import (
	"context"
	"os"
	"path/filepath"
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
	require.Eventually(t, func() bool { _, err := c.Health(context.Background()); return err == nil }, 5*time.Second, 20*time.Millisecond)
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
	require.NoError(t, grants.Insert(ctx, &domain.ProxyGrant{Token: tok, Owner: "g", Provider: credproxy.ProviderBedrock, Source: domain.CredentialSource{Kind: domain.CredentialSigV4}}))

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
