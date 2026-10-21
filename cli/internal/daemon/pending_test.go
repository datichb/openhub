package daemon

import (
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
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// childAdapter lists the subagent sessions of the server (ChildLister).
type childAdapter struct {
	*rulesAdapter
	cmu     sync.Mutex
	parents map[string]string
}

func (c *childAdapter) Children(context.Context, adapters.ServerHandle) (map[string]string, error) {
	c.cmu.Lock()
	defer c.cmu.Unlock()
	out := map[string]string{}
	for k, v := range c.parents {
		out[k] = v
	}
	return out, nil
}

func (c *childAdapter) link(child, parent string) {
	c.cmu.Lock()
	defer c.cmu.Unlock()
	c.parents[child] = parent
}

func commitDenied(rules []sessionspec.PermissionRule) bool {
	for _, r := range rules {
		if r.Action == sessionspec.ActionShell && r.Resource == "git commit*" && r.Effect == sessionspec.EffectDeny {
			return true
		}
	}
	return false
}

// p6-debts: a request the plugin holds until oh confirms the session rules
// (subagent session created by the tool, not adopted yet; imported session)
// is answered from the session rules oh sets, never from the rules the tool
// held; a session oh cannot link is refused.
func TestHeldRequestBeforeAdoption(t *testing.T) {
	adoptTries, adoptRetry = 2, 20*time.Millisecond
	t.Cleanup(func() { adoptTries, adoptRetry = 5, 600*time.Millisecond })

	p := shortPaths(t)
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions, servers, decisions := sqlite.NewSessionStore(st), sqlite.NewServerStore(st), sqlite.NewDecisionStore(st)
	states := sqlite.NewCheckpointStore(st)

	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-1", Behaviors: map[string]string{"semi-auto": "auto"}},
			{ID: "cp-2", Label: map[string]string{"": "Commit"}, Behaviors: map[string]string{"semi-auto": "pause"}, Unlocks: []string{"commit"}},
		},
	}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	for _, id := range []string{"ses_a", "ses_b"} {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h1", Mode: "semi-auto", State: domain.RunActive}))
	}

	fake := &childAdapter{rulesAdapter: &rulesAdapter{fakeAdapter: newFake(), rules: map[string][]sessionspec.PermissionRule{}}, parents: map[string]string{}}
	cp := &checkpoint.Service{Sessions: sessions, States: states, BundlesDir: bundles, SessionsDir: t.TempDir()}
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Decisions: decisions, Checkpoints: cp, Tick: time.Hour, IdleAfter: time.Hour,
			Adapter: func(name string) adapters.ToolAdapter {
				if name == "fake" {
					return fake
				}
				return nil
			}})
	}()
	t.Cleanup(func() { cancel(); <-done })
	select {
	case <-fake.events:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not subscribe")
	}
	c := NewClient(p)
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 5*time.Second, 20*time.Millisecond)
	g, err := c.IssueGrant(ctx, GrantRequest{Owner: "g1", Provider: "amazon-bedrock", Region: "eu-west-1",
		Source: domain.CredentialSource{Kind: domain.CredentialBearer}, Secret: "real"})
	require.NoError(t, err)
	h, _ := c.Health(ctx)
	hook := func(body string) (int, HookPermissionResponse) {
		req, _ := http.NewRequest(http.MethodPost, h.ProxyURL+"/oh/v1/hooks/permission", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+g.Token)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var out HookPermissionResponse
		if resp.StatusCode == http.StatusOK {
			require.NoError(t, json.Unmarshal(raw, &out), string(raw))
		}
		return resp.StatusCode, out
	}
	held := func(session, cmd string) string {
		return `{"sessionID":"` + session + `","agent":"developer","action":"shell","resources":["` + cmd + `"],"pending":true}`
	}

	// A developer subagent commits before cp-2, its creation event not
	// handled: oh adopts it on the spot, sets its rules and refuses.
	fake.link("ses_child", "ses_a")
	code, out := hook(held("ses_child", "git commit -m wip"))
	assert.Equal(t, 200, code)
	assert.Equal(t, "deny", out.Effect, "locked until cp-2")
	assert.Contains(t, out.Message, "git commit -m wip")
	assert.True(t, commitDenied(fake.rulesOf("ses_child")), "adopted: its session rules are set")

	// Nested delegation: the grandchild is linked through its parent.
	fake.link("ses_grand", "ses_child2")
	fake.link("ses_child2", "ses_a")
	_, out = hook(held("ses_grand", "git commit -m wip"))
	assert.Equal(t, "deny", out.Effect)
	assert.True(t, commitDenied(fake.rulesOf("ses_grand")))
	assert.True(t, commitDenied(fake.rulesOf("ses_child2")))

	// A session oh cannot link to one of its sessions: refused, with why.
	code, out = hook(held("ses_ghost", "git commit -m wip"))
	assert.Equal(t, 200, code)
	assert.Equal(t, "deny", out.Effect)
	assert.NotEmpty(t, out.Message)
	code, _ = hook(`{"sessionID":"ses_ghost","action":"shell","resources":["git status"]}`)
	assert.Equal(t, 403, code, "a request that is not held: unchanged")

	// Once cp-2 unlocks the commit, oh confirms: the tool's decision stands.
	_, err = states.UpdateCheckpointState(ctx, "ses_a", func(s *domain.CheckpointState) error {
		s.Unlocks = map[string]domain.CheckpointUnlock{"commit": {Checkpoint: "cp-2", At: time.Now()}}
		s.Passed = map[string]time.Time{"cp-1": time.Now(), "cp-2": time.Now()}
		return nil
	})
	require.NoError(t, err)
	code, out = hook(held("ses_child", "git commit -m wip"))
	assert.Equal(t, 200, code)
	assert.Empty(t, out.Effect, "unlocked: the rendered decision stands")

	// An imported session (no session rules in the tool): oh answers from
	// its own state, and the rules route links its subagent sessions.
	_, out = hook(held("ses_b", "git commit -m x"))
	assert.Equal(t, "deny", out.Effect, "imported session: locked until its cp-2")
	fake.link("ses_bchild", "ses_b")
	require.NoError(t, c.WorkflowRefresh(ctx, "ses_b"))
	assert.True(t, commitDenied(fake.rulesOf("ses_b")))
	assert.True(t, commitDenied(fake.rulesOf("ses_bchild")), "the subagent sessions of an imported session get their rules")
}
