package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// A restarted daemon that cannot bind the previous proxy port puts to sleep
// the groups whose server still uses it (recorded URL, or started before it
// without one); working sessions get an error decision. Up-to-date groups
// keep running.
func TestProxyPortChangeSleepsStaleGroups(t *testing.T) {
	defer func(d time.Duration) { proxyPortRetry = d }(proxyPortRetry)
	proxyPortRetry = 200 * time.Millisecond

	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	servers, sessions, decisions := sqlite.NewServerStore(st), sqlite.NewSessionStore(st), sqlite.NewDecisionStore(st)
	serversDir := filepath.Join(p.Dir, "servers")

	// The previous port is taken by another process.
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { busy.Close() })
	oldPort := busy.Addr().(*net.TCPAddr).Port
	data, _ := json.Marshal(stateFile{ProxyPort: oldPort})
	require.NoError(t, os.WriteFile(p.State(), data, 0o600))

	old := time.Now().Add(-time.Hour)
	for _, g := range []string{"recorded", "legacy"} {
		require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: g, Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady, CreatedAt: old}))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(serversDir, "recorded"), 0o700))
	require.NoError(t, os.WriteFile(ProxyURLPath(serversDir, "recorded"), []byte(fmt.Sprintf("http://host.docker.internal:%d/amazon-bedrock", oldPort)), 0o600))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_work", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "recorded", State: domain.RunActive}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_idle", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "legacy", State: domain.RunIdle}))

	ad := &stopCountingAdapter{fakeAdapter: newFake()}
	runDaemon(t, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Decisions: decisions,
		Grants: sqlite.NewGrantStore(st), ServersDir: serversDir, Tick: 50 * time.Millisecond, IdleAfter: time.Hour, IdleSleep: time.Hour,
		Adapter: func(string) adapters.ToolAdapter { return ad }})

	h, err := NewClient(p).Health(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, oldPort, urlPort(h.ProxyURL))

	// A group started after the restart records the new URL: kept.
	require.NoError(t, os.MkdirAll(filepath.Join(serversDir, "fresh"), 0o700))
	require.NoError(t, os.WriteFile(ProxyURLPath(serversDir, "fresh"), []byte(h.ProxyURL+"/amazon-bedrock"), 0o600))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "fresh", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady, CreatedAt: time.Now()}))

	status := func(g string) domain.ServerStatus {
		s, err := servers.Get(ctx, g)
		require.NoError(t, err)
		return s.Status
	}
	require.Eventually(t, func() bool {
		return status("recorded") == domain.ServerSleeping && status("legacy") == domain.ServerSleeping
	}, 5*time.Second, 50*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, domain.ServerReady, status("fresh"))

	open, err := decisions.ListOpen(ctx, domain.DecisionFilter{})
	require.NoError(t, err)
	require.Len(t, open, 1, "only the working session is told")
	assert.Equal(t, "ses_work", open[0].SessionID)
	assert.Equal(t, domain.DecisionError, open[0].Kind)
	s, err := sessions.Get(ctx, "ses_idle")
	require.NoError(t, err)
	assert.Equal(t, domain.RunSleeping, s.State, "resumable")
}

// When the previous port is free again, nothing is put to sleep.
func TestProxyPortKeptKeepsGroups(t *testing.T) {
	p, servers, grants := hardeningEnv(t)
	ctx := context.Background()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	data, _ := json.Marshal(stateFile{ProxyPort: port})
	require.NoError(t, os.WriteFile(p.State(), data, 0o600))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g", Adapter: "fake", PID: os.Getpid(), Status: domain.ServerReady, CreatedAt: time.Now().Add(-time.Hour)}))
	ad := &stopCountingAdapter{fakeAdapter: newFake()}
	runDaemon(t, Options{Paths: p, Servers: servers, Grants: grants, ServersDir: filepath.Join(p.Dir, "servers"), Tick: 50 * time.Millisecond,
		IdleAfter: time.Hour, IdleSleep: time.Hour, Adapter: func(string) adapters.ToolAdapter { return ad }})
	time.Sleep(300 * time.Millisecond)
	s, err := servers.Get(ctx, "g")
	require.NoError(t, err)
	assert.Equal(t, domain.ServerReady, s.Status)
	assert.Equal(t, int32(0), ad.stops.Load())
}
