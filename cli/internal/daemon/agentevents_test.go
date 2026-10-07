package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// QB2: the daemon records one agent_events row per agent of a session (the
// entry agent and each subagent), with status, tokens, cost and skills, so
// that the per-agent metrics are fed again in v5.
func TestWatcherRecordsAgentEvents(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions, servers, events := sqlite.NewSessionStore(st), sqlite.NewServerStore(st), sqlite.NewAgentEventStore(st)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", Status: domain.SessionStatusRunning,
		GroupKey: "g1", State: domain.RunActive, EntryAgent: "orchestrator-dev"}))

	fake := newFake()
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, AgentEvents: events, Tick: time.Hour, IdleAfter: time.Hour,
			Adapter: func(string) adapters.ToolAdapter { return fake }})
	}()
	t.Cleanup(func() { cancel(); <-done })
	var ch chan adapters.ToolEvent
	select {
	case ch = <-fake.events:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not subscribe")
	}
	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}

	ch <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_r"}
	ch <- adapters.ToolEvent{Kind: adapters.EventSessionCreated, SessionID: "ses_c", ParentID: "ses_r"}
	ch <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_c"}
	ch <- adapters.ToolEvent{SessionID: "ses_c", Feed: &domain.FeedItem{Kind: domain.FeedAgent, Agent: "developer"}}
	ch <- adapters.ToolEvent{SessionID: "ses_c", Call: &adapters.ToolCall{ID: "k1", Action: sessionspec.ActionSkill, Status: adapters.CallCalled, Skill: "dev-standards-go"}}
	fake.set(func() {
		fake.usage["ses_c"] = adapters.SessionResult{Cost: 0.5, TokensIn: 300, TokensOut: 80}
		fake.usage["ses_r"] = adapters.SessionResult{Cost: 0.1, TokensIn: 50, TokensOut: 10}
	})
	ch <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_c", Outcome: "failed"}
	ch <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_r", Outcome: "succeeded"}

	var got []domain.AgentEvent
	require.Eventually(t, func() bool {
		got, err = events.ListBySession(ctx, "ses_r")
		return err == nil && len(got) == 2
	}, 3*time.Second, 20*time.Millisecond)
	by := map[string]domain.AgentEvent{}
	for _, e := range got {
		by[e.AgentName] = e
	}
	dev, entry := by["developer"], by["orchestrator-dev"]
	assert.Equal(t, "ses_c", dev.ID)
	assert.Equal(t, domain.AgentEventFailed, dev.Status)
	assert.InDelta(t, 0.5, dev.CostUSD, 1e-9)
	assert.Equal(t, int64(300), dev.TokensIn)
	assert.Equal(t, []string{"dev-standards-go"}, dev.SkillsLoaded)
	assert.NotNil(t, dev.CompletedAt)
	assert.Equal(t, domain.AgentEventSuccess, entry.Status)
	assert.Equal(t, "p1", entry.ProjectID)

	// A second step of the entry agent updates its row (no duplicate).
	fake.set(func() { fake.usage["ses_r"] = adapters.SessionResult{Cost: 0.3, TokensIn: 90, TokensOut: 20} })
	ch <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_r", Outcome: "interrupted"}
	require.Eventually(t, func() bool {
		got, _ = events.ListBySession(ctx, "ses_r")
		for _, e := range got {
			if e.AgentName == "orchestrator-dev" && e.Status == domain.AgentEventCancelled {
				return len(got) == 2 && e.CostUSD > 0.29
			}
		}
		return false
	}, 3*time.Second, 20*time.Millisecond)
	m, err := events.Metrics(ctx, "p1")
	require.NoError(t, err)
	assert.Len(t, m, 2)
}
