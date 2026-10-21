package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// Writes carry the actor set by oh (person and agent); a claim binds the
// ticket to the session and a close to its commit (bd provenance record).
func TestDaemonBeadsActorAndProvenance(t *testing.T) {
	p := shortPaths(t)
	st := sqlitetest.New(t)
	ctx := context.Background()
	project, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	gittest.Run(t, project, "init", "-q", "-b", "feat/x")
	gittest.Run(t, project, "config", "user.name", "Ada Lovelace")
	gittest.Run(t, project, "-c", "user.email=a@b", "commit", "-q", "--allow-empty", "-m", "work")
	sha := gittest.Run(t, project, "rev-parse", "HEAD")

	sqlitetest.Seed(t, st).Project("p1", project)
	sessions, servers := sqlite.NewSessionStore(st), sqlite.NewServerStore(st)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "x", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		State: domain.RunActive, EntryAgent: "orchestrator-dev"}))

	calls := filepath.Join(t.TempDir(), "calls")
	bd := filepath.Join(t.TempDir(), "bd")
	require.NoError(t, os.WriteFile(bd, []byte(fakeWhere+"printf '%s\\n' \"$*\" >> "+calls+"\n"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(project, ".beads"), 0o700))
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Tick: time.Hour, IdleAfter: time.Hour, BeadsBinary: bd})
	}()
	t.Cleanup(func() { cancel(); <-done })
	c := NewClient(p)
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 5*time.Second, 20*time.Millisecond)
	h, err := c.Health(ctx)
	require.NoError(t, err)
	g, err := c.IssueGatewayGrant(ctx, GatewayGrantRequest{SessionID: "ses_a", GroupKey: "g1", ProjectID: "p1", WorkflowID: "ticket",
		Location: project, BeadsAllow: []string{"show", "update", "close"}})
	require.NoError(t, err)
	bdExec := func(argv ...string) {
		body, _ := json.Marshal(beadswire.ExecRequest{Argv: argv, Cwd: project})
		req, _ := http.NewRequest(http.MethodPost, h.ProxyURL+"/"+beadswire.Prefix+beadswire.ExecPath, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+g.Token)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	bdExec("update", "pt-1", "--claim")
	bdExec("update", "pt-1", "--title", "t")
	bdExec("close", "pt-1")
	data, err := os.ReadFile(calls)
	require.NoError(t, err)
	actor := "--actor=Ada Lovelace via oh/orchestrator-dev"
	assert.Equal(t, []string{
		"--actor=Ada Lovelace update --claim -- pt-1",
		"provenance record --issue=pt-1 --kind=claim --source=oh --ref=ses_a --ref-kind=work-id " + actor + ` --payload={"workflow":"ticket"}`,
		actor + " update --title=t -- pt-1",
		actor + " close -- pt-1",
		"provenance record --issue=pt-1 --kind=commit --source=oh --ref=" + sha + " --ref-kind=git-sha " + actor + ` --payload={"branch":"feat/x","session":"ses_a"}`,
	}, strings.Split(strings.TrimSpace(string(data)), "\n"))
}
