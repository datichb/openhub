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
	for i, c := range choices {
		r := BatchResult{BatchDecision: b.items[i], Action: c.Action}
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

// R6, CP-R6: B lists every checkpoint waiting in the fan-out of the
// selection, each one validated, left waiting or refused with its
// instruction; nothing is answered before the confirmation, and the
// answer of each one is shown.
func TestSessionsBatch(t *testing.T) {
	now := time.Now()
	be := &batchSessions{fakeSessions: fakeSessions{rows: []SessionRow{
		{ID: "ses_parent", Workflow: "implement-spec", Agent: "orchestrator-dev", State: "waiting", Started: now,
			Decisions: []SessionDecision{{ID: "checkpoint:ses_parent:1", SessionID: "ses_parent", Kind: DecisionKindCheckpoint, Summary: "cp-merge", Created: now}}},
		{ID: "ses_c1", Workflow: "ticket", Agent: "orchestrator-dev", State: "waiting", Started: now},
		{ID: "ses_c2", Workflow: "ticket", Agent: "orchestrator-dev", State: "waiting", Started: now},
	}}, items: []BatchDecision{
		{ID: "checkpoint:ses_parent:1", SessionID: "ses_parent", Checkpoint: "cp-merge", Summary: "2 enfants mergés", Verify: "pass"},
		{ID: "checkpoint:ses_c1:1", SessionID: "ses_c1", Checkpoint: "cp-seams", Summary: "points d'injection"},
		{ID: "checkpoint:ses_c2:1", SessionID: "ses_c2", Checkpoint: "cp-seams", Summary: "tests prévus"},
	}}
	sh := &batchShell{}
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
	onLoop(func() { v.Mount(content, app); v.rows = be.rows; v.render() })
	onLoop(func() {
		v.selectKey("s:ses_c1")
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'B', 0))
	})
	var form *InlineFormConfig
	require.Eventually(t, func() bool {
		sh.mu.Lock()
		defer sh.mu.Unlock()
		form = sh.form
		return form != nil
	}, 3*time.Second, 10*time.Millisecond)
	scope, _ := be.state()
	assert.Equal(t, []string{"ses_c1"}, scope, "the fan-out of the selection")
	require.Len(t, form.Fields, 6, "a choice and an instruction per checkpoint")
	assert.Contains(t, form.Fields[0].Label, "cp-merge")
	assert.Contains(t, form.Fields[0].Label, "implement-spec")
	assert.Contains(t, form.Fields[0].Label, "pass")

	// A refusal without its instruction is not submitted.
	onLoop(func() {
		form.OnSubmit(map[string]string{"a:checkpoint:ses_parent:1": BatchReject}, nil)
	})
	sh.cmu.Lock()
	assert.Nil(t, sh.confirm)
	assert.NotEmpty(t, sh.toasts)
	sh.cmu.Unlock()

	onLoop(func() {
		form.OnSubmit(map[string]string{
			"a:checkpoint:ses_parent:1": BatchReject, "m:checkpoint:ses_parent:1": "relancer les tests",
			"a:checkpoint:ses_c1:1": BatchKeep,
			"a:checkpoint:ses_c2:1": BatchApprove,
		}, nil)
	})
	sh.cmu.Lock()
	confirm, title, detail := sh.confirm, sh.ctitle, sh.cdetail
	sh.cmu.Unlock()
	require.NotNil(t, confirm, "confirmation first")
	_, applied := be.state()
	assert.Empty(t, applied, "nothing answered before the confirmation")
	assert.Contains(t, title, "1")
	assert.Contains(t, detail, "relancer les tests")

	onLoop(confirm)
	require.Eventually(t, func() bool { _, a := be.state(); return len(a) == 3 }, 3*time.Second, 10*time.Millisecond)
	_, applied = be.state()
	assert.Equal(t, BatchChoice{ID: "checkpoint:ses_parent:1", Action: BatchReject, Message: "relancer les tests"}, applied[0])
	assert.Equal(t, BatchKeep, applied[1].Action)
	assert.Equal(t, BatchApprove, applied[2].Action)
	require.Eventually(t, func() bool {
		title, body, _ := sh.modal()
		return title != "" && strings.Contains(body, "verification failed")
	}, 3*time.Second, 10*time.Millisecond)
	title, _, _ = sh.modal()
	assert.Contains(t, title, "2")
	onLoop(func() { v.Unmount() })
}

func TestBatchChoices(t *testing.T) {
	items := []BatchDecision{{ID: "d1", Checkpoint: "cp-2"}, {ID: "d2"}}
	got, err := batchChoices(items, map[string]string{"a:d1": "weird"})
	assert.Empty(t, err)
	assert.Equal(t, BatchKeep, got[0].Action, "an unknown choice answers nothing")
	assert.Equal(t, BatchApprove, got[1].Action, "validated by default")
	_, err = batchChoices(items, map[string]string{"a:d1": BatchReject, "m:d1": "  "})
	assert.Contains(t, err, "cp-2")
}
