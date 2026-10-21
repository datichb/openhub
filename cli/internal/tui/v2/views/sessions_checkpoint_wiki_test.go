package views

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/testutil/golden"
)

type wikiSessions struct {
	cardSessions
	mu      sync.Mutex
	decided []string
	failOn  string
}

func (w *wikiSessions) CheckpointWikiDecide(_ context.Context, sessionID, id, choice, content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if id == w.failOn {
		return errors.New("base verrouillée")
	}
	w.decided = append(w.decided, sessionID+"/"+id+"/"+choice+"/"+content)
	return nil
}

func (w *wikiSessions) got() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.decided...)
}

type wikiEditSessions struct {
	wikiSessions
	edits  []string
	result string
	err    error
}

func (w *wikiEditSessions) CheckpointWikiEdit(name, content string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.edits = append(w.edits, name+"|"+content)
	return w.result, w.err
}

// wikiCardShell records what a card and its list of proposals show: the
// pushed view, the instruction prompts, the forms and the toasts.
type wikiCardShell struct {
	cardShell
	pushed  View
	popped  int
	inputs  []string
	answer  func(string)
	toasts  []string
	toastMu sync.Mutex
}

func (s *wikiCardShell) PushView(v View) {
	if sv, ok := v.(interface{ SetShell(ShellAccess) }); ok {
		sv.SetShell(s)
	}
	s.pushed = v
}

func (s *wikiCardShell) PopView() bool { s.popped++; return true }

func (s *wikiCardShell) ShowInputModal(title, _ string, fn func(string)) {
	s.inputs, s.answer = append(s.inputs, title), fn
}

func (s *wikiCardShell) ShowToastMsg(msg string, _ bool) {
	s.toastMu.Lock()
	defer s.toastMu.Unlock()
	s.toasts = append(s.toasts, msg)
}

func (s *wikiCardShell) toastList() []string {
	s.toastMu.Lock()
	defer s.toastMu.Unlock()
	return append([]string(nil), s.toasts...)
}

// runApp runs an application on a simulation screen; onLoop runs a
// function on its event loop.
func runWikiApp(t *testing.T) (*tview.Application, *tview.Flex, func(func())) {
	t.Helper()
	sim := tcell.NewSimulationScreen("")
	require.NoError(t, sim.Init())
	sim.SetSize(120, 40)
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
	return app, content, onLoop
}

// nineProposals are the proposals of the sketch of the card (§3).
func nineProposals() []CheckpointWikiProposal {
	list := []CheckpointWikiProposal{
		{ID: "wp-1", Agent: "onboarder", Page: "technical/stack.md", Tag: "CONFIRMÉ", Status: "accepted", Content: "- Go 1.24"},
		{ID: "wp-2", Agent: "onboarder", Page: "technical/stack.md", Section: "Paquets", Tag: "PROBABLE", Status: "pending",
			Replaces: "- `internal/store` : stockage SQLite", Content: "- `internal/storage` : stockage SQLite (renommé, commit 3f2a1c)",
			Source: "internal/storage/store.go:1"},
		{ID: "wp-3", Agent: "onboarder", Page: "business/index.md", Tag: "PROBABLE", Status: "pending", Content: "Facturation [red]B2B[-]"},
		{ID: "wp-4", Agent: "onboarder", Page: "technical/tests.md", Tag: "CONFIRMÉ", Status: "pending", Replaces: "- tests lents", Content: "- tests rapides"},
	}
	for i := 5; i <= 9; i++ {
		list = append(list, CheckpointWikiProposal{ID: fmt.Sprintf("wp-%d", i), Agent: "onboarder", Page: "technical/conventions.md",
			Tag: "PROBABLE", Status: "pending", Content: fmt.Sprintf("règle %d\nsuite", i)})
	}
	return list
}

// LD-J, tui-wiki-bricks: the card lists the proposals on one line each
// and opens them all in a list (no limit).
func TestCheckpointCardWiki(t *testing.T) {
	withFrench(t)
	be := &wikiSessions{cardSessions: cardSessions{fakeSessions: fakeSessions{rows: checkpointRows()}, card: CheckpointCardView{
		Checkpoint: "cp-2", Label: "Commit", Summary: "prêt", Wiki: nineProposals(),
	}}}
	sh := &wikiCardShell{}
	v := NewSessionsView(SessionsViewConfig{Backend: be})
	v.SetShell(sh)
	app, content, onLoop := runWikiApp(t)
	onLoop(func() { v.Mount(content, app); v.rows = checkpointRows(); v.render() })
	onLoop(func() {
		v.selectKey(refKey(decisionRef{id: "checkpoint:ses_a:per_2", session: "ses_a"}))
		v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	})
	require.Eventually(t, func() bool { title, _, _ := sh.modal(); return title != "" }, 3*time.Second, 20*time.Millisecond)
	_, body, actions := sh.modal()
	assert.Contains(t, body, "Propositions du wiki (9)")
	assert.Contains(t, body, "wp-2 · onboarder · technical/stack.md § Paquets · remplace · PROBABLE")
	assert.Contains(t, body, "wp-9 · onboarder · technical/conventions.md · ajout · PROBABLE", "every proposal, no limit")
	assert.Contains(t, body, "1 acceptée(s), 8 à décider")
	assert.NotContains(t, body, "Facturation", "contents are in the list")
	var labels []string
	for _, a := range actions {
		labels = append(labels, a.Label)
	}
	assert.Equal(t, []string{"Décider", "Propositions du wiki (9) : décider…", "Attacher"}, labels)

	onLoop(func() { actions[1].Callback() })
	cw, ok := sh.pushed.(*CheckpointWikiView)
	require.True(t, ok, "the list is pushed over Sessions")
	assert.Len(t, cw.wiki.list.Items(), 9)
	assert.Equal(t, "y accepter · n refuser · e modifier · s laisser · A tout accepter · Entrée décider le checkpoint · Échap revenir à la fiche",
		cw.StatusHints())
	onLoop(func() { v.Unmount() })
}

func TestCheckpointCardWikiWithoutDecider(t *testing.T) {
	v := NewSessionsView(SessionsViewConfig{Backend: &cardSessions{}})
	assert.Nil(t, v.wikiActions(nil, &SessionDecision{SessionID: "ses_a"}, []CheckpointWikiProposal{{ID: "wp-1"}}), "listed only")
	assert.NotContains(t, checkpointBody(CheckpointCardView{}), "wiki")
}

func newTestCheckpointWiki(t *testing.T, be CheckpointWikiDecider, backend SessionsBackend, sh *wikiCardShell) (*SessionsView, *CheckpointWikiView) {
	t.Helper()
	v := NewSessionsView(SessionsViewConfig{Backend: backend})
	v.SetShell(sh)
	rows := checkpointRows()
	dec := rows[0].Decisions[0]
	cw := newCheckpointWikiView(v, be, &rows[0], &dec, nineProposals())
	cw.SetShell(sh)
	return v, cw
}

// Golden: nine proposals, the before/after of a replacement below.
func TestCheckpointWikiGolden(t *testing.T) {
	withFrench(t)
	be := &wikiSessions{}
	_, cw := newTestCheckpointWiki(t, be, be, &wikiCardShell{})
	cw.wiki.list.Select("wp-2")
	golden.Assert(t, "checkpoint_wiki_9", screenText(t, cw.wiki.list.Primitive(), 90, 18))
	cw.wiki.list.Select("wp-3")
	detail := cw.wiki.list.detail.GetText(true)
	assert.Contains(t, detail, "+ Facturation [red]B2B[-]", "agent text escaped, shown as is")
}

// The decisions of the list: y, n (prompt = confirmation), e (one line or
// $EDITOR), s, A; Enter goes to the decision of the checkpoint, Esc back
// to the card.
func TestCheckpointWikiDecisions(t *testing.T) {
	withFrench(t)
	be := &wikiEditSessions{wikiSessions: wikiSessions{cardSessions: cardSessions{fakeSessions: fakeSessions{rows: checkpointRows()},
		card: CheckpointCardView{Checkpoint: "cp-2", Label: "Commit"}}}}
	sh := &wikiCardShell{}
	v, cw := newTestCheckpointWiki(t, be, be, sh)
	app, content, onLoop := runWikiApp(t)
	onLoop(func() { v.Mount(content, app); v.rows = checkpointRows(); v.render() })
	onLoop(func() { cw.Mount(tview.NewFlex(), app) })
	decided := func(n int) {
		t.Helper()
		require.Eventually(t, func() bool { return len(be.got()) == n }, 3*time.Second, 20*time.Millisecond)
	}
	current := func() (it ActionItem) {
		onLoop(func() { it, _ = cw.wiki.list.Current() })
		return it
	}

	// y on an accepted proposal does not apply; on wp-2 it accepts and
	// moves to the next pending one.
	onLoop(func() { cw.wiki.list.Select("wp-1"); cw.HandleKey(key('y')) })
	assert.Contains(t, sh.toastList(), "« y accepter » ne s'applique pas à wp-1")
	onLoop(func() { cw.wiki.list.Select("wp-2"); cw.HandleKey(key('y')) })
	decided(1)
	assert.Equal(t, "ses_a/wp-2/accept/", be.got()[0])
	require.Eventually(t, func() bool { return current().ID == "wp-3" }, 3*time.Second, 20*time.Millisecond)
	onLoop(func() {})
	p, _ := cw.wiki.prop("wp-2")
	assert.Equal(t, "accepted", p.Status)

	// n asks the reason (optional): validating it refuses.
	onLoop(func() { cw.HandleKey(key('n')) })
	require.Len(t, sh.inputs, 1)
	assert.Equal(t, "Refuser wp-3 (index.md) : motif facultatif, Entrée confirme le refus", sh.inputs[0])
	assert.Len(t, be.got(), 1, "nothing before the prompt is validated")
	onLoop(func() { sh.answer("") })
	decided(2)
	assert.Equal(t, "ses_a/wp-3/refuse/", be.got()[1])
	var row string
	onLoop(func() {
		for _, it := range cw.wiki.list.Items() {
			if it.ID == "wp-3" {
				row = cw.wiki.list.stateText(it)
			}
		}
	})
	assert.Equal(t, "✗ refusée", row, "the refused proposal stays, as such")

	// e: one line, ⏎ for a line break.
	onLoop(func() { cw.wiki.list.Select("wp-5"); cw.HandleKey(key('e')) })
	sh.mu.Lock()
	form := sh.form
	sh.mu.Unlock()
	require.NotNil(t, form)
	require.Len(t, form.Fields, 2, "where, content")
	assert.Equal(t, "règle 5 ⏎ suite", form.Fields[1].Default)
	onLoop(func() { form.OnSubmit(map[string]string{"where": wikiEdit, "content": "règle 5 ⏎ corrigée"}, nil) })
	decided(3)
	assert.Equal(t, "ses_a/wp-5/edit/règle 5\ncorrigée", be.got()[2])
	// Untouched: nothing.
	onLoop(func() { form.OnSubmit(map[string]string{"where": wikiEdit, "content": form.Fields[1].Default}, nil) })
	assert.Contains(t, sh.toastList(), "Contenu inchangé : rien n'est enregistré")
	// $EDITOR from the field.
	be.result = "règle 6\n\nsur deux blocs\n"
	onLoop(func() { cw.wiki.list.Select("wp-6"); cw.HandleKey(key('e')) })
	sh.mu.Lock()
	form = sh.form
	sh.mu.Unlock()
	onLoop(func() { form.OnSubmit(map[string]string{"where": wikiEditor, "content": form.Fields[1].Default}, nil) })
	decided(4)
	assert.Equal(t, "ses_a/wp-6/edit/règle 6\n\nsur deux blocs", be.got()[3])
	be.mu.Lock()
	assert.Equal(t, []string{"wp-6.md|règle 6\nsuite\n"}, be.edits)
	be.mu.Unlock()

	// s on an accepted proposal puts it back; on a pending one it moves on.
	onLoop(func() { cw.wiki.list.Select("wp-1"); cw.HandleKey(key('s')) })
	decided(5)
	assert.Equal(t, "ses_a/wp-1/pending/", be.got()[4])
	onLoop(func() { cw.wiki.list.Select("wp-7"); cw.HandleKey(key('s')) })
	assert.Equal(t, "wp-8", current().ID)
	assert.Len(t, be.got(), 5)

	// A: every pending one (wp-1 again, wp-4, wp-7…wp-9); a failure stops.
	be.failOn = "wp-8"
	onLoop(func() { cw.HandleKey(key('A')) })
	require.Eventually(t, func() bool {
		for _, m := range sh.toastList() {
			if strings.Contains(m, "base verrouillée") {
				return true
			}
		}
		return false
	}, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, []string{"ses_a/wp-1/accept/", "ses_a/wp-4/accept/", "ses_a/wp-7/accept/"}, be.got()[5:])

	// Enter: back to Sessions, then the decision of the checkpoint.
	sh.mu.Lock()
	sh.form = nil
	sh.mu.Unlock()
	onLoop(func() { cw.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)) })
	assert.Equal(t, 1, sh.popped)
	require.Eventually(t, func() bool {
		sh.mu.Lock()
		defer sh.mu.Unlock()
		return sh.form != nil && strings.Contains(sh.form.Title, "cp-2")
	}, 3*time.Second, 20*time.Millisecond)
	onLoop(func() { cw.Unmount() })

	// Esc (unmounted by the shell): the card again.
	sh.mu.Lock()
	sh.title = ""
	sh.mu.Unlock()
	onLoop(func() { cw.Mount(tview.NewFlex(), app) })
	onLoop(func() { cw.Unmount() })
	require.Eventually(t, func() bool { title, _, _ := sh.modal(); return strings.Contains(title, "cp-2") }, 3*time.Second, 20*time.Millisecond)
	onLoop(func() { v.Unmount() })
}

// The configuration of the list keeps the key convention.
func TestCheckpointWikiConvention(t *testing.T) {
	be := &wikiSessions{}
	_, cw := newTestCheckpointWiki(t, be, be, &wikiCardShell{})
	require.NoError(t, cw.wiki.list.cfg.Validate())
}
