package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// fakeAdapter feeds events from a channel and serves pending/usage from maps.
type fakeAdapter struct {
	mu      sync.Mutex
	events  chan chan adapters.ToolEvent // one channel per subscription
	pending map[string]int
	active  map[string]bool
	usage   map[string]adapters.SessionResult
	subs    int
	failAll bool // ActiveSessions fails (server not authenticated / not ours)
}

func newFake() *fakeAdapter {
	return &fakeAdapter{events: make(chan chan adapters.ToolEvent, 4), pending: map[string]int{}, active: map[string]bool{}, usage: map[string]adapters.SessionResult{}}
}

func (f *fakeAdapter) Name() string { return "fake" }
func (f *fakeAdapter) Detect(context.Context) (adapters.ToolInfo, error) {
	return adapters.ToolInfo{}, nil
}
func (f *fakeAdapter) Capabilities() adapters.Capabilities { return adapters.Capabilities{} }
func (f *fakeAdapter) Render(sessionspec.BundleSpec, sessionspec.ProviderSpec) (adapters.RenderedConfig, error) {
	return adapters.RenderedConfig{}, nil
}
func (f *fakeAdapter) StartServer(context.Context, adapters.ServerGroup) (adapters.ServerHandle, error) {
	return adapters.ServerHandle{}, nil
}
func (f *fakeAdapter) StopServer(context.Context, adapters.ServerHandle) error { return nil }
func (f *fakeAdapter) Attest(context.Context, adapters.ServerHandle, sessionspec.BundleSpec, string) (adapters.VisibilityReport, error) {
	return adapters.VisibilityReport{}, nil
}
func (f *fakeAdapter) CreateSession(context.Context, adapters.ServerHandle, sessionspec.SessionSpec) error {
	return nil
}
func (f *fakeAdapter) SendPrompt(context.Context, adapters.ServerHandle, string, string) error {
	return nil
}
func (f *fakeAdapter) AttachCommand(adapters.ServerHandle, string) ([]string, []string) {
	return nil, nil
}
func (f *fakeAdapter) Events(ctx context.Context, _ adapters.ServerHandle) (<-chan adapters.ToolEvent, error) {
	ch := make(chan adapters.ToolEvent, 16)
	f.mu.Lock()
	f.subs++
	f.mu.Unlock()
	f.events <- ch
	return ch, nil
}
func (f *fakeAdapter) ActiveSessions(context.Context, adapters.ServerHandle) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failAll {
		return nil, errors.New("unauthorized")
	}
	var ids []string
	for id, on := range f.active {
		if on {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
func (f *fakeAdapter) Pending(_ context.Context, _ adapters.ServerHandle, id string) ([]adapters.PendingDecision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return make([]adapters.PendingDecision, f.pending[id]), nil
}
func (f *fakeAdapter) Reply(context.Context, adapters.ServerHandle, adapters.DecisionReply) error {
	return nil
}
func (f *fakeAdapter) Control(context.Context, adapters.ServerHandle, string, adapters.ControlOp) error {
	return nil
}
func (f *fakeAdapter) Results(_ context.Context, _ adapters.ServerHandle, id string) (adapters.SessionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.usage[id], nil
}

func (f *fakeAdapter) set(fn func()) { f.mu.Lock(); fn(); f.mu.Unlock() }

func TestWatcherTracksSessionState(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions := sqlite.NewSessionStore(st)
	servers := sqlite.NewServerStore(st)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	for _, id := range []string{"ses_a", "ses_b"} {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))
	}
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_other", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g2", State: domain.RunActive}))

	fake := newFake()
	fake.set(func() { fake.active["ses_a"] = true; fake.pending["ses_b"] = 1 })

	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Tick: time.Hour, IdleAfter: time.Hour,
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
	state := func(id string) domain.RunState {
		s, err := sessions.Get(ctx, id)
		require.NoError(t, err)
		return s.State
	}

	// Resynchronization on connect.
	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}
	require.Eventually(t, func() bool { return state("ses_a") == domain.RunActive && state("ses_b") == domain.RunWaiting }, 3*time.Second, 20*time.Millisecond)

	// ses_a finishes its turn → idle, usage recorded.
	fake.set(func() { fake.usage["ses_a"] = adapters.SessionResult{Cost: 0.25, TokensIn: 100, TokensOut: 40} })
	ch <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_a", Outcome: "succeeded"}
	require.Eventually(t, func() bool { return state("ses_a") == domain.RunIdle }, 3*time.Second, 20*time.Millisecond)
	s, _ := sessions.Get(ctx, "ses_a")
	assert.InDelta(t, 0.25, s.Cost, 1e-9)
	assert.Equal(t, int64(100), s.TokensIn)

	// ses_b decision answered while executing → active.
	fake.set(func() { fake.pending["ses_b"] = 0 })
	ch <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_b"}
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionReplied, SessionID: "ses_b"}
	require.Eventually(t, func() bool { return state("ses_b") == domain.RunActive }, 3*time.Second, 20*time.Millisecond)

	// New question → waiting.
	fake.set(func() { fake.pending["ses_b"] = 1 })
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionAsked, SessionID: "ses_b"}
	require.Eventually(t, func() bool { return state("ses_b") == domain.RunWaiting }, 3*time.Second, 20*time.Millisecond)

	// Sessions of other groups or unknown to oh are ignored.
	ch <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_other"}
	ch <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_unknown"}
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, domain.RunActive, state("ses_other"))

	// Stream drop → reconnection with a new resync.
	close(ch)
	select {
	case ch = <-fake.events:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not reconnect")
	}
	fake.set(func() { fake.pending["ses_b"] = 0; fake.active["ses_b"] = false })
	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}
	require.Eventually(t, func() bool { return state("ses_b") == domain.RunIdle }, 3*time.Second, 20*time.Millisecond)
}
