package shell

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

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
	"github.com/datichb/openhub/cli/internal/workflow"
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
	sim   tcell.SimulationScreen
	fence chan struct{}
	t     *testing.T
}

// fenceKey is never typed: key() injects it after a key to know the event
// loop handled that key (the events are handled in the order they come).
const fenceKey = tcell.KeyF64

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
	ss := &simShell{Shell: s, sim: sim, fence: make(chan struct{}, 1), t: t}
	global := s.app.GetInputCapture()
	s.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == fenceKey {
			ss.fence <- struct{}{}
			return nil
		}
		return global(event)
	})
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
	s.t.Helper()
	s.sim.InjectKey(k, r, mod)
	s.sim.InjectKey(fenceKey, 0, tcell.ModNone)
	select {
	case <-s.fence:
	case <-time.After(10 * time.Second):
		s.t.Fatal("the event loop did not handle the key")
	}
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

// showToastAndWait shows a toast that stays (error) and waits until it is up.
func showToastAndWait(t *testing.T, s *simShell) {
	t.Helper()
	s.ShowToast("hello", ToastError)
	require.Eventually(t, func() bool { return s.toastCount() == 1 }, 2*time.Second, 5*time.Millisecond)
}

func (s *simShell) toastCount() (n int) {
	s.loop(func() { n = len(s.activeToastIDs) })
	return n
}

const keysLaunchSpec = `apiVersion: oh/v1
kind: Workflow
id: ticket
description: { fr: Implémenter un ticket, en: Implement a ticket }
risk: write
inputs:
  ticket: { type: beads-id, required: true }
modes: { default: semi-auto, allowed: [manuel, semi-auto] }
runtime: { default: local, allowed: [local] }
`

// In the launch form (input capturing), Esc closes the form at once: it
// does not close the toasts, which go away on their own.
func TestKeysEscInLaunchFormKeepsTheToasts(t *testing.T) {
	doc, diags := workflow.Parse([]byte(keysLaunchSpec), workflow.Source{Layer: workflow.LayerHub})
	require.False(t, diags.HasErrors(), "%v", diags)
	form := views.NewLaunchFormView(views.LaunchFormConfig{WorkflowID: "ticket", Spec: doc.Spec, Lang: "fr",
		Runtimes:  []views.LaunchRuntime{{Kind: "local", Label: "local", Available: true}},
		Locations: []views.SelectOption{{Label: "base", Value: "base"}}})
	home := &keyView{id: "home", focusList: true}
	s := newKeyShell(t, home)
	s.loop(func() { s.PushView(form) })
	showToastAndWait(t, s)

	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	depth := 0
	s.loop(func() { depth = s.router.StackDepth() })
	assert.Equal(t, 1, depth, "Esc closes the form")
	assert.Equal(t, 1, s.toastCount(), "the toast stays")
}

// In a wizard (input capturing), Esc asks « Quit the wizard? » at once and
// keeps the toasts; outside, Esc closes them again.
func TestKeysEscInWizardKeepsTheToasts(t *testing.T) {
	wiz := views.NewInlineWizardView(views.InlineWizardConfig{ID: "w", Steps: []views.WizardStep{{Label: "a", Required: true,
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			f := tview.NewForm()
			f.AddInputField("Name", "", 0, nil, nil)
			f.AddButton("Next", onDone)
			return f
		}}}})
	home := &keyView{id: "home", focusList: true}
	s := newKeyShell(t, home)
	s.loop(func() { s.PushView(wiz) })
	showToastAndWait(t, s)

	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	focused := func() (label string) {
		s.loop(func() {
			if b, ok := s.app.GetFocus().(*tview.Button); ok {
				label = widgets.ButtonLabel(b)
			}
		})
		return label
	}
	require.Eventually(t, func() bool { return focused() == i18n.T("tui.wizard.quit.continue") }, time.Second, 5*time.Millisecond, "question asked")
	assert.Equal(t, 1, s.toastCount(), "the toast stays")

	s.key(tcell.KeyLeft, 0, tcell.ModNone) // « Quitter »
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	require.Eventually(t, func() bool {
		depth := 0
		s.loop(func() { depth = s.router.StackDepth() })
		return depth == 1
	}, time.Second, 5*time.Millisecond, "wizard quit")
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.Equal(t, 0, s.toastCount(), "outside the wizard, Esc closes the toasts")
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
		s.loop(func() {
			list, isList := s.app.GetFocus().(*widgets.SectionedList)
			ok = isList && len(list.GetItems()) > 0 // the catalogue is loaded
		})
		return ok
	}, 2*time.Second, 5*time.Millisecond)
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	select {
	case id := <-launched:
		assert.Equal(t, "ticket", id)
	case <-time.After(2 * time.Second):
		t.Fatal("Enter did not launch the workflow")
	}
	assert.False(t, s.omnibarActive())
}

// Confirm runs onYes only once confirmed; Esc cancels. A dangerous action
// starts on « Annuler » (Enter by reflex cancels), a safe one on « Oui » (N19).
func TestConfirm(t *testing.T) {
	v := &keyView{id: "home", focusList: true}
	s := newKeyShell(t, v)
	yes := 0
	focused := func() (label string) {
		s.loop(func() {
			if b, ok := s.app.GetFocus().(*tview.Button); ok {
				label = widgets.ButtonLabel(b)
			}
		})
		return label
	}
	shown := func() (ok bool) {
		s.loop(func() { ok = s.pages.HasPage("inline-overlay") })
		return ok
	}
	s.loop(func() { s.Confirm("Delete?", "detail", true, func() { yes++ }) })
	assert.Equal(t, i18n.T("tui.confirm.cancel"), focused())
	s.key(tcell.KeyEnter, 0, tcell.ModNone) // « Annuler »: nothing done
	assert.False(t, shown())
	assert.Equal(t, 0, yes)

	s.loop(func() { s.Confirm("Delete?", "detail", true, func() { yes++ }) })
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.False(t, shown())
	assert.Equal(t, 0, yes)

	s.loop(func() { s.Confirm("Delete?", "detail", true, func() { yes++ }) })
	s.key(tcell.KeyBacktab, 0, tcell.ModNone) // to « Oui »
	assert.Equal(t, i18n.T("tui.confirm.yes"), focused())
	s.key(tcell.KeyEnter, 0, tcell.ModNone) // press it
	assert.Equal(t, 1, yes)
	var focus tview.Primitive
	s.loop(func() { focus = s.app.GetFocus() })
	_, onList := focus.(*widgets.SectionedList)
	assert.True(t, onList, "focus back on the list")

	// Safe action: the focus starts on « Oui »; Tab cycles text → buttons.
	s.loop(func() { s.Confirm("Save?", "detail", false, func() { yes++ }) })
	assert.Equal(t, i18n.T("tui.confirm.yes"), focused())
	s.key(tcell.KeyTab, 0, tcell.ModNone) // « Annuler »
	assert.Equal(t, i18n.T("tui.confirm.cancel"), focused())
	s.key(tcell.KeyTab, 0, tcell.ModNone) // the text
	assert.Equal(t, "", focused())
	s.key(tcell.KeyDown, 0, tcell.ModNone) // ↓ scrolls the text, stays there
	assert.Equal(t, "", focused())
	s.key(tcell.KeyTab, 0, tcell.ModNone) // « Oui » again
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	assert.Equal(t, 2, yes)
}

// Recap without provider credentials: Enter on « Configure the provider »
// opens the provider view, Esc comes back to the form and the recap checks
// again (now configured).
func TestKeysLaunchFormConfigureProvider(t *testing.T) {
	doc, diags := workflow.Parse([]byte(keysLaunchSpec), workflow.Source{Layer: workflow.LayerHub})
	require.False(t, diags.HasErrors(), "%v", diags)
	var mu sync.Mutex
	configured, calls := false, 0
	home := &keyView{id: "home", focusList: true}
	providerView := &keyView{id: "provider", focusList: true}
	s := newKeyShell(t, home, providerView)
	form := views.NewLaunchFormView(views.LaunchFormConfig{WorkflowID: "ticket", Spec: doc.Spec, Lang: "fr", Tickets: []string{"bd-1"},
		Runtimes:  []views.LaunchRuntime{{Kind: "local", Label: "local", Available: true}},
		Locations: []views.SelectOption{{Label: "base", Value: "base"}},
		Recap: func(context.Context, views.LaunchChoices) (*views.LaunchRecap, error) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if !configured {
				return nil, views.ProviderCredentialsError("no credentials")
			}
			return &views.LaunchRecap{}, nil
		},
		ConfigureProvider: func() { s.NavigateTo("provider") }})
	s.loop(func() { s.PushView(form) })
	s.key(tcell.KeyCtrlS, 0, tcell.ModNone)
	label := i18n.T("tui.launch.configure_provider")
	focused := func() (l string) {
		s.loop(func() {
			if b, ok := s.app.GetFocus().(*tview.Button); ok {
				l = widgets.ButtonLabel(b)
			}
		})
		return l
	}
	require.Eventually(t, func() bool { return focused() == label }, 2*time.Second, 5*time.Millisecond)

	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	current := func() (id string) {
		s.loop(func() { id = s.router.Current().ID() })
		return id
	}
	require.Equal(t, "provider", current())
	mu.Lock()
	configured = true
	mu.Unlock()
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	require.Equal(t, form.ID(), current(), "back on the form")
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return calls == 2
	}, 2*time.Second, 5*time.Millisecond, "recap checked again")
	assert.Eventually(t, func() bool { return focused() != label }, 2*time.Second, 5*time.Millisecond, "button gone")
}

// A text field with a check shows its message under the field as it is
// typed; an invalid value is not submitted (Bedrock region).
func TestInlineFormValidateAsTyped(t *testing.T) {
	home := &keyView{id: "home", focusList: true}
	s := newKeyShell(t, home)
	submitted := make(chan string, 1)
	s.loop(func() {
		s.ShowInlineForm(views.InlineFormConfig{Title: "Region",
			Fields: []views.FormField{{Key: "region", Label: "Region", Type: views.FieldText,
				Validate: func(v string) (string, bool) {
					if v == "bad" {
						return "invalid region", true
					}
					return "", false
				}}},
			OnSubmit: func(vals map[string]string, _ map[string][]string) { submitted <- vals["region"] }})
	})
	for _, r := range "bad" {
		s.key(tcell.KeyRune, r, tcell.ModNone)
	}
	screen := func() string {
		cells, w, h := s.sim.GetContents()
		var b strings.Builder
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if rs := cells[y*w+x].Runes; len(rs) > 0 {
					b.WriteRune(rs[0])
				}
			}
			b.WriteByte('\n')
		}
		return b.String()
	}
	require.Eventually(t, func() bool { s.loop(func() {}); return strings.Contains(screen(), "✗ invalid region") }, time.Second, 5*time.Millisecond, "message under the field")

	s.key(tcell.KeyTab, 0, tcell.ModNone)   // to « Confirm »
	s.key(tcell.KeyEnter, 0, tcell.ModNone) // refused
	select {
	case v := <-submitted:
		t.Fatalf("invalid value submitted: %q", v)
	case <-time.After(30 * time.Millisecond):
	}
	shown := false
	s.loop(func() { shown = s.pages.HasPage("inline-overlay") })
	assert.True(t, shown, "the form stays")

	s.key(tcell.KeyBacktab, 0, tcell.ModNone) // back to the field
	for range 3 {
		s.key(tcell.KeyBackspace2, 0, tcell.ModNone)
	}
	for _, r := range "good" {
		s.key(tcell.KeyRune, r, tcell.ModNone)
	}
	s.loop(func() {})
	assert.NotContains(t, screen(), "✗ invalid region", "message cleared")
	s.key(tcell.KeyTab, 0, tcell.ModNone)
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	select {
	case v := <-submitted:
		assert.Equal(t, "good", v)
	case <-time.After(time.Second):
		t.Fatal("valid value not submitted")
	}
}

// capturingView keeps the keys (like the tabs of Sessions); passes lets the
// shell take ? and Ctrl+P.
type capturingView struct {
	keyView
	passes bool
}

func (v *capturingView) CapturesInput() bool   { return true }
func (v *capturingView) PassesShellKeys() bool { return v.passes }

// In a view that keeps the keys but passes ? and Ctrl+P (tabs, batch of
// Sessions), ? opens the help and Ctrl+P the palette; the focus comes back
// to the widget of the view. A form that keeps every key blocks them.
func TestKeysHelpAndPaletteInCapturingView(t *testing.T) {
	home := &keyView{id: "home", focusList: true}
	tab := &capturingView{keyView: keyView{id: "tab", focusList: true}, passes: true}
	s := newKeyShell(t, home)
	s.loop(func() { s.PushView(tab) })
	var before tview.Primitive
	s.loop(func() { before = s.app.GetFocus() })

	s.key(tcell.KeyCtrlP, 0, tcell.ModNone)
	assert.True(t, s.omnibarActive())
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.False(t, s.omnibarActive())
	var after tview.Primitive
	s.loop(func() { after = s.app.GetFocus() })
	assert.Equal(t, before, after, "focus back on the widget of the view")

	s.key(tcell.KeyRune, '?', tcell.ModNone)
	help := false
	s.loop(func() { help = s.pages.HasPage("inline-overlay") })
	assert.True(t, help, "? opens the help")
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	depth := 0
	s.loop(func() { depth = s.router.StackDepth() })
	assert.Equal(t, 2, depth, "Esc closes the help, not the view")

	form := &capturingView{keyView: keyView{id: "form", focusList: true}}
	s.loop(func() { s.PushView(form) })
	s.key(tcell.KeyCtrlP, 0, tcell.ModNone)
	assert.False(t, s.omnibarActive(), "a form keeps Ctrl+P")
	s.key(tcell.KeyRune, '?', tcell.ModNone)
	s.loop(func() { help = s.pages.HasPage("inline-overlay") })
	assert.False(t, help, "a form keeps ?")
}
