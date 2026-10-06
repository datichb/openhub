package views

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRemote struct {
	*fakeSessions
	mu      sync.Mutex
	fetched []string
	applied map[string]string
	plan    *ReplayView
}

func (f *fakeRemote) FetchRemote(_ context.Context, id string) (RemoteFetchReport, error) {
	f.mu.Lock()
	f.fetched = append(f.fetched, id)
	f.mu.Unlock()
	return RemoteFetchReport{Lines: []string{"✔ artefacts", "✔ session importée"}, Journal: 3}, nil
}

func (f *fakeRemote) PlanReplay(context.Context, string) (*ReplayView, error) { return f.plan, nil }

func (f *fakeRemote) ApplyReplay(_ context.Context, _ string, res map[string]string) (*ReplayView, error) {
	f.mu.Lock()
	f.applied = res
	f.mu.Unlock()
	return &ReplayView{Lines: []string{"✔ done"}}, nil
}

// modalShell records the scrollable modals.
type modalShell struct {
	mockShell
	mu     sync.Mutex
	titles []string
	acts   [][]ModalAction
	toasts []string
}

func (m *modalShell) ShowScrollableModal(title, _ string, actions []ModalAction) {
	m.mu.Lock()
	m.titles = append(m.titles, title)
	m.acts = append(m.acts, actions)
	m.mu.Unlock()
}

func (m *modalShell) ShowToastMsg(msg string, _ bool) {
	m.mu.Lock()
	m.toasts = append(m.toasts, msg)
	m.mu.Unlock()
}

func (m *modalShell) last() (string, []ModalAction, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.titles) == 0 {
		return "", nil, 0
	}
	return m.titles[len(m.titles)-1], m.acts[len(m.acts)-1], len(m.titles)
}

func remoteRows() []SessionRow {
	now := time.Now()
	return []SessionRow{
		{ID: "ses_r1", Project: "api", Workflow: "ticket", State: "completed", Finished: true, Changed: now, Remote: RemoteStatusReady, RemoteDetail: "pipeline #812 · MR prête"},
		{ID: "ses_r2", Project: "api", Workflow: "ticket", State: "active", Remote: RemoteStatusRunning},
		{ID: "ses_l", Project: "api", State: "active"},
	}
}

func TestRemoteSection(t *testing.T) {
	rest, fetch := splitRemote(remoteRows())
	require.Len(t, fetch, 1)
	assert.Equal(t, "ses_r1", fetch[0].ID)
	assert.Len(t, rest, 2, "running remote sessions stay in « En cours »")
	items := remoteSection(nil, fetch)
	require.Len(t, items, 2)
	assert.True(t, items[0].IsHeader)
	assert.Contains(t, items[1].SecondaryText, "pipeline #812")
}

func TestSessionsFetchAndConflictWindow(t *testing.T) {
	be := &fakeRemote{fakeSessions: &fakeSessions{rows: remoteRows()}, plan: &ReplayView{
		Lines: []string{"○ bd update bd-40 --status done"}, Pending: 2,
		Conflicts: []ReplayConflict{{Ticket: "bd-40", Title: "Export", Fields: []ReplayField{{Field: "status", Local: "in_progress", Remote: "done"}}}},
	}}
	sh := &modalShell{}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	sim := tcell.NewSimulationScreen("")
	require.NoError(t, sim.Init())
	app := tview.NewApplication().SetScreen(sim)
	content := tview.NewFlex()
	app.SetRoot(content, true)
	done := make(chan struct{})
	go func() { _ = app.Run(); close(done) }()
	t.Cleanup(func() { app.Stop(); <-done })
	onLoop := func(fn func()) {
		ch := make(chan struct{})
		app.QueueUpdateDraw(func() { fn(); close(ch) })
		<-ch
	}
	onLoop(func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		n := 0
		onLoop(func() { n = len(v.rows) })
		return n == 3
	}, 3*time.Second, 20*time.Millisecond)

	// Not finished: g refuses.
	onLoop(func() {
		v.Focus("ses_r2")
		v.render()
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'g', 0))
	})
	assert.Empty(t, be.fetched)

	// g fetches, then the report offers the replay.
	onLoop(func() {
		v.Focus("ses_r1")
		v.render()
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'g', 0))
	})
	require.Eventually(t, func() bool { _, _, n := sh.last(); return n == 1 }, 3*time.Second, 20*time.Millisecond)
	title, acts, _ := sh.last()
	assert.Contains(t, title, "ticket")
	assert.Equal(t, []string{"ses_r1"}, be.fetched)
	require.True(t, strings.Contains(acts[0].Label, "3"), acts[0].Label)

	// Replay → conflict window → « Fusionner les notes » → confirmation → applied.
	onLoop(func() { acts[0].Callback() })
	require.Eventually(t, func() bool { _, _, n := sh.last(); return n == 2 }, 3*time.Second, 20*time.Millisecond)
	title, acts, _ = sh.last()
	assert.Contains(t, title, "bd-40")
	require.Len(t, acts, 4, "keep local, apply remote, merge notes, later")
	onLoop(func() { acts[2].Callback() })
	_, acts, n := sh.last()
	require.Equal(t, 3, n, "confirmation of the replay")
	be.mu.Lock()
	assert.Nil(t, be.applied, "nothing applied before confirmation")
	be.mu.Unlock()
	onLoop(func() { acts[0].Callback() })
	require.Eventually(t, func() bool {
		be.mu.Lock()
		defer be.mu.Unlock()
		return be.applied != nil
	}, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, map[string]string{"bd-40": ResolutionMergeNotes}, be.applied)
	onLoop(func() { v.Unmount() })
}
