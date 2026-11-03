package shell

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The windows of the real Sessions view: what a button of the decision card
// opens stacks over the card, an empty instruction is refused without closing
// anything, a search in a reader gives the reader its focus back.

// decidingBackend records the answers and can fail the verification of the
// checkpoint card.
type decidingBackend struct {
	*fakeSessionsBackend
	dmu     sync.Mutex
	decided []string
	verify  views.CheckpointVerifyView
}

func (b *decidingBackend) Decide(_ context.Context, id, choice, message string, _ map[string]string) error {
	b.dmu.Lock()
	defer b.dmu.Unlock()
	b.decided = append(b.decided, fmt.Sprintf("%s|%s|%s", id, choice, message))
	return nil
}

func (b *decidingBackend) CheckpointCard(ctx context.Context, id string) (views.CheckpointCardView, error) {
	c, err := b.fakeSessionsBackend.CheckpointCard(ctx, id)
	c.Verify = b.verify
	return c, err
}

func (b *decidingBackend) answers() []string {
	b.dmu.Lock()
	defer b.dmu.Unlock()
	return append([]string(nil), b.decided...)
}

// sessionsSim mounts the real Sessions view on the row `focus`.
func sessionsSim(t *testing.T, focus string, be *decidingBackend, cfg Config) *panicSim {
	t.Helper()
	sv := views.NewSessionsView(views.SessionsViewConfig{Backend: be})
	cfg.ProjectName, cfg.Views, cfg.HomeViewID = "test", []views.View{&keyView{id: "home", focusList: true}, sv}, "home"
	p := runPanicSim(t, 120, 36, cfg)
	p.loopSafe(func() {
		p.NavigateHome("home", views.ModeHub)
		sv.Focus(focus)
		p.NavigateTo(sv.ID())
	})
	require.Eventually(t, func() bool {
		p.loopSafe(func() {})
		be.mu.Lock()
		defer be.mu.Unlock()
		return be.listed > 0
	}, 3*time.Second, 10*time.Millisecond)
	return p
}

func newDecidingBackend() *decidingBackend {
	return &decidingBackend{fakeSessionsBackend: &fakeSessionsBackend{rows: sessionsMouseRows()}}
}

// « Decide » opens the form over the card; Esc gives the card back, with its
// button.
func TestSessionsDecideFormStacksOverTheCard(t *testing.T) {
	p := sessionsSim(t, "d:d-cp1", newDecidingBackend(), Config{})
	p.press(tcell.KeyEnter, 0)
	awaitText(t, p, "Plan validation")
	decide := p.focus()

	p.press(tcell.KeyEnter, 0) // « Decide »
	require.True(t, p.page("sub-overlay"), "the form opens over the card:\n%s", p.text())
	require.True(t, p.page("inline-overlay"))

	p.press(tcell.KeyEscape, 0)
	assert.False(t, p.page("sub-overlay"))
	require.True(t, p.page("inline-overlay"), "the card stays")
	assert.Contains(t, p.text(), "Plan validation")
	assert.Same(t, decide, p.focus(), "on the button it had")

	p.press(tcell.KeyEscape, 0)
	assert.False(t, p.hasOverlayOnLoop())
}

// A choice that needs an instruction is refused while the form is open: the
// card and what was typed stay, nothing is sent.
func TestSessionsDecideRefusesAnEmptyMessageWithoutClosing(t *testing.T) {
	be := newDecidingBackend()
	p := sessionsSim(t, "d:d-cp1", be, Config{})
	p.press(tcell.KeyEnter, 0)
	awaitText(t, p, "Plan validation")
	p.press(tcell.KeyEnter, 0) // « Decide »
	p.press(tcell.KeyRight, 0) // the choice: fix
	p.press(tcell.KeyTab, 0)   // the message
	p.press(tcell.KeyTab, 0)   // « Confirm »
	p.press(tcell.KeyEnter, 0) // refused
	assert.True(t, p.page("sub-overlay"), "the form stays open")
	assert.True(t, p.page("inline-overlay"), "and the card under it")
	awaitText(t, p, i18n.T("tui.checkpoint.message_required"))
	assert.Empty(t, be.answers(), "nothing was sent")

	p.press(tcell.KeyBacktab, 0) // the message
	p.press(tcell.KeyRune, 'x')
	p.press(tcell.KeyTab, 0)
	p.press(tcell.KeyEnter, 0)
	require.Eventually(t, func() bool { return len(be.answers()) == 1 }, 3*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"d-cp1|fix|x"}, be.answers())
	assert.False(t, p.hasOverlayOnLoop(), "answered: the card and the form are gone")
}

// « Fix » and « Override » of a failed verification ask their text over the
// card, refuse an empty one, and give the card back on Esc.
func TestSessionsFailedVerificationButtonsKeepTheCard(t *testing.T) {
	failed := views.CheckpointVerifyView{Status: "fail", Summary: "tests fail"}
	for name, button := range map[string]int{"fix": 0, "override": 1} {
		t.Run(name, func(t *testing.T) {
			be := newDecidingBackend()
			be.verify = failed
			p := sessionsSim(t, "d:d-cp1", be, Config{})
			p.press(tcell.KeyEnter, 0)
			awaitText(t, p, "Plan validation")
			for i := 0; i < button; i++ {
				p.press(tcell.KeyRight, 0)
			}
			label := p.focus()
			p.press(tcell.KeyEnter, 0)
			require.True(t, p.page("sub-overlay"), "the input opens over the card:\n%s", p.text())

			p.press(tcell.KeyEnter, 0) // empty
			assert.True(t, p.page("sub-overlay"), "an empty text is refused, the input stays")
			assert.True(t, p.page("inline-overlay"))
			assert.Empty(t, be.answers())

			p.press(tcell.KeyEscape, 0)
			assert.False(t, p.page("sub-overlay"))
			assert.Contains(t, p.text(), "Plan validation")
			assert.Same(t, label, p.focus())
		})
	}
}

// « Override » asks a reason then a confirmation: cancelling the
// confirmation brings the card back.
func TestSessionsOverrideChainKeepsTheCard(t *testing.T) {
	be := newDecidingBackend()
	be.verify = views.CheckpointVerifyView{Status: "fail", Summary: "tests fail"}
	p := sessionsSim(t, "d:d-cp1", be, Config{})
	p.press(tcell.KeyEnter, 0)
	awaitText(t, p, "Plan validation")
	p.press(tcell.KeyRight, 0)
	p.press(tcell.KeyEnter, 0) // « Override »
	p.press(tcell.KeyRune, 'w')
	p.press(tcell.KeyEnter, 0) // the reason
	require.True(t, p.page("sub-overlay"), "the confirmation opens over the card:\n%s", p.text())
	require.True(t, p.page("inline-overlay"), "the card is still there")
	assert.Contains(t, p.text(), i18n.Tf("tui.checkpoint.override_confirm", "plan"))

	p.press(tcell.KeyEscape, 0) // cancel
	assert.False(t, p.page("sub-overlay"))
	assert.Contains(t, p.text(), "Plan validation", "the card is back")
	assert.Empty(t, be.answers())
}

// / in the Trace tab: the search input gives the reader its focus back (the
// arrows scroll the trace, they do not change the session), and Tab does not
// lose what was typed.
func TestSessionsSearchInTheTraceKeepsTheReaderFocus(t *testing.T) {
	p := sessionsSim(t, "ses_inbox", newDecidingBackend(), Config{})
	p.press(tcell.KeyTab, 0) // the Trace tab
	awaitText(t, p, "step 0")
	reader := p.focus()

	p.press(tcell.KeyRune, '/')
	require.True(t, p.page("inline-overlay"), "the search input:\n%s", p.text())
	p.press(tcell.KeyRune, 'a')
	p.press(tcell.KeyRune, 'b')
	p.press(tcell.KeyTab, 0)
	assert.True(t, p.page("inline-overlay"), "Tab does not close the input")
	assert.Contains(t, p.text(), "ab", "and keeps what was typed")

	p.press(tcell.KeyEscape, 0)
	assert.False(t, p.hasOverlayOnLoop())
	assert.Same(t, reader, p.focus(), "the reader has the focus back")
}

// Ctrl+Q while a reader is open over the decision card: the dialog opens over
// them, Cancel brings the reader back, and closing everything leaves
// nothing on the screen.
func TestSessionsQuitDialogOverReaderAndCard(t *testing.T) {
	var sh *Shell
	cancelled := 0
	cfg := Config{BeforeQuit: func(func()) {
		sh.ShowInlineForm(views.InlineFormConfig{Title: "quit dialog",
			Fields:   []views.FormField{{Key: "k", Label: "choice", Type: views.FieldBool, Default: "true"}},
			OnCancel: func() { cancelled++; sh.CancelQuit() }})
	}}
	p := sessionsSim(t, "d:d-cp1", newDecidingBackend(), cfg)
	sh = p.Shell
	p.press(tcell.KeyEnter, 0)
	awaitText(t, p, "Plan validation")
	p.press(tcell.KeyRight, 0) // « Read rp-1 »
	p.press(tcell.KeyEnter, 0)
	awaitText(t, p, "line of the report")
	reader := p.focus()

	p.press(tcell.KeyCtrlQ, 0)
	awaitText(t, p, "quit dialog")
	require.True(t, p.page("sub-overlay-2"))

	p.press(tcell.KeyEscape, 0) // Cancel
	assert.Equal(t, 1, cancelled)
	assert.Contains(t, p.text(), "line of the report", "the reader is back")
	assert.Same(t, reader, p.focus())

	p.press(tcell.KeyEscape, 0)
	p.press(tcell.KeyEscape, 0)
	assert.False(t, p.hasOverlayOnLoop(), "nothing is left without the focus")
	assert.True(t, p.loopContentFocused())
}
