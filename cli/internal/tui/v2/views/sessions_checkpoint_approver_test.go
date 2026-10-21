package views

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

// CP-R9: the card names the expected approver and warns when the person
// who validates is none of them; y asks for a confirmation then; the batch
// warns item by item. Never refused.
func TestCheckpointApproverCard(t *testing.T) {
	frLocale(t)
	mismatch := CheckpointApproverView{Expected: "Grace Hopper", Identity: "Ada Lovelace <ada@example.com>", Mismatch: true}
	for name, approver := range map[string]CheckpointApproverView{
		"mismatch":    mismatch,
		"expected":    {Expected: "Ada Lovelace", Identity: "Ada Lovelace"},
		"no_approver": {Identity: "Ada"},
	} {
		golden.Assert(t, "checkpoint_card/approver_"+name, cardBodyText(t, CheckpointCardView{Approver: approver}))
	}

	rows := checkpointRows()
	rows[0].Decisions[0].Approver = "Grace Hopper"
	be := &cardSessions{fakeSessions: fakeSessions{rows: rows}, card: CheckpointCardView{Checkpoint: "cp-2", Label: "Commit", Approver: mismatch}}
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
	onLoop(func() { v.Mount(content, app); v.rows = rows; v.render() })

	// y: confirmation with the warning, then validated.
	onLoop(func() {
		v.selectKey(refKey(decisionRef{id: "checkpoint:ses_a:per_2", session: "ses_a"}))
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'y', 0))
	})
	require.Eventually(t, func() bool { sh.cmu.Lock(); defer sh.cmu.Unlock(); return sh.confirm != nil }, 3*time.Second, 10*time.Millisecond)
	assert.Empty(t, be.decisions(), "asked first")
	sh.cmu.Lock()
	confirm, title, detail := sh.confirm, sh.ctitle, sh.cdetail
	sh.cmu.Unlock()
	assert.Contains(t, title, "cp-2", "confirmation title names the checkpoint")
	assert.Contains(t, detail, "Grace Hopper", "confirmation names the expected approver")
	assert.Contains(t, detail, "Ada Lovelace", "confirmation names the identity")
	onLoop(confirm)
	require.Eventually(t, func() bool { return len(be.decisions()) == 1 }, 3*time.Second, 10*time.Millisecond)
	assert.Equal(t, "checkpoint:ses_a:per_2=approve", be.decisions()[0])

	// The decision form says it too.
	onLoop(func() { v.checkpointForm(v.row("ses_a"), &rows[0].Decisions[0], mismatch, checkpointApprove) })
	sh.mu.Lock()
	form := sh.form
	sh.mu.Unlock()
	require.NotNil(t, form)
	assert.Contains(t, form.Title, "Grace Hopper")
	onLoop(func() { v.Unmount() })
}

func TestBatchLabelApprover(t *testing.T) {
	v := NewSessionsView(SessionsViewConfig{Backend: &fakeSessions{}})
	long := BatchDecision{ID: "d", SessionID: "ses_x", Checkpoint: "cp-2", Summary: "un résumé assez long pour remplir toute la ligne de la décision du lot, et plus encore",
		Approver: "Grace Hopper", Identity: "Ada", ApproverMismatch: true}
	label := v.batchLabel(long)
	assert.Contains(t, label, "Grace Hopper", "never clipped away")
	assert.Contains(t, label, "⚠")
	long.ApproverMismatch = false
	assert.NotContains(t, v.batchLabel(long), "Grace Hopper")
}
