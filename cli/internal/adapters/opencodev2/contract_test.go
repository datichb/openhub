//go:build integration

// Contract tests against a real `opencode serve` (no LLM call).
// Run with: go test -tags integration ./internal/adapters/opencodev2/...
package opencodev2

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type liveServer struct {
	*Server
	client  *Client
	project string
	root    string
}

func startLiveServer(t *testing.T, config string, readyAgent string) *liveServer {
	t.Helper()
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode binary not found")
	}
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	require.NoError(t, os.MkdirAll(project, 0o755))
	require.NoError(t, exec.Command("git", "init", "-q", project).Run())

	srv, err := StartServer(context.Background(), ServerOptions{
		WorkDir:       project,
		DataDir:       filepath.Join(root, "data"),
		ConfigContent: config,
		ReadyAgent:    readyAgent,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = srv.Stop(context.Background(), 5*time.Second) })
	return &liveServer{Server: srv, client: srv.Client, project: project, root: root}
}

func TestContractServer(t *testing.T) {
	cfg := `{"agents":{"build":{"disabled":true},"plan":{"disabled":true},"general":{"disabled":true},"explore":{"disabled":true},` +
		`"pinger":{"mode":"primary","description":"contract agent"}}}`
	s := startLiveServer(t, cfg, "pinger")
	ctx := context.Background()
	c := s.client

	info, err := c.Info(ctx)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(info.Version, "2."), "version %s", info.Version)

	agents, err := c.Agents(ctx, s.project)
	require.NoError(t, err)
	visible := map[string]bool{}
	for _, a := range agents {
		if !a.Hidden {
			visible[a.ID] = true
		}
	}
	assert.Equal(t, map[string]bool{"pinger": true}, visible)

	skills, err := c.Skills(ctx, s.project)
	require.NoError(t, err)
	ids := []string{}
	for _, sk := range skills {
		ids = append(ids, sk.ID)
	}
	assert.Contains(t, ids, "opencode", "built-in skills are expected until denied by permissions")

	// Unauthorized
	_, err = NewClient(c.BaseURL(), "wrong").Info(ctx)
	assert.True(t, IsUnauthorized(err))

	// Events: subscribe before mutating
	evCtx, evCancel := context.WithTimeout(ctx, 20*time.Second)
	defer evCancel()
	events, _, err := c.Events(evCtx)
	require.NoError(t, err)
	first := <-events
	assert.Equal(t, EventTypeConnected, first.Type)

	// Sessions
	id := "ses_contract0000000000000001"
	created, err := c.CreateSession(ctx, CreateSessionRequest{
		ID: id, Title: "contract", Agent: "pinger",
		Location:    &Location{Directory: s.project},
		Permissions: []Rule{{Action: "skill", Resource: "opencode", Effect: "deny"}},
	})
	require.NoError(t, err)
	assert.Equal(t, id, created.ID)
	assert.Equal(t, "pinger", created.Agent)

	got, err := c.GetSession(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "contract", got.Title)
	assert.Equal(t, s.project, got.Location.Directory)

	list, err := c.ListSessions(ctx, s.project)
	require.NoError(t, err)
	require.Len(t, list, 1)

	_, err = c.GetSession(ctx, "ses_missing000000000000000001")
	assert.True(t, IsNotFound(err))

	perms, err := c.Permissions(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, perms)
	forms, err := c.Forms(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, forms)
	diff, err := c.Diff(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, diff)
	require.NoError(t, c.SetEnvironment(ctx, id, map[string]string{"OH_SESSION": id}))
	require.NoError(t, c.Wait(ctx, id))

	sawCreated := false
	for !sawCreated {
		select {
		case ev, ok := <-events:
			require.True(t, ok, "event stream closed")
			if ev.Type == "session.created" && ev.SessionID() == id {
				sawCreated = true
			}
		case <-evCtx.Done():
			t.Fatal("session.created event not received")
		}
	}

	code, err := c.Pair(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, code.Code)
	assert.Contains(t, c.PairURL(code.Code), "/auth/connect/")
}

func TestContractServerLifecycle(t *testing.T) {
	cfg := `{"agents":{"pinger":{"mode":"primary","description":"p"}}}`
	t0 := time.Now()
	s := startLiveServer(t, cfg, "pinger")
	startup := time.Since(t0)
	t.Logf("server ready in %s", startup)
	assert.Less(t, startup, 10*time.Second)
	assert.True(t, s.Alive())
	assert.True(t, s.Healthy(context.Background()))

	// Data isolation: the tool database lives in the oh-provided data dir.
	_, err := os.Stat(filepath.Join(s.root, "data", "opencode", "opencode.db"))
	assert.NoError(t, err)

	// Re-attach from another "oh process" with only url/password/pid.
	again := AttachServer(s.URL, s.Password, s.PID)
	assert.True(t, again.Alive())
	assert.True(t, again.Healthy(context.Background()))

	require.NoError(t, s.Stop(context.Background(), 5*time.Second))
	assert.False(t, s.Alive())
	assert.False(t, again.Healthy(context.Background()))

	// No process left in the server's process group.
	out, _ := exec.Command("pgrep", "-g", fmt.Sprint(s.PID)).Output()
	assert.Empty(t, strings.TrimSpace(string(out)), "orphan processes in group %d", s.PID)

	// Port released.
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", s.Port))
	require.NoError(t, err)
	l.Close()
}

func TestContractServerStartupFailure(t *testing.T) {
	if _, err := exec.LookPath("opencode"); err != nil {
		t.Skip("opencode binary not found")
	}
	root := t.TempDir()
	_, err := StartServer(context.Background(), ServerOptions{
		WorkDir:      root,
		DataDir:      filepath.Join(root, "data"),
		ReadyAgent:   "does-not-exist",
		ReadyTimeout: 4 * time.Second,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does-not-exist")
}
