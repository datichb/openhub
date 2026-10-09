package views

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// N19, with real key events: Esc asks, ← moves to « Quitter », Enter quits
// and pops the wizard.
func TestInlineWizardView_QuitPromptKeys(t *testing.T) {
	v := NewInlineWizardView(InlineWizardConfig{ID: "w", Steps: []WizardStep{{Label: "a", Required: true, Form: func(_ *tview.Application, onDone func()) *tview.Form {
		f := tview.NewForm()
		f.AddInputField("Name", "", 0, nil, nil)
		f.AddButton("Next", onDone)
		return f
	}}}})
	sh := &popShell{}
	v.SetShell(sh)
	sc := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sc.Init())
	sc.SetSize(80, 24)
	app := tview.NewApplication().SetScreen(sc)
	content := tview.NewFlex()
	app.SetRoot(content, true)
	app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
		if v.HandleKey(e) == nil {
			return nil
		}
		return e
	})
	done := make(chan struct{})
	go func() { _ = app.Run(); close(done) }()
	t.Cleanup(func() { app.Stop(); <-done })
	onLoop(app, func() { v.Mount(content, app) })
	focused := func() string {
		label := ""
		onLoop(app, func() {
			if b, ok := app.GetFocus().(*tview.Button); ok {
				label = b.GetLabel()
			}
		})
		return label
	}
	key := func(k tcell.Key) {
		sc.InjectKey(k, 0, tcell.ModNone)
		onLoop(app, func() {})
	}

	key(tcell.KeyEscape)
	require.Eventually(t, func() bool { return focused() == i18n.T("tui.wizard.quit.continue") }, time.Second, 5*time.Millisecond)
	focusIs := func(label string) {
		t.Helper()
		assert.Eventually(t, func() bool { return focused() == label }, time.Second, 5*time.Millisecond, label)
	}
	key(tcell.KeyLeft)
	focusIs(i18n.T("tui.wizard.quit.quit"))
	key(tcell.KeyLeft)
	focusIs(i18n.T("tui.wizard.quit.continue")) // wraps around
	key(tcell.KeyRight)
	focusIs(i18n.T("tui.wizard.quit.quit"))
	key(tcell.KeyEnter)
	require.Eventually(t, func() bool {
		n := 0
		onLoop(app, func() { n = sh.popped })
		return n == 1
	}, time.Second, 5*time.Millisecond)
}
