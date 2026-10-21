package views

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

type reportSessions struct {
	cardSessions
	read []string
}

func (r *reportSessions) CheckpointReport(_ context.Context, sessionID, id string) (string, string, error) {
	r.read = append(r.read, sessionID+"/"+id)
	return "Retour du reviewer", "## Verdict [red]corriger[-]\n" + "ligne\n", nil
}

func TestCheckpointCardReports(t *testing.T) {
	withFrench(t)
	at := time.Date(2026, 10, 13, 10, 4, 0, 0, time.Local)
	be := &reportSessions{cardSessions: cardSessions{fakeSessions: fakeSessions{rows: checkpointRows()}, card: CheckpointCardView{
		Checkpoint: "cp-2", Label: "Commit", Summary: "prêt",
		Reports: []CheckpointReport{{ID: "rp-1", Agent: "developer", Title: "Fait [::b]gras", Bytes: 820, At: at},
			{ID: "rp-2", Agent: "reviewer", Title: "Review", Bytes: 2150, At: at}},
	}}}
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
	onLoop(func() {
		v.selectKey(refKey(decisionRef{id: "checkpoint:ses_a:per_2", session: "ses_a"}))
		v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	})
	require.Eventually(t, func() bool { title, _, _ := sh.modal(); return title != "" }, 3*time.Second, 20*time.Millisecond)
	title, body, actions := sh.modal()
	golden.Assert(t, "checkpoint_card/reports", compactCardText(t, title, body, actions))
	require.Len(t, actions, 4, "decide, read rp-1, read rp-2, attach")
	assert.Equal(t, "Lire rp-2", actions[2].Label)

	// The reader shows the escaped report.
	actions[2].Callback()
	require.Eventually(t, func() bool { title, _, _ := sh.modal(); return title != "" && title[:4] == "rp-2" }, 3*time.Second, 20*time.Millisecond)
	title, body, _ = sh.modal()
	assert.Equal(t, "rp-2 · reviewer · Retour du reviewer", title)
	golden.Assert(t, "checkpoint_card/report_reader", compactCardText(t, title, body, nil))
	assert.Equal(t, []string{"ses_a/rp-2"}, be.read)
	onLoop(func() { v.Unmount() })
}

func TestCheckpointCardWithoutReader(t *testing.T) {
	t.Parallel()
	v := NewSessionsView(SessionsViewConfig{Backend: &cardSessions{}})
	assert.Nil(t, v.reportActions("ses_a", "d", []CheckpointReport{{ID: "rp-1"}}), "listed only")
	body := checkpointBody(CheckpointCardView{Reports: []CheckpointReport{{ID: "rp-1", Bytes: 10}}})
	assert.Contains(t, body, "rp-1 · ?")
}

// Order on the card: verification, evidence, then reports.
func TestCheckpointCardOrderVerifyEvidenceReports(t *testing.T) {
	t.Parallel()
	body := checkpointBody(CheckpointCardView{
		Verify:  CheckpointVerifyView{Status: verifyPass, Summary: "go test ./... ok", Evidence: []CheckpointEvidenceView{{Cmd: "go test ./...", Observed: true}}},
		Reports: []CheckpointReport{{ID: "rp-1", Agent: "reviewer", Title: "Review"}},
	})
	verify, evidence, reports := strings.Index(body, "go test ./... ok"), strings.LastIndex(body, "go test ./..."), strings.Index(body, "rp-1")
	assert.True(t, verify >= 0 && evidence > verify && reports > evidence, body)
}
