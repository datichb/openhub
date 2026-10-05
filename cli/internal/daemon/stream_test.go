package daemon

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

type notes struct {
	mu  sync.Mutex
	got [][2]string
}

func (n *notes) add(_ context.Context, title, msg string) error {
	n.mu.Lock()
	n.got = append(n.got, [2]string{title, msg})
	n.mu.Unlock()
	return nil
}

func (n *notes) list() [][2]string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([][2]string(nil), n.got...)
}

func TestStreamFeedSubagentsAndNotifications(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions := sqlite.NewSessionStore(st)
	servers := sqlite.NewServerStore(st)
	decisions := sqlite.NewDecisionStore(st)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_r", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive, EntryAgent: "lead"}))

	fake := newFake()
	var sent notes
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Decisions: decisions, Tick: time.Hour, IdleAfter: time.Hour,
			Notify: sent.add, NotifyWindow: 100 * time.Millisecond,
			ProjectName: func(context.Context, string) string { return "openhub" },
			Adapter:     func(string) adapters.ToolAdapter { return fake }})
	}()
	t.Cleanup(func() { cancel(); <-done })
	var ch chan adapters.ToolEvent
	select {
	case ch = <-fake.events:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not subscribe")
	}
	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}

	c := NewClient(p)
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 3*time.Second, 20*time.Millisecond)
	sctx, scancel := context.WithCancel(ctx)
	defer scancel()
	feed, err := c.Stream(sctx, "ses_r")
	require.NoError(t, err)
	changes, err := c.Stream(sctx, "")
	require.NoError(t, err)

	next := func(evs <-chan StreamEvent, want func(StreamEvent) bool) StreamEvent {
		t.Helper()
		deadline := time.After(3 * time.Second)
		for {
			select {
			case ev, ok := <-evs:
				require.True(t, ok, "stream closed")
				if want(ev) {
					return ev
				}
			case <-deadline:
				t.Fatal("event not received")
			}
		}
	}
	isFeed := func(kind domain.FeedKind) func(StreamEvent) bool {
		return func(ev StreamEvent) bool { return ev.Feed != nil && ev.Feed.Kind == kind }
	}

	agent := &domain.FeedItem{SessionID: "ses_r", Kind: domain.FeedAgent, Agent: "lead"}
	ch <- adapters.ToolEvent{Kind: adapters.EventActivity, SessionID: "ses_r", Feed: agent}
	ch <- adapters.ToolEvent{Kind: adapters.EventActivity, SessionID: "ses_r", Feed: agent} // same agent: not repeated
	ch <- adapters.ToolEvent{Kind: adapters.EventActivity, SessionID: "ses_r", Feed: &domain.FeedItem{SessionID: "ses_r", Kind: domain.FeedTool, Tool: "shell", Title: "ls"}}
	assert.Equal(t, "lead", next(feed, isFeed(domain.FeedAgent)).Feed.Agent)
	assert.Equal(t, "ls", next(feed, isFeed(domain.FeedTool)).Feed.Title)

	// A subagent session: its activity is shown on the root session.
	ch <- adapters.ToolEvent{Kind: adapters.EventSessionCreated, SessionID: "ses_c", ParentID: "ses_r"}
	ch <- adapters.ToolEvent{Kind: adapters.EventActivity, SessionID: "ses_c", Feed: &domain.FeedItem{SessionID: "ses_c", Kind: domain.FeedAgent, Agent: "helper"}}
	ch <- adapters.ToolEvent{Kind: adapters.EventActivity, SessionID: "ses_c", Feed: &domain.FeedItem{SessionID: "ses_c", Kind: domain.FeedText, Text: "2+2=4"}}
	assert.Equal(t, "helper", next(feed, isFeed(domain.FeedAgent)).Feed.Agent)
	txt := next(feed, isFeed(domain.FeedText)).Feed
	assert.Equal(t, "ses_r", txt.SessionID)
	assert.Equal(t, "helper", txt.Agent)

	// The subagent asks a permission: filed under the root session, answered on the child.
	fake.set(func() {
		fake.decisions["ses_c"] = []adapters.PendingDecision{{ID: "per_c", SessionID: "ses_c", Kind: adapters.DecisionPermission, Action: "shell"}}
	})
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionAsked, SessionID: "ses_c"}
	next(changes, func(ev StreamEvent) bool {
		return ev.Change != nil && ev.Change.SessionID == "ses_r" && ev.Change.Decisions
	})
	require.Eventually(t, func() bool {
		open, _ := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_r"})
		return len(open) == 1 && open[0].ToolSessionID() == "ses_c"
	}, 3*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool { s, _ := sessions.Get(ctx, "ses_r"); return s.State == domain.RunWaiting }, 3*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool { return len(sent.list()) == 1 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "openhub · lead", sent.list()[0][1])
	assert.Contains(t, sent.list()[0][0], "!")

	// Answered in the tool → resolved, root back to idle; a finished turn
	// with nobody attached is notified.
	fake.set(func() { fake.decisions["ses_c"] = nil })
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionReplied, SessionID: "ses_c"}
	require.Eventually(t, func() bool {
		open, _ := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_r"})
		return len(open) == 0
	}, 3*time.Second, 20*time.Millisecond)
	ch <- adapters.ToolEvent{Kind: adapters.EventExecEnded, Outcome: "succeeded", SessionID: "ses_r"}
	next(changes, func(ev StreamEvent) bool { return ev.Change != nil && ev.Change.State == domain.RunIdle })
	require.Eventually(t, func() bool { return len(sent.list()) == 2 }, 3*time.Second, 20*time.Millisecond)

	// The backlog is replayed to a late follower.
	late, err := c.Stream(sctx, "ses_r")
	require.NoError(t, err)
	assert.Equal(t, "lead", next(late, isFeed(domain.FeedAgent)).Feed.Agent)
}

func TestNotifierGroups(t *testing.T) {
	var sent notes
	d := &Daemon{opts: Options{}}
	n := newNotifier(d, sent.add)
	n.window = 50 * time.Millisecond
	n.mu.Lock()
	n.enqueueLocked(note{kind: "permission", sessionID: "a"})
	n.enqueueLocked(note{kind: "question", sessionID: "b"})
	n.enqueueLocked(note{kind: "done", sessionID: "c"})
	n.mu.Unlock()
	require.Eventually(t, func() bool { return len(sent.list()) == 1 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, "oh", sent.list()[0][0])
}
