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
	client  *Client
	project string
	cmd     *exec.Cmd
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startLiveServer(t *testing.T, config string) *liveServer {
	t.Helper()
	bin, err := exec.LookPath("opencode")
	if err != nil {
		t.Skip("opencode binary not found")
	}
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	require.NoError(t, os.MkdirAll(project, 0o755))
	require.NoError(t, exec.Command("git", "init", "-q", project).Run())

	port := freePort(t)
	cmd := exec.Command(bin, "serve", "--hostname", "127.0.0.1", "--port", fmt.Sprint(port))
	cmd.Dir = project
	cmd.Env = append(os.Environ(),
		"OPENCODE_SERVER_PASSWORD=contract",
		"XDG_DATA_HOME="+filepath.Join(root, "data"),
		"OPENCODE_DISABLE_PROJECT_CONFIG=1",
		"OPENCODE_DISABLE_AUTOUPDATE=true",
		"OPENCODE_CONFIG_CONTENT="+config,
	)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	c := NewClient(fmt.Sprintf("http://127.0.0.1:%d", port), "contract")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		agents, err := c.Agents(ctx, project)
		if err == nil && len(agents) > 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("server not ready: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
	}
	return &liveServer{client: c, project: project, cmd: cmd}
}

func TestContractServer(t *testing.T) {
	cfg := `{"agents":{"build":{"disabled":true},"plan":{"disabled":true},"general":{"disabled":true},"explore":{"disabled":true},` +
		`"pinger":{"mode":"primary","description":"contract agent"}}}`
	s := startLiveServer(t, cfg)
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
