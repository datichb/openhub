package views

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wikiSessions struct {
	cardSessions
	mu      sync.Mutex
	decided []string
}

func (w *wikiSessions) CheckpointWikiDecide(_ context.Context, sessionID, id, choice, content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.decided = append(w.decided, sessionID+"/"+id+"/"+choice+"/"+content)
	return nil
}

func (w *wikiSessions) got() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.decided...)
}

// LD-J: the proposals of the session on the card of the commit checkpoint,
// escaped, accepted, edited or refused from it.
func TestCheckpointCardWiki(t *testing.T) {
	withFrench(t)
	be := &wikiSessions{cardSessions: cardSessions{fakeSessions: fakeSessions{rows: checkpointRows()}, card: CheckpointCardView{
		Checkpoint: "cp-2", Label: "Commit", Summary: "prêt",
		Wiki: []CheckpointWikiProposal{
			{ID: "wp-1", Agent: "reviewer", Page: "technical/conventions.md", Section: "Nommage", Tag: "DÉDUIT", Status: "pending", Content: "kebab-case [red]x[-]\nligne 2\nligne 3"},
			{ID: "wp-2", Agent: "developer", Page: "index.md", Tag: "CONFIRMÉ", Status: "accepted", Content: "zod"},
		},
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
	_, body, actions := sh.modal()
	assert.Contains(t, body, "Propositions du wiki (2)")
	assert.Contains(t, body, "wp-1 · reviewer · technical/conventions.md § Nommage · DÉDUIT")
	assert.Contains(t, body, "kebab-case [red[]x[-[]", "agent text escaped")
	assert.NotContains(t, body, "ligne 3", "two lines at most")
	assert.Contains(t, body, "acceptée")
	var labels []string
	for _, a := range actions {
		labels = append(labels, a.Label)
	}
	assert.Equal(t, []string{"Décider", "Accepter les 2 propositions du wiki", "Wiki wp-1", "Wiki wp-2", "Attacher"}, labels)

	// Accept all: only the pending one is asked again.
	onLoop(func() { actions[1].Callback() })
	require.Eventually(t, func() bool { return len(be.got()) == 1 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, []string{"ses_a/wp-1/accept/"}, be.got())

	// One proposal: an edited content is an edit, line breaks restored.
	onLoop(func() { actions[2].Callback() })
	sh.mu.Lock()
	form := sh.form
	sh.mu.Unlock()
	require.NotNil(t, form)
	assert.Equal(t, "kebab-case [red]x[-] ⏎ ligne 2 ⏎ ligne 3", form.Fields[1].Default)
	onLoop(func() {
		form.OnSubmit(map[string]string{"choice": wikiAccept, "content": "kebab-case ⏎ sauf hooks"}, nil)
	})
	require.Eventually(t, func() bool { return len(be.got()) == 2 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "ses_a/wp-1/edit/kebab-case\nsauf hooks", be.got()[1])
	onLoop(func() { form.OnSubmit(map[string]string{"choice": wikiRefuse, "content": form.Fields[1].Default}, nil) })
	require.Eventually(t, func() bool { return len(be.got()) == 3 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "ses_a/wp-1/refuse/", be.got()[2])
	onLoop(func() { v.Unmount() })
}

func TestCheckpointCardWikiWithoutDecider(t *testing.T) {
	v := NewSessionsView(SessionsViewConfig{Backend: &cardSessions{}})
	assert.Nil(t, v.wikiActions(nil, &SessionDecision{SessionID: "ses_a"}, []CheckpointWikiProposal{{ID: "wp-1"}}), "listed only")
	assert.NotContains(t, checkpointBody(CheckpointCardView{}), "wiki")
}
