package shell

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func (s *simShell) contentFocused() (is bool) {
	s.loop(func() { is = s.content.HasFocus() })
	return is
}

// Esc closes a window whether it has buttons or not: a reader (no button) used
// to trap the user, only a click outside closed it.
func TestEscClosesEveryModal(t *testing.T) {
	cases := map[string][]views.ModalAction{
		"without button": nil,
		"with buttons":   {{Label: "ok"}, {Label: "other"}},
		"one button":     {{Label: "ok"}},
	}
	for name, actions := range cases {
		t.Run(name, func(t *testing.T) {
			s := newKeyShell(t, &keyView{id: "home", focusList: true})
			s.loop(func() { s.ShowScrollableModal("reader", "body", actions) })
			require.True(t, s.overlayShown())

			s.key(tcell.KeyEscape, 0, tcell.ModNone)
			assert.False(t, s.overlayShown(), "Esc closed the window")
			assert.True(t, s.contentFocused(), "the focus is back on the content")
		})
	}
}

// click presses and releases the left button on a cell.
func (p *panicSim) click(x, y int) {
	p.t.Helper()
	p.mouse(x, y, tcell.Button1)
	p.mouse(x, y, tcell.ButtonNone)
	p.settle()
	p.loopSafe(func() {})
}

func (p *panicSim) page(name string) (has bool) {
	p.t.Helper()
	p.loopSafe(func() { has = p.pages.HasPage(name) })
	return has
}

func (p *panicSim) focus() (f tview.Primitive) {
	p.t.Helper()
	p.loopSafe(func() { f = p.app.GetFocus() })
	return f
}

// cardWithReader shows a window with a « read » button that opens a reader
// over it, the way the decision card does for a report.
func cardWithReader(t *testing.T) *panicSim {
	t.Helper()
	p := runPanicSim(t, 120, 36, Config{ProjectName: "test", Views: []views.View{&keyView{id: "home", focusList: true}}, HomeViewID: "home"})
	p.loopSafe(func() {
		p.NavigateHome("home", views.ModeHub)
		p.ShowScrollableModal("the card", "text of the card", []views.ModalAction{
			{Label: "decide"},
			{Label: "read", KeepOpen: true, Callback: func() { p.ShowScrollableModal("the reader", "text of the reader", nil) }},
		})
	})
	p.press(tcell.KeyRight, 0) // « read »
	return p
}

// A reader opened from a window opens over it, on a darker backdrop: Esc
// or a click outside closes the reader and gives the card its focus back.
func TestReaderOpenedFromAWindowIsASubWindow(t *testing.T) {
	t.Run("Esc", func(t *testing.T) {
		p := cardWithReader(t)
		readButton := p.focus()
		p.press(tcell.KeyEnter, 0)
		require.True(t, p.page("inline-overlay"), "the card stays under the reader")
		require.True(t, p.page("sub-overlay"))
		assert.Contains(t, p.text(), "text of the reader")

		p.press(tcell.KeyEscape, 0)
		assert.False(t, p.page("sub-overlay"), "Esc closed the reader")
		assert.True(t, p.page("inline-overlay"), "the card is still there")
		assert.Contains(t, p.text(), "text of the card")
		assert.Same(t, readButton, p.focus(), "the card has the focus it had")

		p.press(tcell.KeyEscape, 0)
		assert.False(t, p.page("inline-overlay"), "Esc on the card closes it, as before")
		assert.True(t, p.loopContentFocused(), "the focus is back on the content")
	})

	t.Run("click outside", func(t *testing.T) {
		p := cardWithReader(t)
		readButton := p.focus()
		p.press(tcell.KeyEnter, 0)
		require.True(t, p.page("sub-overlay"))

		p.click(0, 0)
		assert.False(t, p.page("sub-overlay"), "the click closed the reader")
		assert.True(t, p.page("inline-overlay"), "and only the reader")
		assert.Contains(t, p.text(), "text of the card")
		assert.Same(t, readButton, p.focus())
	})

	t.Run("click inside the reader", func(t *testing.T) {
		p := cardWithReader(t)
		p.press(tcell.KeyEnter, 0)
		p.click(60, 18)
		assert.True(t, p.page("sub-overlay"), "a click inside does not close it")
	})

	t.Run("settled elsewhere", func(t *testing.T) {
		p := cardWithReader(t)
		var card any
		p.loopSafe(func() { card = p.CurrentOverlay() })
		p.press(tcell.KeyEnter, 0)
		require.True(t, p.page("sub-overlay"))
		var current any
		p.loopSafe(func() { current = p.CurrentOverlay() })
		assert.Same(t, card, current, "the overlay followed is the card, not the reader")

		var closed bool
		p.loopSafe(func() { closed = p.CloseOverlay(card) })
		assert.True(t, closed)
		assert.False(t, p.page("sub-overlay"), "the reader closes with the card")
		assert.False(t, p.page("inline-overlay"))
	})

	t.Run("a button without KeepOpen closes the card first", func(t *testing.T) {
		p := cardWithReader(t)
		p.press(tcell.KeyLeft, 0) // « decide »
		p.press(tcell.KeyEnter, 0)
		assert.False(t, p.page("inline-overlay"))
		assert.False(t, p.page("sub-overlay"))
	})

	t.Run("without a card", func(t *testing.T) {
		p := runPanicSim(t, 120, 36, Config{ProjectName: "test", Views: []views.View{&keyView{id: "home", focusList: true}}, HomeViewID: "home"})
		p.loopSafe(func() { p.NavigateHome("home", views.ModeHub); p.ShowScrollableModal("alone", "text", nil) })
		assert.True(t, p.page("inline-overlay"), "a window over the content stays a full window")
		assert.False(t, p.page("sub-overlay"))
	})
}

func (p *panicSim) loopContentFocused() (is bool) {
	p.t.Helper()
	p.loopSafe(func() { is = p.content.HasFocus() })
	return is
}

// The report reader of the real Sessions view: the card of the decision
// stays under it.
func TestSessionsReportReaderKeepsTheCard(t *testing.T) {
	setup := func(t *testing.T) (*panicSim, *fakeSessionsBackend, *views.SessionsView) {
		t.Helper()
		be := &fakeSessionsBackend{rows: sessionsMouseRows()}
		sv := views.NewSessionsView(views.SessionsViewConfig{Backend: be})
		p := runPanicSim(t, 120, 36, Config{ProjectName: "test", Views: []views.View{&keyView{id: "home", focusList: true}, sv}, HomeViewID: "home"})
		p.loopSafe(func() {
			p.NavigateHome("home", views.ModeHub)
			sv.Focus("d:d-cp1")
			p.NavigateTo(sv.ID())
		})
		require.Eventually(t, func() bool {
			p.loopSafe(func() {})
			be.mu.Lock()
			defer be.mu.Unlock()
			return be.listed > 0
		}, 3*time.Second, 10*time.Millisecond)
		p.press(tcell.KeyEnter, 0)
		awaitText(t, p, "Plan validation")
		p.press(tcell.KeyRight, 0) // « Read rp-1 »
		p.press(tcell.KeyEnter, 0)
		awaitText(t, p, "line of the report")
		require.True(t, p.page("inline-overlay"))
		require.True(t, p.page("sub-overlay"), "the reader is a sub-window")
		return p, be, sv
	}

	t.Run("Esc", func(t *testing.T) {
		p, _, _ := setup(t)
		p.press(tcell.KeyEscape, 0)
		assert.False(t, p.page("sub-overlay"))
		assert.True(t, p.page("inline-overlay"), "the card of the decision is still there")
		assert.Contains(t, p.text(), "Plan validation")
		assert.True(t, p.loopFocusIsButton(), "focused on a button of the card")
		p.press(tcell.KeyEscape, 0)
		assert.False(t, p.page("inline-overlay"), "Esc closes the card as before")
	})

	t.Run("click outside", func(t *testing.T) {
		p, _, _ := setup(t)
		p.click(0, 0)
		assert.False(t, p.page("sub-overlay"))
		assert.True(t, p.page("inline-overlay"), "the card stays")
		assert.Contains(t, p.text(), "Plan validation")
	})

	t.Run("settled elsewhere while reading", func(t *testing.T) {
		p, be, sv := setup(t)
		be.mu.Lock()
		be.rows = withoutDecision(be.rows, "d-cp1")
		be.mu.Unlock()
		p.loopSafe(func() { sv.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'r', 0)) }) // reload
		require.Eventually(t, func() bool { p.loopSafe(func() {}); return !p.page("inline-overlay") }, 3*time.Second, 20*time.Millisecond,
			"the card closes with the decision:\n%s", p.text())
		assert.False(t, p.page("sub-overlay"), "and the reader with it")
	})
}

func (p *panicSim) loopFocusIsButton() (is bool) {
	p.t.Helper()
	p.loopSafe(func() { _, is = p.app.GetFocus().(*tview.Button) })
	return is
}

func awaitText(t *testing.T, p *panicSim, want string) {
	t.Helper()
	require.Eventually(t, func() bool {
		p.loopSafe(func() {})
		return strings.Contains(p.text(), want)
	}, 3*time.Second, 20*time.Millisecond, "the screen shows %q:\n%s", want, p.text())
}

func withoutDecision(rows []views.SessionRow, id string) []views.SessionRow {
	out := make([]views.SessionRow, len(rows))
	for i, r := range rows {
		var keep []views.SessionDecision
		for _, d := range r.Decisions {
			if d.ID != id {
				keep = append(keep, d)
			}
		}
		r.Decisions = keep
		out[i] = r
	}
	return out
}
