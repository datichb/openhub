package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// A16: the daemon checks once the shell of a local session; when another
// bd comes first, an alert (✗) is raised that the agent loop does not clear.
func TestShellAlert(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions, servers, decisions := sqlite.NewSessionStore(st), sqlite.NewServerStore(st), sqlite.NewDecisionStore(st)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))

	root := t.TempDir()
	shims, other := filepath.Join(root, "shims"), filepath.Join(root, "other")
	for _, d := range []string{shims, other} {
		require.NoError(t, os.MkdirAll(d, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(d, "bd"), []byte("#!/bin/sh\n"), 0o755))
	}
	sessionsDir := t.TempDir()
	write := func(id, path string) {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive, Runtime: "local"}))
		require.NoError(t, os.MkdirAll(filepath.Join(sessionsDir, id), 0o700))
		data, _ := json.Marshal(map[string]string{"SHELL": "/bin/sh", "PATH": path, gateway.EnvPathFirst: shims})
		require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, id, "env.json"), data, 0o600))
	}
	write("ses_bad", other+":"+shims+":/usr/bin:/bin")
	write("ses_ok", shims+":"+other+":/usr/bin:/bin")

	fake := newFake()
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Decisions: decisions, SessionsDir: sessionsDir,
			Tick: time.Hour, IdleAfter: time.Hour,
			Adapter: func(name string) adapters.ToolAdapter {
				if name == "fake" {
					return fake
				}
				return nil
			}})
	}()
	t.Cleanup(func() { cancel(); <-done })
	var ch chan adapters.ToolEvent
	select {
	case ch = <-fake.events:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not subscribe")
	}
	open := func(id string) []domain.Decision {
		l, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
		require.NoError(t, err)
		return l
	}
	for _, id := range []string{"ses_bad", "ses_ok"} {
		ch <- adapters.ToolEvent{Kind: adapters.EventActivity, SessionID: id}
	}
	require.Eventually(t, func() bool { return len(open("ses_bad")) == 1 }, 5*time.Second, 20*time.Millisecond)
	d := open("ses_bad")[0]
	assert.Equal(t, domain.DecisionError, d.Kind)
	assert.Contains(t, d.Payload.Message, filepath.Join(other, "bd"))
	assert.True(t, isOhAlert(d))

	ch <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_bad"}
	ch <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_bad", Outcome: "succeeded"}
	time.Sleep(300 * time.Millisecond)
	assert.Len(t, open("ses_bad"), 1, "not cleared by the agent loop")
	assert.Empty(t, open("ses_ok"))
}
