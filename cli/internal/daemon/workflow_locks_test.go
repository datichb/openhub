package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
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

func deniesShell(rules []sessionspec.PermissionRule, pattern string) bool {
	for _, r := range rules {
		if r.Action == sessionspec.ActionShell && r.Resource == pattern && r.Effect == sessionspec.EffectDeny {
			return true
		}
	}
	return false
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// A17, A36, A38 (piste T, T1): before cp-2, commit and closing are refused
// (session rules, Beads gateway); after it, closing waits for a commit and
// locks again; a question imitating a checkpoint is answered by oh; the end
// of the workflow completes the session with its outputs.
func TestWorkflowLocksAndEnd(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	project, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	gitRun(t, project, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(project, "a.txt"), []byte("a"), 0o644))
	gitRun(t, project, "add", ".")
	gitRun(t, project, "commit", "-qm", "init")
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1',?)`, project)
	require.NoError(t, err)
	sessions, servers, decisions := sqlite.NewSessionStore(st), sqlite.NewServerStore(st), sqlite.NewDecisionStore(st)
	states := sqlite.NewCheckpointStore(st)

	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket", DefaultMode: "manuel",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-1", Label: map[string]string{"": "Démarrer le ticket"}, Behaviors: map[string]string{"manuel": "pause"}},
			{ID: "cp-2", Label: map[string]string{"": "Commit ou correction"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true,
				Unlocks: []string{sessionspec.UnlockCommit, sessionspec.UnlockPush, sessionspec.UnlockClose}},
			{ID: "cp-3", Label: map[string]string{"": "Ticket suivant"}, Behaviors: map[string]string{"manuel": "pause"}},
		},
		Outputs: []sessionspec.OutputDef{{ID: "branch", Type: "branch"}, {ID: "tickets", Type: "beads-ids"}},
	}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h1", Mode: "manuel", State: domain.RunActive, LaunchPath: project, WorkflowID: "ticket"}))

	bd := filepath.Join(t.TempDir(), "bd")
	require.NoError(t, os.WriteFile(bd, []byte("#!/bin/sh\necho \"bd $*\"\n"), 0o755))
	fake := &rulesAdapter{fakeAdapter: newFake(), rules: map[string][]sessionspec.PermissionRule{}}
	fake.usage["ses_a"] = adapters.SessionResult{Branch: "feat/pt-1"}
	cp := &checkpoint.Service{Sessions: sessions, States: states, SessionOutputs: states, BundlesDir: bundles, SessionsDir: t.TempDir()}
	var endMu sync.Mutex
	var ended []domain.Session
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Decisions: decisions, Checkpoints: cp,
			Tick: time.Hour, IdleAfter: time.Hour, BeadsBinary: bd,
			OnSessionEnd: func(_ context.Context, s domain.Session) { endMu.Lock(); ended = append(ended, s); endMu.Unlock() },
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
	g, err := c.IssueGatewayGrant(ctx, GatewayGrantRequest{SessionID: "ses_a", GroupKey: "g1", ProjectID: "p1", Location: project, BeadsAllow: []string{"show", "update", "close"}})
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
	state := func() domain.CheckpointState {
		s, err := states.GetCheckpointState(ctx, "ses_a")
		require.NoError(t, err)
		return s
	}

	// Before cp-2: commit, push and closing are refused to every agent.
	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}
	require.Eventually(t, func() bool { return deniesShell(fake.rulesOf("ses_a"), "*git*commit*") }, 3*time.Second, 20*time.Millisecond)
	assert.True(t, deniesShell(fake.rulesOf("ses_a"), "*git*push*"))
	assert.True(t, deniesShell(fake.rulesOf("ses_a"), "*bd*close*"))
	code, _ := bdExec("update", "pt-1", "--claim")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"pt-1"}, state().Claimed)
	code, out := bdExec("close", "pt-1", "--reason", "done")
	assert.Equal(t, http.StatusForbidden, code)
	assert.Contains(t, out.Error, "cp-2")
	code, _ = bdExec("update", "pt-1", "-s", "closed")
	assert.Equal(t, http.StatusForbidden, code, "closing through update is refused too")

	// A question imitating cp-2 is answered by oh with the instruction (A36).
	fake.set(func() {
		fake.decisions["ses_a"] = []adapters.PendingDecision{{ID: "que_1", SessionID: "ses_a", Kind: adapters.DecisionQuestion, Title: "[OrchestratorDev — CP-2]",
			Fields: []adapters.FormField{{Key: "q0", Title: "Commit ou corriger ?", Type: "string", Custom: true, Options: []adapters.FormOption{{Value: "Commit", Label: "Commit"}}}}}}
	})
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionAsked, SessionID: "ses_a"}
	require.Eventually(t, func() bool { return fake.replyCount() == 1 }, 3*time.Second, 20*time.Millisecond)
	fake.rmu.Lock()
	reply := fake.replies[0]
	fake.rmu.Unlock()
	assert.Equal(t, adapters.DecisionQuestion, reply.Kind)
	assert.Contains(t, reply.Answer["q0"], "workflow_checkpoint")
	l, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_a"})
	require.NoError(t, err)
	assert.Empty(t, l, "no question decision for an imitation")
	st0 := state()
	assert.False(t, st0.HasPassed("cp-2"))
	fake.set(func() { fake.decisions["ses_a"] = nil })

	// cp-2 passed: commit allowed; closing waits for the commit.
	_, err = c.WorkflowCheckpoint(ctx, "ses_a", checkpoint.Call{ID: "cp-2"})
	require.NoError(t, err)
	assert.False(t, deniesShell(fake.rulesOf("ses_a"), "*git*commit*"))
	require.NoError(t, os.WriteFile(filepath.Join(project, "a.txt"), []byte("b"), 0o644))
	code, out = bdExec("close", "pt-1")
	assert.Equal(t, http.StatusForbidden, code)
	assert.Contains(t, out.Error, "commit")
	gitRun(t, project, "commit", "-qam", "feat: b")
	code, out = bdExec("close", "pt-1", "--reason", "Implemented in commit x")
	require.Equal(t, http.StatusOK, code, out.Error)
	assert.Equal(t, []string{"pt-1"}, state().Closed)
	assert.True(t, deniesShell(fake.rulesOf("ses_a"), "*git*commit*"), "locked again once the ticket is closed")

	// The step ends with every ticket closed: completed, outputs declared (A38).
	ch <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_a", Outcome: "succeeded"}
	require.Eventually(t, func() bool { s, _ := sessions.Get(ctx, "ses_a"); return s.State == domain.RunCompleted }, 3*time.Second, 20*time.Millisecond)
	s, err := sessions.Get(ctx, "ses_a")
	require.NoError(t, err)
	assert.Equal(t, "feat/pt-1", s.Outputs["branch"])
	assert.Equal(t, []any{"pt-1"}, s.Outputs["tickets"])
	endMu.Lock()
	require.Len(t, ended, 1, "the end of the session runs (team claims), as when it is stopped")
	assert.Equal(t, domain.RunCompleted, ended[0].State)
	endMu.Unlock()

	// The user goes on: the session is back in play.
	ch <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_a"}
	require.Eventually(t, func() bool { s, _ := sessions.Get(ctx, "ses_a"); return s.State == domain.RunActive }, 3*time.Second, 20*time.Millisecond)
	assert.Nil(t, state().Finished)
	code, _ = bdExec("show", "pt-1")
	assert.Equal(t, http.StatusOK, code, "Beads reachable again")
}
