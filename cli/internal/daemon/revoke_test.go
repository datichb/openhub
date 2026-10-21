package daemon

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// captureLogs sends the default logger to a buffer for the test.
func captureLogs(t *testing.T) *syncBuffer {
	t.Helper()
	logs := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return logs
}

// flakyGrants fails the first fails calls of Revoke and RevokeOwner, then
// delegates to the real store.
type flakyGrants struct {
	domain.GrantStore
	mu           sync.Mutex
	fails        int
	revokes      int
	ownerRevokes int
}

func (f *flakyGrants) fail() bool {
	if f.fails > 0 {
		f.fails--
		return true
	}
	return false
}

func (f *flakyGrants) Revoke(ctx context.Context, token string, at time.Time) error {
	f.mu.Lock()
	f.revokes++
	failed := f.fail()
	f.mu.Unlock()
	if failed {
		return errors.New("database is locked")
	}
	return f.GrantStore.Revoke(ctx, token, at)
}

func (f *flakyGrants) RevokeOwner(ctx context.Context, owner string, at time.Time) error {
	f.mu.Lock()
	f.ownerRevokes++
	failed := f.fail()
	f.mu.Unlock()
	if failed {
		return errors.New("database is locked")
	}
	return f.GrantStore.RevokeOwner(ctx, owner, at)
}

func revokeEnv(t *testing.T) (*sqlite.Store, *sqlite.GrantStore, *sqlite.ServerStore) {
	t.Helper()
	st := sqlitetest.New(t)
	return st, sqlite.NewGrantStore(st), sqlite.NewServerStore(st)
}

func insertGrant(t *testing.T, g domain.GrantStore, owner string, age time.Duration) string {
	t.Helper()
	tok := credproxy.NewToken()
	require.NoError(t, g.Insert(context.Background(), &domain.ProxyGrant{
		TokenHash: credproxy.TokenHash(tok), Owner: owner, Provider: credproxy.ProviderBedrock,
		Source: domain.CredentialSource{Kind: domain.CredentialSigV4}, CreatedAt: time.Now().Add(-age),
	}))
	return credproxy.TokenHash(tok)
}

func activeHashes(t *testing.T, g domain.GrantStore) []string {
	t.Helper()
	list, err := g.ListActive(context.Background())
	require.NoError(t, err)
	var out []string
	for _, x := range list {
		out = append(out, x.TokenHash)
	}
	return out
}

// S-29: a database error on revocation is logged and the revocation tried
// once more; the token is gone from the database afterwards.
func TestRevokeOwnerLogsAndRetriesDatabaseError(t *testing.T) {
	logs := captureLogs(t)
	_, grants, _ := revokeEnv(t)
	hash := insertGrant(t, grants, "g1", 0)
	flaky := &flakyGrants{GrantStore: grants, fails: 1}
	d := &Daemon{opts: Options{Grants: flaky}, proxy: credproxy.New()}

	assert.True(t, d.revokeOwner(context.Background(), "g1"))
	assert.Equal(t, 2, flaky.ownerRevokes, "tried again once")
	assert.NotContains(t, activeHashes(t, grants), hash)
	assert.Contains(t, logs.String(), "revocation failed, trying again")
	assert.Contains(t, logs.String(), "database is locked")
}

// A revocation that fails twice is reported: the log says so and the route
// answers an error, so that the caller can try again.
func TestRevokeOwnerReportsPersistentFailure(t *testing.T) {
	logs := captureLogs(t)
	_, grants, _ := revokeEnv(t)
	hash := insertGrant(t, grants, "g1", 0)
	flaky := &flakyGrants{GrantStore: grants, fails: 10}
	d := &Daemon{opts: Options{Grants: flaky}, proxy: credproxy.New()}

	req := httptest.NewRequest(http.MethodDelete, "/owners/g1/grants", nil)
	req.SetPathValue("owner", "g1")
	rec := httptest.NewRecorder()
	d.handleRevokeOwner(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, 2, flaky.ownerRevokes, "one retry, no more")
	assert.Contains(t, activeHashes(t, grants), hash, "still stored: the purge at startup revokes it")
	assert.Contains(t, logs.String(), "stays valid in the database")
}

// Restoration: the revocation of an orphan grant is retried too.
func TestRestoreRetriesOrphanRevocation(t *testing.T) {
	logs := captureLogs(t)
	_, grants, servers := revokeEnv(t)
	ctx := context.Background()
	old := time.Now().Add(-time.Hour)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "asleep", Adapter: "x", Status: domain.ServerSleeping, CreatedAt: old}))
	// The purge is not available on this store: the per-grant path revokes.
	flaky := &flakyGrants{GrantStore: grants, fails: 1}
	hash := insertGrant(t, grants, "asleep", time.Hour)
	d := &Daemon{opts: Options{Grants: flaky, Servers: servers}, proxy: credproxy.New(), pending: map[string]domain.ProxyGrant{}}

	d.restoreGrants(ctx)

	assert.Equal(t, 2, flaky.revokes)
	assert.NotContains(t, activeHashes(t, grants), hash)
	assert.Contains(t, logs.String(), "revocation failed, trying again")
}

// S-29: at startup the grants without a live owner are revoked in the
// database, including the rows that can no longer be listed.
func TestStartupPurgesOrphanGrants(t *testing.T) {
	st, grants, servers := revokeEnv(t)
	ctx := context.Background()
	old := time.Now().Add(-time.Hour)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "live", Adapter: "x", PID: os.Getpid(), Status: domain.ServerReady, CreatedAt: old}))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "stopped", Adapter: "x", Status: domain.ServerStopped, CreatedAt: old}))

	kept := insertGrant(t, grants, "live", time.Hour)
	fresh := insertGrant(t, grants, "starting-now", 0) // a client is starting its server
	gone := insertGrant(t, grants, "deleted", time.Hour)
	ended := insertGrant(t, grants, "stopped", time.Hour)
	// A row whose credential source is unreadable is not listed (S-23): only
	// the purge reaches it.
	unreadable := credproxy.TokenHash(credproxy.NewToken())
	_, err := st.DB().Exec(`INSERT INTO proxy_grants (token, owner, provider, source, created_at) VALUES (?, 'deleted', 'amazon-bedrock', 'not json', ?)`,
		unreadable, old)
	require.NoError(t, err)

	p := shortPaths(t)
	runDaemon(t, Options{Paths: p, Servers: servers, Grants: grants, Tick: time.Hour, IdleAfter: time.Hour,
		SigV4: func(context.Context, string, string) (credproxy.Auth, error) {
			return credproxy.BearerAuth{Token: "s"}, nil
		}})

	assert.ElementsMatch(t, []string{kept, fresh}, activeHashes(t, grants))
	var revoked int
	require.NoError(t, st.DB().QueryRow(`SELECT COUNT(*) FROM proxy_grants WHERE revoked_at IS NOT NULL AND token IN (?, ?, ?)`, gone, ended, unreadable).Scan(&revoked))
	assert.Equal(t, 3, revoked, "the grants of a missing or ended group, listable or not, are revoked in the database")
}

// The purge failing is logged and tried again, and the grants of the others
// are still restored.
func TestPurgeOrphanGrantsErrorIsLogged(t *testing.T) {
	logs := captureLogs(t)
	_, grants, servers := revokeEnv(t)
	d := &Daemon{opts: Options{Grants: failingPurge{grants}, Servers: servers}}
	d.purgeOrphanGrants(context.Background())
	assert.Contains(t, logs.String(), "orphan purge")
	assert.Contains(t, logs.String(), "disk I/O error")
}

type failingPurge struct{ domain.GrantStore }

func (failingPurge) RevokeOrphans(context.Context, time.Time, time.Time) (int, error) {
	return 0, errors.New("disk I/O error")
}

// S-29: the gateway grants of a session deleted or stopped for good are
// purged at startup; those of a session that can resume are kept.
func TestStartGatewayPurgesEndedSessions(t *testing.T) {
	st, _, servers := revokeEnv(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions := sqlite.NewSessionStore(st)
	for id, state := range map[string]domain.RunState{"ses_run": domain.RunActive, "ses_stopped": domain.RunStopped, "ses_done": domain.RunCompleted} {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: state}))
	}
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "x", PID: os.Getpid(), Status: domain.ServerReady, CreatedAt: time.Now()}))

	p := shortPaths(t)
	seed, err := gateway.OpenStore(p.Gateway())
	require.NoError(t, err)
	tokens := map[string]string{}
	for _, id := range []string{"ses_run", "ses_stopped", "ses_done", "ses_deleted"} {
		tokens[id], err = seed.Issue(gateway.Grant{SessionID: id, GroupKey: "g1", Location: "/p1"})
		require.NoError(t, err)
	}

	d := &Daemon{opts: Options{Paths: p, Servers: servers, Sessions: sessions}, proxy: credproxy.New()}
	d.startGateway(ctx)

	for id, want := range map[string]bool{"ses_run": true, "ses_done": true, "ses_stopped": false, "ses_deleted": false} {
		_, ok := d.gateway.Lookup(tokens[id])
		assert.Equal(t, want, ok, id)
	}
	// The purge is written: the next daemon does not see them either.
	reopened, err := gateway.OpenStore(p.Gateway())
	require.NoError(t, err)
	assert.Equal(t, 2, reopened.Len())
}
