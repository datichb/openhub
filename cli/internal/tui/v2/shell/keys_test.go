package shell

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// keyView is a view with a SectionedList (focusable or not) that records
// what reaches it: its own keys (HandleKey) and the list selections.
type keyView struct {
	id        string
	focusList bool   // the list takes the focus (else a text view does)
	consume   string // runes consumed by HandleKey
	mu        sync.Mutex
	keys      []rune
	selected  int
}

func (v *keyView) ID() string          { return v.id }
func (v *keyView) Title() string       { return v.id }
func (v *keyView) StatusHints() string { return v.id + " hints" }
func (v *keyView) Unmount()            {}
func (v *keyView) Mount(content *tview.Flex, app *tview.Application) {
	list := widgets.NewSectionedList()
	list.SetItems([]widgets.SectionItem{{MainText: "one"}, {MainText: "two"}})
	list.SetItemSelectedFunc(func(int, widgets.SectionItem) {
		v.mu.Lock()
		v.selected++
		v.mu.Unlock()
	})
	text := tview.NewTextView().SetText("read only")
	content.AddItem(list, 0, 1, v.focusList)
	content.AddItem(text, 1, 0, !v.focusList)
}
func (v *keyView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyRune {
		for _, r := range v.consume {
			if r == event.Rune() {
				v.mu.Lock()
				v.keys = append(v.keys, r)
				v.mu.Unlock()
				return nil
			}
		}
	}
	return event
}

func (v *keyView) seen() ([]rune, int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]rune(nil), v.keys...), v.selected
}

// simShell runs a shell on a simulation screen of the given size.
type simShell struct {
	*Shell
	sim tcell.SimulationScreen
}

func runSimShell(t *testing.T, cols, rows int, cfg Config) *simShell {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sim.Init())
	s := New(cfg)
	s.app.SetScreen(sim) // initialises the screen again (80×25)
	sim.SetSize(cols, rows)
	done := make(chan struct{})
	go func() { _ = s.Run(); close(done) }()
	t.Cleanup(func() { s.app.Stop(); <-done })
	ss := &simShell{Shell: s, sim: sim}
	require.Eventually(t, func() bool { return s.IsRunning() }, 2*time.Second, 5*time.Millisecond)
	return ss
}

// loop runs fn on the event loop and waits for it.
func (s *simShell) loop(fn func()) {
	ch := make(chan struct{})
	s.app.QueueUpdateDraw(func() { fn(); close(ch) })
	<-ch
}

// key injects a key and waits until the event loop handled it.
func (s *simShell) key(k tcell.Key, r rune, mod tcell.ModMask) {
	s.sim.InjectKey(k, r, mod)
	time.Sleep(5 * time.Millisecond)
	s.loop(func() {})
	s.loop(func() {})
}

func (s *simShell) omnibarActive() (active bool) {
	s.loop(func() { active = s.omnibar.IsActive() })
	return active
}

func newKeyShell(t *testing.T, home *keyView, extra ...views.View) *simShell {
	t.Helper()
	all := append([]views.View{home}, extra...)
	s := runSimShell(t, 120, 40, Config{ProjectName: "test", Views: all, HomeViewID: home.id})
	s.loop(func() { s.NavigateHome(home.id, views.ModeHub) })
	return s
}

// Enter on a focused SectionedList selects the item, it does not open the
// palette (N19).
func TestKeysEnterGoesToTheFocusedList(t *testing.T) {
	v := &keyView{id: "home", focusList: true}
	s := newKeyShell(t, v)
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	_, selected := v.seen()
	assert.Equal(t, 1, selected)
	assert.False(t, s.omnibarActive())
}

// Without a widget using Enter, Enter opens the palette.
func TestKeysEnterWithoutListOpensThePalette(t *testing.T) {
	v := &keyView{id: "home"}
	s := newKeyShell(t, v)
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	assert.True(t, s.omnibarActive())
}

// / and Ctrl+P open the palette; another letter does not.
func TestKeysPaletteOnSlashAndCtrlP(t *testing.T) {
	v := &keyView{id: "home", focusList: true}
	s := newKeyShell(t, v)
	s.key(tcell.KeyRune, 'x', tcell.ModNone)
	assert.False(t, s.omnibarActive(), "an unused letter does nothing")
	s.key(tcell.KeyRune, '/', tcell.ModNone)
	assert.True(t, s.omnibarActive())
	s.loop(func() { s.omnibar.Deactivate() })
	s.key(tcell.KeyCtrlP, 0, tcell.ModNone)
	assert.True(t, s.omnibarActive())
}

// Esc closes the toasts shown without going back; then Esc goes back.
func TestKeysEscClosesTheToastsFirst(t *testing.T) {
	home := &keyView{id: "home", focusList: true}
	second := &keyView{id: "second", focusList: true}
	s := newKeyShell(t, home)
	s.loop(func() { s.PushView(second) })
	s.ShowToast("hello", ToastInfo)
	require.Eventually(t, func() bool {
		n := 0
		s.loop(func() { n = len(s.activeToastIDs) })
		return n == 1
	}, 2*time.Second, 5*time.Millisecond)

	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	var depth, toasts int
	s.loop(func() { depth, toasts = s.router.StackDepth(), len(s.activeToastIDs) })
	assert.Equal(t, 0, toasts, "toast closed")
	assert.Equal(t, 2, depth, "not popped")

	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	s.loop(func() { depth = s.router.StackDepth() })
	assert.Equal(t, 1, depth, "second Esc goes back")
}

// `d` handled by a view goes to the view only: it no longer closes a toast.
func TestKeysViewLetterBeforeToast(t *testing.T) {
	v := &keyView{id: "home", focusList: true, consume: "d"}
	s := newKeyShell(t, v)
	s.ShowToast("hello", ToastInfo)
	require.Eventually(t, func() bool {
		n := 0
		s.loop(func() { n = len(s.activeToastIDs) })
		return n == 1
	}, 2*time.Second, 5*time.Millisecond)
	s.key(tcell.KeyRune, 'd', tcell.ModNone)
	keys, _ := v.seen()
	assert.Equal(t, []rune{'d'}, keys)
	n := 0
	s.loop(func() { n = len(s.activeToastIDs) })
	assert.Equal(t, 1, n, "the toast stays")
}

// ? opens the help, before the view.
func TestKeysQuestionMarkOpensTheHelp(t *testing.T) {
	v := &keyView{id: "home", focusList: true, consume: "?"}
	s := newKeyShell(t, v)
	s.key(tcell.KeyRune, '?', tcell.ModNone)
	keys, _ := v.seen()
	assert.Empty(t, keys, "reserved key")
	shown := false
	s.loop(func() { shown = s.pages.HasPage("inline-overlay") })
	assert.True(t, shown)
}

// Workflow catalogue: Enter launches the selected workflow (N19).
func TestKeysCatalogEnterLaunches(t *testing.T) {
	launched := make(chan string, 1)
	cat := views.NewWorkflowCatalogView(views.WorkflowCatalogConfig{
		Load: func(context.Context) (views.CatalogData, error) {
			return views.CatalogData{Entries: []views.CatalogEntry{{ID: "ticket", Layer: "hub", Risk: "write", Valid: true, Chain: []string{"hub:ticket"}}}}, nil
		},
		Launch: func(id string) { launched <- id },
	})
	home := &keyView{id: "home"}
	s := newKeyShell(t, home, cat)
	s.loop(func() { s.NavigateTo("workflows") })
	require.Eventually(t, func() bool {
		ok := false
		s.loop(func() { _, ok = s.app.GetFocus().(*widgets.SectionedList) })
		return ok
	}, 2*time.Second, 5*time.Millisecond)
	// Wait for the catalogue to load.
	time.Sleep(50 * time.Millisecond)
	s.loop(func() {})
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	select {
	case id := <-launched:
		assert.Equal(t, "ticket", id)
	case <-time.After(2 * time.Second):
		t.Fatal("Enter did not launch the workflow")
	}
	assert.False(t, s.omnibarActive())
}

// Confirm runs onYes only once confirmed; Esc cancels.
func TestConfirm(t *testing.T) {
	v := &keyView{id: "home", focusList: true}
	s := newKeyShell(t, v)
	yes := 0
	s.loop(func() { s.Confirm("Delete?", "detail", true, func() { yes++ }) })
	s.key(tcell.KeyEnter, 0, tcell.ModNone) // focus on the text: nothing
	s.loop(func() {})
	assert.Equal(t, 0, yes)
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	shown := true
	s.loop(func() { shown = s.pages.HasPage("inline-overlay") })
	assert.False(t, shown)
	assert.Equal(t, 0, yes)

	s.loop(func() { s.Confirm("Delete?", "detail", true, func() { yes++ }) })
	s.key(tcell.KeyTab, 0, tcell.ModNone)   // to « Confirm »
	s.key(tcell.KeyEnter, 0, tcell.ModNone) // press it
	assert.Equal(t, 1, yes)
	var focus tview.Primitive
	s.loop(func() { focus = s.app.GetFocus() })
	_, onList := focus.(*widgets.SectionedList)
	assert.True(t, onList, "focus back on the list")
}
