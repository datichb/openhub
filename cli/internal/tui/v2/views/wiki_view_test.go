package views

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// wikiRepo is a team-state with a wiki: the other methods are not called.
type wikiRepo struct {
	teamstate.TeamStateWriter
	mu        sync.Mutex
	proposals []teamstate.WikiProposal
	pages     map[string]string
	calls     []string
	events    []teamstate.Event
	failWith  error
}

func (r *wikiRepo) Path() string               { return "" }
func (r *wikiRepo) Pull(context.Context) error { return nil }
func (r *wikiRepo) WikiListPages() ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, n := range []string{"architecture", "conventions"} {
		if _, ok := r.pages[n]; ok {
			out = append(out, n)
		}
	}
	return out, nil
}

func (r *wikiRepo) WikiReadPage(name string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.pages[name]; ok {
		return c, nil
	}
	return "", errors.New("introuvable")
}

func (r *wikiRepo) WikiListPending() ([]teamstate.WikiProposal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]teamstate.WikiProposal(nil), r.proposals...), nil
}

func (r *wikiRepo) decide(verb, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, verb+" "+id)
	if r.failWith != nil {
		return r.failWith
	}
	for i, p := range r.proposals {
		if p.ID == id {
			r.proposals = append(r.proposals[:i], r.proposals[i+1:]...)
			break
		}
	}
	return nil
}

func (r *wikiRepo) WikiAcceptProposal(_ context.Context, id string) error {
	return r.decide("accept", id)
}
func (r *wikiRepo) WikiRejectProposal(_ context.Context, id string) error {
	return r.decide("reject", id)
}

func (r *wikiRepo) AppendEvent(_ context.Context, e teamstate.Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

func (r *wikiRepo) got() ([]string, []teamstate.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...), append([]teamstate.Event(nil), r.events...)
}

// wikiShell records the modals, toasts and confirmations of the view;
// confirmations are accepted.
type wikiShell struct {
	mockShell
	mu       sync.Mutex
	title    string
	body     string
	actions  []ModalAction
	toasts   []string
	confirms []string
}

func (s *wikiShell) ShowScrollableModal(title, body string, actions []ModalAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.title, s.body, s.actions = title, body, actions
}

func (s *wikiShell) ShowToastMsg(msg string, _ bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toasts = append(s.toasts, msg)
}

func (s *wikiShell) Confirm(title, detail string, _ bool, onYes func()) {
	s.mu.Lock()
	s.confirms = append(s.confirms, title+" · "+detail)
	s.mu.Unlock()
	onYes()
}

func (s *wikiShell) modal() (string, string, []ModalAction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.title, s.body, s.actions
}

func (s *wikiShell) toastList() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.toasts...)
}

// runWikiView mounts the view on a running application (simulation
// screen); onLoop runs a function on its event loop.
func runWikiView(t *testing.T, v *WikiView) (onLoop func(func())) {
	t.Helper()
	sim := tcell.NewSimulationScreen("")
	require.NoError(t, sim.Init())
	sim.SetSize(160, 40)
	app := tview.NewApplication().SetScreen(sim)
	content := tview.NewFlex()
	app.SetRoot(content, true)
	done := make(chan struct{})
	go func() { _ = app.Run(); close(done) }()
	t.Cleanup(func() { app.Stop(); <-done })
	onLoop = func(fn func()) {
		ch := make(chan struct{})
		app.QueueUpdateDraw(func() { fn(); close(ch) })
		<-ch
	}
	onLoop(func() { v.Mount(content, app) })
	return onLoop
}

// The team wiki view: proposals then pages, the detail of the selected
// item, accept (a), reject (x, confirmed), the proposal and page modals.
func TestWikiView(t *testing.T) {
	withFrench(t)
	repo := &wikiRepo{
		proposals: []teamstate.WikiProposal{
			{ID: "wp-a", Page: "conventions", Content: "kebab-case [red]partout[-]", Confidence: "CONFIRMED", Author: "reviewer", Project: "atlas",
				CreatedAt: time.Now().Add(-2 * time.Hour)},
			{ID: "wp-b", Page: "architecture", Content: "hexagonale", Confidence: "UNCERTAIN", Author: "developer", Project: "atlas"},
		},
		pages: map[string]string{"architecture": "# Architecture [b]x[-]", "conventions": "# Conventions"},
	}
	sh := &wikiShell{}
	v := NewWikiView(func() TeamResolution { return TeamResolution{Enabled: true, MemberID: "alice"} })
	v.openRepo = func() teamstate.TeamStateWriter { return repo }
	v.SetShell(sh)
	onLoop := runWikiView(t, v)

	loaded := func(n int) func() bool {
		return func() bool {
			ok := false
			items := 3 // the pages and their section
			if n > 0 {
				items += n + 1
			}
			onLoop(func() { ok = len(v.proposals) == n && v.slist != nil && len(v.slist.GetItems()) == items })
			return ok
		}
	}
	require.Eventually(t, loaded(2), 3*time.Second, 20*time.Millisecond)

	var header, detail string
	var texts []string
	var current int
	onLoop(func() {
		header = v.header.GetText(true)
		for _, it := range v.slist.GetItems() {
			texts = append(texts, it.MainText)
		}
		current, _, _ = v.slist.CurrentItem()
		detail = v.page.Detail.GetText(true)
	})
	assert.Contains(t, header, "2 proposition(s) en attente")
	require.Len(t, texts, 6, "two sections, two proposals, two pages")
	assert.Contains(t, texts[0], "Propositions en attente (2)")
	assert.Contains(t, texts[1], "[wp-a] conventions")
	assert.Contains(t, texts[1], "reviewer")
	assert.Contains(t, texts[1], "il y a 2 heures")
	assert.Contains(t, texts[3], "Pages wiki (2)")
	assert.Contains(t, texts[4], "architecture")
	assert.Equal(t, 1, current, "the first proposal is selected")
	assert.Contains(t, detail, "conventions  [wp-a]")
	assert.Contains(t, detail, "reviewer")
	assert.Contains(t, detail, "kebab-case [red]partout[-]", "agent text escaped, shown as is")

	// Selection: the detail follows.
	onLoop(func() { v.slist.SelectIndex(2); v.showItemDetail(2); detail = v.page.Detail.GetText(true) })
	assert.Contains(t, detail, "architecture  [wp-b]")
	assert.Contains(t, detail, "UNCERTAIN")
	onLoop(func() { v.showItemDetail(4); detail = v.page.Detail.GetText(true) })
	assert.Contains(t, detail, "wiki/architecture.md")

	// Enter on a proposal: its modal, accept / reject / close.
	onLoop(func() { v.slist.SelectIndex(1); v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)) })
	title, body, actions := sh.modal()
	assert.Equal(t, "Proposition : conventions", title)
	assert.Contains(t, body, "kebab-case [red[]partout[-[]")
	require.Len(t, actions, 3)
	assert.Equal(t, []string{"[Accepter]", "[Rejeter]", "[Fermer]"}, []string{actions[0].Label, actions[1].Label, actions[2].Label})

	// a: the selected proposal is accepted, an event recorded, the list
	// reloaded.
	onLoop(func() { v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'a', 0)) })
	require.Eventually(t, loaded(1), 3*time.Second, 20*time.Millisecond)
	calls, events := repo.got()
	assert.Equal(t, []string{"accept wp-a"}, calls)
	require.Len(t, events, 1)
	assert.Equal(t, teamstate.EventWikiAccepted, events[0].Type)
	assert.Equal(t, "alice", events[0].Actor)
	assert.Equal(t, "wp-a", events[0].Data["proposal_id"])
	assert.Contains(t, sh.toastList(), "Proposition acceptée : conventions")

	// x: confirmed, then rejected.
	onLoop(func() { v.slist.SelectIndex(1); v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'x', 0)) })
	require.Eventually(t, loaded(0), 3*time.Second, 20*time.Millisecond)
	calls, events = repo.got()
	assert.Equal(t, []string{"accept wp-a", "reject wp-b"}, calls)
	assert.Equal(t, teamstate.EventWikiRejected, events[1].Type)
	sh.mu.Lock()
	assert.Equal(t, []string{"Rejeter la proposition ? · La proposition pour la page architecture (wp-b) sera supprimée."}, sh.confirms)
	sh.mu.Unlock()
	assert.Contains(t, sh.toastList(), "Proposition rejetée : architecture")

	// Only pages left: Enter opens a page (escaped).
	onLoop(func() {
		header = v.header.GetText(true)
		v.slist.SelectIndex(1)
		v.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0))
	})
	assert.Contains(t, header, "Pages du wiki d'équipe")
	title, body, _ = sh.modal()
	assert.Equal(t, "architecture", title)
	assert.Equal(t, "# Architecture [b[]x[-[]", body)
	// a / x on a page do nothing.
	onLoop(func() {
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'a', 0))
		v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'x', 0))
	})
	calls, _ = repo.got()
	assert.Len(t, calls, 2)
	onLoop(func() { v.Unmount() })
}

// A failed decision is shown and the proposal stays; an empty wiki and a
// team-state not configured have their own header.
func TestWikiViewFailuresAndEmpty(t *testing.T) {
	withFrench(t)
	repo := &wikiRepo{proposals: []teamstate.WikiProposal{{ID: "wp-a", Page: "conventions", Author: "reviewer"}},
		failWith: errors.New("push refusé")}
	sh := &wikiShell{}
	v := NewWikiView(func() TeamResolution { return TeamResolution{Enabled: true} })
	v.openRepo = func() teamstate.TeamStateWriter { return repo }
	v.SetShell(sh)
	onLoop := runWikiView(t, v)
	require.Eventually(t, func() bool {
		ok := false
		onLoop(func() { ok = len(v.proposals) == 1 })
		return ok
	}, 3*time.Second, 20*time.Millisecond)
	onLoop(func() { v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'a', 0)) })
	require.Eventually(t, func() bool {
		for _, m := range sh.toastList() {
			if m == "Échec d'acceptation : push refusé" {
				return true
			}
		}
		return false
	}, 3*time.Second, 20*time.Millisecond)
	_, events := repo.got()
	assert.Empty(t, events, "no event for a failed decision")

	repo.mu.Lock()
	repo.proposals, repo.failWith = nil, nil
	repo.mu.Unlock()
	var header string
	var texts []string
	onLoop(func() { v.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'r', 0)) })
	require.Eventually(t, func() bool {
		onLoop(func() {
			header = v.header.GetText(true)
			texts = nil
			for _, it := range v.slist.GetItems() {
				texts = append(texts, it.MainText)
			}
		})
		return len(texts) == 1
	}, 3*time.Second, 20*time.Millisecond)
	assert.Contains(t, header, "Wiki vide")
	assert.Contains(t, texts[0], "Aucune page wiki")
	onLoop(func() { v.Unmount() })

	off := NewWikiView(func() TeamResolution { return TeamResolution{} })
	off.SetShell(sh)
	onLoop = runWikiView(t, off)
	onLoop(func() { header = off.header.GetText(true) })
	assert.Contains(t, header, "Team non configurée")
	onLoop(func() { off.doAcceptProposal(teamstate.WikiProposal{ID: "wp-z"}) })
	assert.Contains(t, sh.toastList(), "Team non configurée")
	assert.Len(t, off.ContextCommands(), 2)
	assert.Equal(t, "team.wiki", off.ID())
}
