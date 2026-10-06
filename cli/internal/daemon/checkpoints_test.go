package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// rulesAdapter records replies and session rules.
type rulesAdapter struct {
	*fakeAdapter
	rmu     sync.Mutex
	rules   map[string][]sessionspec.PermissionRule
	replies []adapters.DecisionReply
}

func (r *rulesAdapter) SetSessionRules(_ context.Context, _ adapters.ServerHandle, id string, rules []sessionspec.PermissionRule) error {
	r.rmu.Lock()
	defer r.rmu.Unlock()
	r.rules[id] = rules
	return nil
}

func (r *rulesAdapter) Reply(_ context.Context, _ adapters.ServerHandle, d adapters.DecisionReply) error {
	r.rmu.Lock()
	defer r.rmu.Unlock()
	r.replies = append(r.replies, d)
	return nil
}

func (r *rulesAdapter) rulesOf(id string) []sessionspec.PermissionRule {
	r.rmu.Lock()
	defer r.rmu.Unlock()
	return r.rules[id]
}

func (r *rulesAdapter) replyCount() int {
	r.rmu.Lock()
	defer r.rmu.Unlock()
	return len(r.replies)
}

func denies(rules []sessionspec.PermissionRule, agent string) bool {
	for _, r := range rules {
		if r.Action == sessionspec.ActionSubagent && r.Resource == agent && r.Effect == sessionspec.EffectDeny {
			return true
		}
	}
	return false
}

func TestWatcherCheckpoints(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions, servers, decisions := sqlite.NewSessionStore(st), sqlite.NewServerStore(st), sqlite.NewDecisionStore(st)
	states := sqlite.NewCheckpointStore(st)

	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-1", Behaviors: map[string]string{"semi-auto": "auto"}},
			{ID: "cp-2", Label: map[string]string{"": "Commit"}, Behaviors: map[string]string{"semi-auto": "pause"}},
		},
		Gates:                   []sessionspec.AgentGate{{Agent: "developer", After: "cp-1"}},
		MaxConsecutiveSubagents: 3,
	}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h1", Mode: "semi-auto", State: domain.RunActive}))

	fake := &rulesAdapter{fakeAdapter: newFake(), rules: map[string][]sessionspec.PermissionRule{}}
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
	var ch chan adapters.ToolEvent
	select {
	case ch = <-fake.events:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not subscribe")
	}
	open := func() []domain.Decision {
		l, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_a"})
		require.NoError(t, err)
		return l
	}

	// Resync: the rules of the session are applied (developer locked).
	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}
	require.Eventually(t, func() bool { return denies(fake.rulesOf("ses_a"), "developer") }, 3*time.Second, 20*time.Millisecond)

	// cp-1 is automatic in semi-auto: let through, no decision.
	cpCall := func(id string) *adapters.ToolCall {
		return &adapters.ToolCall{ID: "call_" + id, Action: bundle.CheckpointAction(), Status: adapters.CallCalled, Input: map[string]any{"id": id, "summary": "s " + id}}
	}
	fake.set(func() {
		fake.decisions["ses_a"] = []adapters.PendingDecision{{ID: "per_1", SessionID: "ses_a", Kind: adapters.DecisionPermission, Action: "workflow_workflow_checkpoint", Call: cpCall("cp-1")}}
	})
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionAsked, SessionID: "ses_a"}
	require.Eventually(t, func() bool { return fake.replyCount() == 1 }, 3*time.Second, 20*time.Millisecond)
	fake.rmu.Lock()
	assert.Equal(t, "once", fake.replies[0].Decision)
	fake.rmu.Unlock()
	assert.Empty(t, open())

	// The call runs (MCP server → daemon): cp-1 passed, developer released.
	fake.set(func() { fake.decisions["ses_a"] = nil })
	c := NewClient(p)
	_, err = c.WorkflowCheckpoint(ctx, "ses_a", checkpoint.Call{ID: "cp-1"})
	require.NoError(t, err)
	assert.False(t, denies(fake.rulesOf("ses_a"), "developer"))

	// cp-2 pauses: ⏸ decision in the inbox.
	fake.set(func() {
		fake.decisions["ses_a"] = []adapters.PendingDecision{{ID: "per_2", SessionID: "ses_a", Kind: adapters.DecisionPermission, Action: "workflow_workflow_checkpoint", Call: cpCall("cp-2")}}
	})
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionAsked, SessionID: "ses_a"}
	require.Eventually(t, func() bool { l := open(); return len(l) == 1 && l[0].Kind == domain.DecisionCheckpoint }, 3*time.Second, 20*time.Millisecond)
	d := open()[0]
	assert.Equal(t, "per_2", d.ToolRef)
	assert.Equal(t, "Commit", d.Payload.Title)
	assert.Equal(t, "cp-2", d.Payload.Data[checkpoint.DataCheckpoint])
	s, _ := states.GetCheckpointState(ctx, "ses_a")
	assert.Equal(t, "cp-2", s.Waiting)

	// Validated in the tool UI: no longer listed → resolved by the tool.
	fake.set(func() { fake.decisions["ses_a"] = nil })
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionReplied, SessionID: "ses_a"}
	require.Eventually(t, func() bool { return len(open()) == 0 }, 3*time.Second, 20*time.Millisecond)
	s, _ = states.GetCheckpointState(ctx, "ses_a")
	assert.Empty(t, s.Waiting)

	// Circuit breaker: 3 delegations without the user → ✗ decision, delegation held.
	for i := 0; i < 3; i++ {
		ch <- adapters.ToolEvent{Kind: adapters.EventActivity, SessionID: "ses_a", Call: &adapters.ToolCall{ID: "sub" + string(rune('a'+i)), Action: sessionspec.ActionSubagent, Status: adapters.CallCalled, Input: map[string]any{"agent": "developer"}}}
	}
	require.Eventually(t, func() bool { l := open(); return len(l) == 1 && l[0].Kind == domain.DecisionCircuit }, 3*time.Second, 20*time.Millisecond)
	assert.True(t, denies(fake.rulesOf("ses_a"), "*"))
	assert.Equal(t, domain.RunWaiting, func() domain.RunState { s, _ := sessions.Get(ctx, "ses_a"); return s.State }())

	// A delegation done records the agent.
	ch <- adapters.ToolEvent{Kind: adapters.EventActivity, SessionID: "ses_a", Call: &adapters.ToolCall{ID: "suba", Action: sessionspec.ActionSubagent, Status: adapters.CallOK}}
	require.Eventually(t, func() bool { s, _ := states.GetCheckpointState(ctx, "ses_a"); return s.HasRun("developer") }, 3*time.Second, 20*time.Millisecond)
}
