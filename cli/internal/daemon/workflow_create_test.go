package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// N9 (DC8): through the real Beads gateway, `bd create` is refused before
// the checkpoint that unlocks `create` (cadrage, feature: cp-tickets) and
// accepted after it.
func TestWorkflowCreateGuard(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	project, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	gitRun(t, project, "init", "-q")
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1',?)`, project)
	require.NoError(t, err)
	sessions, servers, decisions := sqlite.NewSessionStore(st), sqlite.NewServerStore(st), sqlite.NewDecisionStore(st)
	states := sqlite.NewCheckpointStore(st)

	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "cadrage", DefaultMode: "manuel",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-tickets", Label: map[string]string{"": "Valider le découpage"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true,
				Unlocks: []string{sessionspec.UnlockCreate}},
			{ID: "cp-recap", Label: map[string]string{"": "Récap du cadrage"}, Behaviors: map[string]string{"manuel": "pause"}},
		}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_c", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h1", Mode: "manuel", State: domain.RunActive, LaunchPath: project, WorkflowID: "cadrage"}))

	bd := filepath.Join(t.TempDir(), "bd")
	require.NoError(t, os.WriteFile(bd, []byte("#!/bin/sh\necho \"bd $*\"\n"), 0o755))
	fake := &rulesAdapter{fakeAdapter: newFake(), rules: map[string][]sessionspec.PermissionRule{}}
	cp := &checkpoint.Service{Sessions: sessions, States: states, SessionOutputs: states, BundlesDir: bundles, SessionsDir: t.TempDir()}
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Decisions: decisions, Checkpoints: cp,
			Tick: time.Hour, IdleAfter: time.Hour, BeadsBinary: bd,
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
	c := NewClient(p)
	h, err := c.Health(ctx)
	require.NoError(t, err)
	g, err := c.IssueGatewayGrant(ctx, GatewayGrantRequest{SessionID: "ses_c", GroupKey: "g1", ProjectID: "p1", Location: project, BeadsAllow: []string{"show", "list", "create", "q"}})
	require.NoError(t, err)
	bdExec := func(argv ...string) (int, beadswire.ExecResponse) {
		body, _ := json.Marshal(beadswire.ExecRequest{Argv: argv, Cwd: project})
		req, _ := http.NewRequest(http.MethodPost, h.ProxyURL+"/"+beadswire.Prefix+beadswire.ExecPath, bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+g.Token)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		var out beadswire.ExecResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
		return resp.StatusCode, out
	}

	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}
	require.Eventually(t, func() bool { return deniesShell(fake.rulesOf("ses_c"), "*bd*create*") }, 3*time.Second, 20*time.Millisecond)
	code, out := bdExec("create", "Un ticket", "-t", "task")
	assert.Equal(t, http.StatusForbidden, code)
	assert.Contains(t, out.Error, "cp-tickets")
	code, _ = bdExec("q", "Un ticket")
	assert.Equal(t, http.StatusForbidden, code, "quick capture is a creation too")
	code, _ = bdExec("list")
	assert.Equal(t, http.StatusOK, code, "reading is not locked")

	_, err = c.WorkflowCheckpoint(ctx, "ses_c", checkpoint.Call{ID: "cp-tickets"})
	require.NoError(t, err)
	assert.False(t, deniesShell(fake.rulesOf("ses_c"), "*bd*create*"))
	code, out = bdExec("create", "Un ticket", "-t", "task")
	require.Equal(t, http.StatusOK, code, out.Error)
	code, _ = bdExec("create", "Un autre", "-t", "task")
	assert.Equal(t, http.StatusOK, code, "the window stays open for the whole split")
}
