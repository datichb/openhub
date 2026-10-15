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

type batchSessions struct {
	fakeSessions
	bmu     sync.Mutex
	scope   []string
	items   []BatchDecision
	applied []BatchChoice
}

func (b *batchSessions) BatchItems(_ context.Context, id string) ([]BatchDecision, error) {
	b.bmu.Lock()
	defer b.bmu.Unlock()
	b.scope = append(b.scope, id)
	return b.items, nil
}

func (b *batchSessions) ApplyBatch(_ context.Context, choices []BatchChoice) ([]BatchResult, error) {
	b.bmu.Lock()
	defer b.bmu.Unlock()
	b.applied = append(b.applied, choices...)
	var out []BatchResult
	for _, c := range choices {
		var d BatchDecision
		for _, it := range b.items {
			if it.ID == c.ID {
				d = it
			}
		}
		r := BatchResult{BatchDecision: d, Action: c.Action}
		if c.ID == "checkpoint:ses_c2:1" && c.Action == BatchApprove {
			r.Error = "verification failed"
		}
		out = append(out, r)
	}
	return out, nil
}

func (b *batchSessions) state() ([]string, []BatchChoice) {
	b.bmu.Lock()
	defer b.bmu.Unlock()
	return append([]string(nil), b.scope...), append([]BatchChoice(nil), b.applied...)
}

type batchShell struct {
	cardShell
	cmu     sync.Mutex
	confirm func()
	ctitle  string
	cdetail string
	toasts  []string
}

func (s *batchShell) Confirm(title, detail string, _ bool, onYes func()) {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	s.ctitle, s.cdetail, s.confirm = title, detail, onYes
}
func (s *batchShell) ShowToastMsg(msg string, _ bool) {
	s.cmu.Lock()
	defer s.cmu.Unlock()
	s.toasts = append(s.toasts, msg)
}

// dialogShell answers the prompts of the view: the next answer of the
// input modals, the value chosen in the select modals.
type dialogShell struct {
	batchShell
	dmu     sync.Mutex
	inputs  []string // titles asked
	answers []string
	selects []string
	choose  string
	danger  bool
}

func (s *dialogShell) ShowInputModal(title, _ string, done func(string)) {
	s.dmu.Lock()
	s.inputs = append(s.inputs, title)
	var a string
	if len(s.answers) > 0 {
		a, s.answers = s.answers[0], s.answers[1:]
	}
	s.dmu.Unlock()
	done(a)
}

func (s *dialogShell) ShowSelectModal(title string, _ []SelectOption, _ string, done func(string)) {
	s.dmu.Lock()
	s.selects = append(s.selects, title)
	c := s.choose
	s.dmu.Unlock()
	done(c)
}

func (s *dialogShell) Confirm(title, detail string, danger bool, onYes func()) {
	s.dmu.Lock()
	s.danger = danger
	s.dmu.Unlock()
	s.batchShell.Confirm(title, detail, danger, onYes)
}

func (s *dialogShell) answer(a ...string) {
	s.dmu.Lock()
	defer s.dmu.Unlock()
	s.answers = append(s.answers, a...)
}

// harness mounts the view in a running application; onLoop runs on its
// event loop.
func harness(t *testing.T, v *SessionsView, rows []SessionRow) func(func()) {
	t.Helper()
	sim := tcell.NewSimulationScreen("")
	require.NoError(t, sim.Init())
	sim.SetSize(140, 40)
	app := tview.NewApplication().SetScreen(sim)
	content := tview.NewFlex()
	app.SetRoot(content, true)
	done := make(chan struct{})
	go func() { _ = app.Run(); close(done) }()
	onLoop := func(fn func()) {
		ch := make(chan struct{})
		app.QueueUpdateDraw(func() { fn(); close(ch) })
		<-ch
	}
	t.Cleanup(func() { onLoop(func() { v.Unmount() }); app.Stop(); <-done })
	onLoop(func() { v.Mount(content, app); v.rows = rows; v.render() })
	return onLoop
}

// press sends a key to the view, then to the focused list of the batch
// (the table of the list gets the keys the view returns).
func pressBatch(v *SessionsView, ev *tcell.EventKey) {
	if v.HandleKey(ev) != nil && v.batch != nil {
		v.batch.list.HandleKey(ev)
	}
}

func batchRows() []SessionRow {
	now := time.Now()
	return []SessionRow{
		{ID: "ses_parent", Workflow: "implement-spec", Agent: "orchestrator-dev", State: "waiting", Started: now, FanoutRole: FanoutRoleParent,
			Decisions: []SessionDecision{{ID: "checkpoint:ses_parent:1", SessionID: "ses_parent", Kind: DecisionKindCheckpoint, Summary: "cp-merge", Created: now}}},
		{ID: "ses_c1", Workflow: "ticket", Agent: "developer", State: "waiting", Started: now, FanoutRole: FanoutRoleChild, FanoutParent: "ses_parent",
			Decisions: []SessionDecision{{ID: "checkpoint:ses_c1:1", SessionID: "ses_c1", Kind: DecisionKindCheckpoint, Summary: "cp-seams", Created: now}}},
		{ID: "ses_c2", Workflow: "ticket", Agent: "developer", State: "waiting", Started: now, FanoutRole: FanoutRoleChild, FanoutParent: "ses_parent",
			Decisions: []SessionDecision{{ID: "checkpoint:ses_c2:1", SessionID: "ses_c2", Kind: DecisionKindCheckpoint, Summary: "cp-seams", Created: now}}},
	}
}

// R6, CP-R6: B shows the checkpoints of the fan-out of the selection in
// place of the list, filtered by checkpoint (/, as --checkpoint); a failed
// verification is not validated in the batch (Enter opens its card); the
// choices are y/s/n (refusal with its instruction); C confirms; only the
// lines of the filter are answered, and the answer of each is shown.
func TestSessionsBatch(t *testing.T) {
	frLocale(t)
	be := &batchSessions{fakeSessions: fakeSessions{rows: batchRows()}, items: []BatchDecision{
		{ID: "checkpoint:ses_parent:1", SessionID: "ses_parent", Checkpoint: "cp-merge", Summary: "2 enfants mergés", Verify: "pass"},
		{ID: "checkpoint:ses_c1:1", SessionID: "ses_c1", Checkpoint: "cp-seams", Summary: "points d'injection", Verify: "pass",
			Approver: "Grace Hopper", Identity: "Ada", ApproverMismatch: true},
		{ID: "checkpoint:ses_c2:1", SessionID: "ses_c2", Checkpoint: "cp-seams", Summary: "tests prévus", Verify: "fail"},
	}}
	sh := &dialogShell{}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	onLoop := harness(t, v, be.rows)
	onLoop(func() {
		v.selectKey("d:checkpoint:ses_c1:1")
		v.HandleKey(key('B'))
	})
	require.Eventually(t, func() bool { var ok bool; onLoop(func() { ok = v.batch != nil }); return ok }, 3*time.Second, 10*time.Millisecond)
	scope, _ := be.state()
	assert.Equal(t, []string{"ses_c1"}, scope, "the fan-out of the selection")
	onLoop(func() {
		assert.True(t, v.CapturesInput(), "Esc closes the batch")
		assert.Equal(t, "cp-seams", v.batch.filter, "the checkpoint of the decision selected")
		assert.Equal(t, 2, v.batch.list.Len())
		// The failed verification is left waiting and is not validated.
		v.batch.list.Select("checkpoint:ses_c2:1")
		pressBatch(v, key('y'))
		assert.Equal(t, BatchKeep, v.batch.choice["checkpoint:ses_c2:1"].Action)
		assert.NotEmpty(t, sh.toasts)
		// n asks the instruction.
		v.batch.list.Select("checkpoint:ses_c1:1")
	})
	sh.answer("relancer les tests")
	onLoop(func() {
		pressBatch(v, key('n'))
		assert.Equal(t, BatchChoice{ID: "checkpoint:ses_c1:1", Action: BatchReject, Message: "relancer les tests"}, v.batch.choice["checkpoint:ses_c1:1"])
		assertGolden(t, "sessions_batch_filtered", screenText(t, v.batch.root, 110, 16))
	})

	// / every checkpoint, then back to cp-seams.
	onLoop(func() {
		v.HandleKey(key('/'))
		assert.Equal(t, 3, v.batch.list.Len())
		sh.choose = "cp-seams"
		v.HandleKey(key('/'))
		assert.Equal(t, 2, v.batch.list.Len())
	})

	// Enter on the failed verification: its card.
	onLoop(func() {
		v.batch.list.Select("checkpoint:ses_c2:1")
		pressBatch(v, tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	})
	title, _, _ := sh.modal()
	assert.Contains(t, title, "cp-seams")

	// C: confirmation, then only the lines of the filter are answered.
	onLoop(func() { v.HandleKey(key('C')) })
	sh.cmu.Lock()
	confirm, ctitle, detail := sh.confirm, sh.ctitle, sh.cdetail
	sh.cmu.Unlock()
	require.NotNil(t, confirm)
	_, applied := be.state()
	assert.Empty(t, applied, "nothing answered before the confirmation")
	assert.Contains(t, ctitle, "refuser 1")
	assert.Contains(t, detail, "relancer les tests")
	onLoop(confirm)
	require.Eventually(t, func() bool { _, a := be.state(); return len(a) == 2 }, 3*time.Second, 10*time.Millisecond)
	_, applied = be.state()
	assert.Equal(t, BatchReject, applied[0].Action)
	assert.Equal(t, BatchKeep, applied[1].Action)
	require.Eventually(t, func() bool { title, _, _ := sh.modal(); return strings.Contains(title, "Lot") }, 3*time.Second, 10*time.Millisecond)
	onLoop(func() {
		assert.Nil(t, v.batch, "the list is back")
		assert.False(t, v.CapturesInput())
	})
}

// Esc closes the batch without answering anything.
func TestSessionsBatchCancel(t *testing.T) {
	be := &batchSessions{fakeSessions: fakeSessions{rows: batchRows()}, items: []BatchDecision{
		{ID: "checkpoint:ses_parent:1", SessionID: "ses_parent", Checkpoint: "cp-merge"},
	}}
	sh := &dialogShell{}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	onLoop := harness(t, v, be.rows)
	onLoop(func() { v.selectKey("s:ses_parent"); v.HandleKey(key('B')) })
	require.Eventually(t, func() bool { var ok bool; onLoop(func() { ok = v.batch != nil }); return ok }, 3*time.Second, 10*time.Millisecond)
	onLoop(func() {
		assert.Empty(t, v.batch.filter, "no decision selected: every checkpoint")
		v.HandleKey(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
		assert.Nil(t, v.batch)
	})
	_, applied := be.state()
	assert.Empty(t, applied)
}
