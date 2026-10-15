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

// r9b (triage): through the real Beads gateway, every change to tickets
// (update, label, duplicate…) is refused before the checkpoint that unlocks
// `update`, reading is not; after it, the changed tickets are recorded in
// the output declared with `record: updated`, a duplicate closed in the one
// with `record: closed` (no checkpoint unlocks closing in this workflow).
func TestWorkflowUpdateGuard(t *testing.T) {
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
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "spec", DefaultMode: "manuel",
		Outputs: []sessionspec.OutputDef{{ID: "updated", Type: "beads-ids", Record: checkpoint.RecordUpdated},
			{ID: "closed", Type: "beads-ids", Record: checkpoint.RecordClosed}},
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-triage", Label: map[string]string{"": "Changements proposés"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true,
				Unlocks: []string{sessionspec.UnlockCreate, sessionspec.UnlockUpdate}},
		}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_c", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h1", Mode: "manuel", State: domain.RunActive, LaunchPath: project, WorkflowID: "triage"}))

	bd := filepath.Join(t.TempDir(), "bd")
	created := `case " $* " in *" create "*) n=$(cat "$0.n" 2>/dev/null || echo 0); n=$((n+1)); echo $n > "$0.n"; echo "✓ Created issue: ev-$n — x";; esac` + "\n"
	require.NoError(t, os.WriteFile(bd, []byte(fakeWhere+"echo \"bd $*\"\n"+created), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".beads"), 0o700))
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
	g, err := c.IssueGatewayGrant(ctx, GatewayGrantRequest{SessionID: "ses_c", GroupKey: "g1", ProjectID: "p1", Location: project, BeadsAllow: []string{"show", "list", "create", "update", "label", "duplicate"}})
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
	require.Eventually(t, func() bool { return deniesShell(fake.rulesOf("ses_c"), "bd update*") }, 3*time.Second, 20*time.Millisecond)
	assert.True(t, deniesShell(fake.rulesOf("ses_c"), "bd label add*"))
	for _, argv := range [][]string{{"update", "ev-1", "-p", "1"}, {"label", "add", "ev-1", "ev-2", "bug"}, {"duplicate", "ev-2", "--of", "ev-1"}} {
		code, out := bdExec(argv...)
		assert.Equal(t, http.StatusForbidden, code, argv)
		assert.Contains(t, out.Error, "cp-triage", argv)
	}
	code, _ := bdExec("list")
	assert.Equal(t, http.StatusOK, code, "reading is not locked")

	fake.set(func() {
		fake.decisions["ses_c"] = []adapters.PendingDecision{{ID: "per_cpt", SessionID: "ses_c", Kind: adapters.DecisionPermission,
			Call: &adapters.ToolCall{ID: "call_1", Action: "mcp:workflow/checkpoint", Input: map[string]any{"id": "cp-triage", "summary": "changes"}}}}
	})
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionAsked, SessionID: "ses_c"}
	require.Eventually(t, func() bool {
		st, err := states.GetCheckpointState(ctx, "ses_c")
		return err == nil && st.Waiting == "cp-triage"
	}, 3*time.Second, 20*time.Millisecond)
	fake.set(func() { fake.decisions["ses_c"] = nil })
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionReplied, SessionID: "ses_c", Answer: &adapters.DecisionAnswer{ID: "per_cpt", Approved: true}}
	_, err = c.WorkflowCheckpoint(ctx, "ses_c", checkpoint.Call{ID: "cp-triage"})
	require.NoError(t, err)
	assert.False(t, deniesShell(fake.rulesOf("ses_c"), "bd update*"))
	for _, argv := range [][]string{{"update", "ev-1", "-p", "1"}, {"label", "add", "ev-1", "ev-3", "needs-split"}, {"duplicate", "ev-2", "--of", "ev-1"}} {
		code, out := bdExec(argv...)
		require.Equal(t, http.StatusOK, code, "%v: %s", argv, out.Error)
	}
	sess, err := sessions.Get(ctx, "ses_c")
	require.NoError(t, err)
	assert.Equal(t, []any{"ev-1", "ev-3"}, sess.Outputs["updated"], "changed tickets recorded by oh, not the label")
	assert.Equal(t, []any{"ev-2"}, sess.Outputs["closed"], "the duplicate closed")
}
