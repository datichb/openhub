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

type cardSessions struct {
	fakeSessions
	card CheckpointCardView
}

func (c *cardSessions) CheckpointCard(context.Context, string) (CheckpointCardView, error) {
	return c.card, nil
}

type cardShell struct {
	mockShell
	mu      sync.Mutex
	title   string
	body    string
	actions []ModalAction
	form    *InlineFormConfig
}

func (s *cardShell) ShowScrollableModal(title, body string, actions []ModalAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.title, s.body, s.actions = title, body, actions
}

func (s *cardShell) ShowInlineForm(cfg InlineFormConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.form = &cfg
}

func (s *cardShell) modal() (string, string, []ModalAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.title, s.body, s.actions
}

func checkpointRows() []SessionRow {
	now := time.Now()
	return []SessionRow{{ID: "ses_a", Project: "openhub", Workflow: "ticket", Agent: "orchestrator-dev", State: "waiting", StateIcon: "⏸", Started: now,
		Timeline:  "✔ cp-1 10:03 → developer (3) → ⏸ cp-2",
		Decisions: []SessionDecision{{ID: "checkpoint:ses_a:per_2", SessionID: "ses_a", Kind: DecisionKindCheckpoint, Icon: "⏸", Summary: "cp-2 « Commit »", Created: now}}}}
}

func TestCheckpointCard(t *testing.T) {
	be := &cardSessions{fakeSessions: fakeSessions{rows: checkpointRows()}, card: CheckpointCardView{
		Checkpoint: "cp-2", Label: "Commit", Summary: "0 bloquant · 2 mineurs",
		Files:     []CheckpointFile{{File: "src/export/csv.go", Status: "modified", Additions: 140, Deletions: 18}},
		Additions: 140, Deletions: 18, Patch: "diff --git a/x b/x",
		Messages: []string{"reviewer › « 2 remarques mineures »"}, Timeline: "✔ cp-1 → ⏸ cp-2",
	}}
	sh := &cardShell{}
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
	onLoop(func() { v.Mount(content, app); v.rows = checkpointRows(); v.render() })

	// Detail: the timeline of the session.
	onLoop(func() {
		v.Focus("ses_a")
		v.renderDetail(v.row("ses_a"))
		assert.Contains(t, v.detail.GetText(true), "developer (3)")
	})

	// Enter on the checkpoint: card with diff, messages and timeline.
	onLoop(func() {
		v.selectKey(refKey(decisionRef{id: "checkpoint:ses_a:per_2", session: "ses_a"}))
		v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	})
	require.Eventually(t, func() bool { title, _, _ := sh.modal(); return title != "" }, 3*time.Second, 20*time.Millisecond)
	title, body, actions := sh.modal()
	assert.Contains(t, title, "cp-2")
	assert.Contains(t, title, "Commit")
	for _, want := range []string{"0 bloquant", "+140 −18", "src/export/csv.go", "2 remarques mineures", "cp-1"} {
		assert.Contains(t, body, want)
	}
	require.Len(t, actions, 3, "decide, full diff, attach")

	// Decide → form; "fix" without a message is refused, with one it is sent.
	actions[0].Callback()
	sh.mu.Lock()
	form := sh.form
	sh.mu.Unlock()
	require.NotNil(t, form)
	assert.Equal(t, checkpointApprove, form.Fields[0].Default)
	form.OnSubmit(map[string]string{"decision": checkpointFix, "message": " "}, nil)
	form.OnSubmit(map[string]string{"decision": checkpointFix, "message": "nommage"}, nil)
	require.Eventually(t, func() bool { return len(be.decisions()) == 1 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "checkpoint:ses_a:per_2=fix:nommage", be.decisions()[0])

	// y validates the selected checkpoint directly.
	onLoop(func() {
		v.selectKey(refKey(decisionRef{id: "checkpoint:ses_a:per_2", session: "ses_a"}))
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'y', 0))
	})
	require.Eventually(t, func() bool { return len(be.decisions()) == 2 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "checkpoint:ses_a:per_2=approve", be.decisions()[1])
	onLoop(func() { v.Unmount() })
}

func TestCheckpointBodyWithoutData(t *testing.T) {
	body := checkpointBody(CheckpointCardView{})
	assert.True(t, strings.Contains(body, "─"))
	assert.NotContains(t, body, "+0")
}
