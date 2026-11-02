package views

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
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
	runLoop(t, app)
	onLoop(app, func() { v.Mount(content, app) })
	focused := func() string {
		label := ""
		onLoop(app, func() {
			if b, ok := app.GetFocus().(*tview.Button); ok {
				label = widgets.ButtonLabel(b)
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

// A notification shows in the hints bar without blocking the wizard, then the
// hints of the step come back; the stub of the shell of a standalone wizard
// routes ShowToastMsg to it.
func TestInlineWizardView_Notify(t *testing.T) {
	old := notifyHold
	notifyHold = 80 * time.Millisecond
	t.Cleanup(func() { notifyHold = old })

	v := NewInlineWizardView(InlineWizardConfig{ID: "w", Steps: []WizardStep{{Label: "a", Form: func(_ *tview.Application, onDone func()) *tview.Form {
		f := tview.NewForm()
		f.AddInputField("Name", "", 0, nil, nil)
		f.AddButton("Next", onDone)
		return f
	}}}})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	hints := func() (text string) {
		onLoop(app, func() { text = v.hintsBar.GetText(true) })
		return text
	}
	base := hints()
	require.NotContains(t, base, "saved")

	v.Notify("saved", false)
	require.Eventually(t, func() bool { return strings.Contains(hints(), "saved") }, time.Second, 5*time.Millisecond, "the notification shows")
	require.Eventually(t, func() bool { return hints() == base }, time.Second, 5*time.Millisecond, "the hints of the step come back")

	(&standaloneShellStub{app: app, wizard: v}).ShowToastMsg("failed", true)
	require.Eventually(t, func() bool { return strings.Contains(hints(), "failed") }, time.Second, 5*time.Millisecond, "the toast of the stub shows")
	require.Eventually(t, func() bool { return hints() == base }, time.Second, 5*time.Millisecond)

	onLoop(app, func() { v.Unmount() })
	require.NotPanics(t, func() { v.Notify("late", false) }, "a notification after the end is ignored")
}

// Ctrl+S, and the button of the engine in the grouped layout, submit a step
// like its own button does: the handler of that button runs (it may set the
// state of the step) before the wizard goes on.
func TestInlineWizardView_SubmitRunsTheStepButton(t *testing.T) {
	for _, tc := range []struct {
		name    string
		grouped bool
		keys    []tcell.Key
	}{
		{"ctrl+s", false, []tcell.Key{tcell.KeyCtrlS}},
		{"ctrl+s, grouped layout", true, []tcell.Key{tcell.KeyCtrlS}},
		{"button of the engine, grouped layout", true, []tcell.Key{tcell.KeyTab, tcell.KeyEnter}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var chosen string
			second := make(chan struct{})
			steps := []WizardStep{
				{Label: "one", Form: func(_ *tview.Application, onDone func()) *tview.Form {
					f := tview.NewForm()
					f.AddInputField("Name", "", 0, nil, nil)
					f.AddButton("Next", func() { chosen = "from the button"; onDone() })
					return f
				}},
				{Label: "two", Form: func(_ *tview.Application, onDone func()) *tview.Form {
					close(second)
					f := tview.NewForm()
					f.AddButton("Next", onDone)
					return f
				}},
			}
			cfg := InlineWizardConfig{ID: "w", Steps: steps}
			if tc.grouped {
				cfg.Groups = []StepGroup{{Label: "G", StartIdx: 0}}
			}
			v := NewInlineWizardView(cfg)
			v.SetShell(&popShell{})
			sc := tcell.NewSimulationScreen("UTF-8")
			require.NoError(t, sc.Init())
			sc.SetSize(120, 40)
			app := tview.NewApplication().SetScreen(sc)
			content := tview.NewFlex()
			app.SetRoot(content, true)
			app.SetInputCapture(func(e *tcell.EventKey) *tcell.EventKey {
				if v.HandleKey(e) == nil {
					return nil
				}
				return e
			})
			runLoop(t, app)
			onLoop(app, func() { v.Mount(content, app) })

			for _, k := range tc.keys {
				sc.InjectKey(k, 0, tcell.ModNone)
				onLoop(app, func() {})
			}

			select {
			case <-second:
			case <-time.After(2 * time.Second):
				t.Fatal("the wizard did not go on to the second step")
			}
			onLoop(app, func() {})
			assert.Equal(t, "from the button", chosen, "the handler of the button ran")
		})
	}
}
