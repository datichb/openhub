package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func TestDaemonBeadsGateway(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	project, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1',?)`, project)
	require.NoError(t, err)
	sessions, servers := sqlite.NewSessionStore(st), sqlite.NewServerStore(st)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "x", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))
	// A grant of a group that no longer runs is dropped at startup.
	stale, err := gateway.OpenStore(p.Gateway())
	require.NoError(t, err)
	staleTok, err := stale.Issue(gateway.Grant{SessionID: "ses_old", GroupKey: "gone", Location: project})
	require.NoError(t, err)

	bd := filepath.Join(t.TempDir(), "bd")
	require.NoError(t, os.WriteFile(bd, []byte("#!/bin/sh\necho \"$* in $(pwd -P)\"\n"), 0o755))

	start := func() (context.CancelFunc, chan error) {
		dctx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() {
			done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Tick: time.Hour, IdleAfter: time.Hour,
				BeadsBinary: bd,
				GatewayView: func(_ context.Context, group string) (gateway.View, error) {
					assert.Equal(t, "g1", group)
					return gateway.View{Paths: ohruntime.PathMap{{Host: project, Inner: "/work/p1"}}, Locations: []string{project}}, nil
				}})
		}()
		c := NewClient(p)
		require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 5*time.Second, 20*time.Millisecond)
		return cancel, done
	}
	cancel, done := start()
	c := NewClient(p)
	h, err := c.Health(ctx)
	require.NoError(t, err)

	exec := func(tok string, argv ...string) (int, beadswire.ExecResponse) {
		body, _ := json.Marshal(beadswire.ExecRequest{Argv: argv, Cwd: "/work/p1"})
		req, _ := http.NewRequest(http.MethodPost, h.ProxyURL+"/"+beadswire.Prefix+beadswire.ExecPath, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		var out beadswire.ExecResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
		return resp.StatusCode, out
	}

	code, _ := exec(staleTok, "show", "x")
	assert.Equal(t, http.StatusUnauthorized, code, "stale grant dropped")

	g, err := c.IssueGatewayGrant(ctx, GatewayGrantRequest{SessionID: "ses_a", GroupKey: "g1", ProjectID: "p1", Location: project, BeadsAllow: []string{"show"}})
	require.NoError(t, err)
	code, out := exec(g.Token, "show", "bd-1")
	require.Equal(t, http.StatusOK, code, out.Error)
	assert.Equal(t, "show bd-1 in "+project+"\n", string(out.Stdout))
	code, out = exec(g.Token, "create", "x")
	assert.Equal(t, http.StatusForbidden, code)
	assert.NotEmpty(t, out.Error)

	// The grant survives a daemon restart (only its hash is on disk).
	data, err := os.ReadFile(p.Gateway())
	require.NoError(t, err)
	assert.NotContains(t, string(data), g.Token)
	cancel()
	<-done
	cancel, done = start()
	t.Cleanup(func() { cancel(); <-done })
	h, err = c.Health(ctx)
	require.NoError(t, err)
	code, _ = exec(g.Token, "show", "bd-1")
	assert.Equal(t, http.StatusOK, code)

	// A stopped session loses Beads; revoking the group drops the grant.
	s, err := sessions.Get(ctx, "ses_a")
	require.NoError(t, err)
	s.State = domain.RunStopped
	require.NoError(t, sessions.Update(ctx, s))
	code, _ = exec(g.Token, "show", "bd-1")
	assert.Equal(t, http.StatusUnauthorized, code)
	require.NoError(t, c.RevokeOwner(ctx, "g1"))
	data, _ = os.ReadFile(p.Gateway())
	assert.NotContains(t, string(data), "ses_a")
}

// envFake records the session environments applied by the daemon.
type envFake struct {
	*fakeAdapter
	mu  sync.Mutex
	env map[string]map[string]string
}

func (e *envFake) SetSessionEnv(_ context.Context, _ adapters.ServerHandle, id string, env map[string]string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.env[id] = env
	return nil
}

func (e *envFake) envOf(id string) map[string]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.env[id]
}

// Sub-sessions do not inherit the session environment in opencode: the
// daemon applies the root one, with a gateway token of their own.
func TestDaemonSubagentSessionEnvironment(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions, servers := sqlite.NewSessionStore(st), sqlite.NewServerStore(st)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))
	sdir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(sdir, "ses_r"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(sdir, "ses_r", "env.json"), []byte(`{"FOO":"bar"}`), 0o600))

	fake := &envFake{fakeAdapter: newFake(), env: map[string]map[string]string{}}
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, SessionsDir: sdir, Tick: time.Hour, IdleAfter: time.Hour,
			Adapter: func(string) adapters.ToolAdapter { return fake }})
	}()
	t.Cleanup(func() { cancel(); <-done })
	var ch chan adapters.ToolEvent
	select {
	case ch = <-fake.events:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not subscribe")
	}
	c := NewClient(p)
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 3*time.Second, 20*time.Millisecond)
	root, err := c.IssueGatewayGrant(ctx, GatewayGrantRequest{SessionID: "ses_r", GroupKey: "g1", Location: "/p1",
		BeadsAllow: []string{"show"}, GatewayURL: "http://host.docker.internal:1/oh-gateway"})
	require.NoError(t, err)
	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}

	ch <- adapters.ToolEvent{Kind: adapters.EventSessionCreated, SessionID: "ses_c", ParentID: "ses_r"}
	ch <- adapters.ToolEvent{Kind: adapters.EventSessionCreated, SessionID: "ses_cc", ParentID: "ses_c"}
	require.Eventually(t, func() bool { return fake.envOf("ses_c") != nil && fake.envOf("ses_cc") != nil }, 3*time.Second, 20*time.Millisecond)
	child := fake.envOf("ses_c")
	assert.Equal(t, "bar", child["FOO"])
	assert.Equal(t, "ses_r", child["OH_SESSION_ID"])
	assert.Equal(t, "http://host.docker.internal:1/oh-gateway", child[beadswire.EnvURL])
	tok := child[beadswire.EnvToken]
	require.NotEmpty(t, tok)
	assert.NotEqual(t, root.Token, tok)
	assert.NotEqual(t, tok, fake.envOf("ses_cc")[beadswire.EnvToken], "one token per sub-session")
	assert.Equal(t, "ses_r", fake.envOf("ses_cc")["OH_SESSION_ID"], "grand-children belong to the root session")

	// Both tokens keep working; revoking the group drops them all.
	st2, err := gateway.OpenStore(p.Gateway())
	require.NoError(t, err)
	g, ok := st2.Lookup(tok)
	require.True(t, ok)
	assert.Equal(t, "ses_c", g.Holder)
	assert.Equal(t, []string{"show"}, g.BeadsAllow)
	_, ok = st2.Lookup(root.Token)
	assert.True(t, ok, "the root token is kept")
	require.NoError(t, c.RevokeOwner(ctx, "g1"))
	st3, _ := gateway.OpenStore(p.Gateway())
	assert.Equal(t, 0, st3.Len())
}

// The oh MCP servers of a group are served over HTTP on the proxy
// listeners, authenticated by the group proxy token (P4-T08).
func TestDaemonMCPGateway(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	work, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1',?)`, work)
	require.NoError(t, err)
	sessions, servers := sqlite.NewSessionStore(st), sqlite.NewServerStore(st)
	for _, g := range []string{"g1", "g2"} {
		require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: g, Adapter: "x", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady, WorkDir: work}))
	}
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_g2", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g2", State: domain.RunActive}))
	exe, err := os.Executable()
	require.NoError(t, err)

	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Grants: sqlite.NewGrantStore(st), Tick: time.Hour, IdleAfter: time.Hour,
			MCPCommand: func(_ context.Context, srv domain.Server, name string) (gateway.MCPCommand, error) {
				require.Equal(t, "team", name)
				return gateway.MCPCommand{Argv: []string{exe}, Dir: srv.WorkDir, Env: map[string]string{"OH_DAEMON_TEST_MCP": "1", "OH_TEST_SERVICE_TOKEN": "from-keychain-" + srv.GroupKey}}, nil
			}})
	}()
	t.Cleanup(func() { cancel(); <-done })
	c := NewClient(p)
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 5*time.Second, 20*time.Millisecond)
	grant, err := c.IssueGrant(ctx, GrantRequest{Owner: "g1", Provider: "amazon-bedrock", Region: "eu-west-1",
		Source: domain.CredentialSource{Kind: domain.CredentialBearer}, Secret: "real"})
	require.NoError(t, err)
	h, err := c.Health(ctx)
	require.NoError(t, err)

	post := func(token, body string) (int, string) {
		req, _ := http.NewRequest(http.MethodPost, h.ProxyURL+"/oh/v1/hooks/mcp/team", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	code, _ := post("ohs_wrong", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	assert.Equal(t, http.StatusUnauthorized, code)

	code, body := post(grant.Token, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"serverInfo"`)
	code, body = post(grant.Token, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"whoami","_meta":{"ai.opencode/sessionID":"ses_unknown"}}}`)
	require.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, "from-keychain-g1@"+work, "secret read by the server on the machine, in the group directory")

	// A session of another group is refused.
	code, _ = post(grant.Token, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"whoami","_meta":{"ai.opencode/sessionID":"ses_g2"}}}`)
	assert.Equal(t, http.StatusForbidden, code)

	// Revoking the group stops its MCP servers and the token.
	require.NoError(t, c.RevokeOwner(ctx, "g1"))
	code, _ = post(grant.Token, `{"jsonrpc":"2.0","id":4,"method":"tools/list"}`)
	assert.Equal(t, http.StatusUnauthorized, code)
}
