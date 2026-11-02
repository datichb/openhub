package views

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
)

func TestRunInlineWizardStandalone(t *testing.T) {
	tests := []struct {
		name       string
		withHook   bool
		wantResult StandaloneResult
	}{
		{"all steps skipped: the OnComplete hook is forwarded", true, StandaloneResult{Completed: true}},
		{"no OnComplete hook", false, StandaloneResult{Completed: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var completed []bool
			cfg := InlineWizardConfig{
				ID:    "test.standalone",
				Title: "Standalone",
				Steps: []WizardStep{{Label: "A", Skip: true}, {Label: "B", SkipIf: func() bool { return true }}},
			}
			if tc.withHook {
				cfg.OnComplete = func(ok bool, err error) {
					assert.NoError(t, err, "OnComplete error")
					completed = append(completed, ok)
				}
			}

			var result StandaloneResult
			require.NotPanics(t, func() { result = RunInlineWizardStandalone(NewInlineWizardView(cfg)) }, "run")
			assert.Equal(t, tc.wantResult, result, "result")
			if tc.withHook {
				assert.Equal(t, []bool{true}, completed, "the original OnComplete fires once, completed")
			}
		})
	}
}

func TestStandaloneShellStub(t *testing.T) {
	t.Parallel()
	stub := &standaloneShellStub{app: tview.NewApplication()}

	t.Run("the methods the wizard uses are no-ops", func(t *testing.T) {
		require.NotPanics(t, func() {
			stub.SetOmnibarVisible(false)
			stub.SetOmnibarVisible(true)
			stub.PushView(nil)
			_ = stub.Context()
		})
	})

	t.Run("a notification without a wizard is ignored", func(t *testing.T) {
		require.NotPanics(t, func() { stub.ShowToastMsg("saved", false) })
	})

	t.Run("the other methods panic", func(t *testing.T) {
		calls := map[string]func(){
			"ShowInputModal":       func() { stub.ShowInputModal("", "", nil) },
			"ShowPasswordModal":    func() { stub.ShowPasswordModal("", nil) },
			"ShowSelectModal":      func() { stub.ShowSelectModal("", nil, "", nil) },
			"ShowMultiSelectModal": func() { stub.ShowMultiSelectModal("", nil, nil, nil) },
			"ShowScrollableModal":  func() { stub.ShowScrollableModal("", "", nil) },
			"ShowInlineForm":       func() { stub.ShowInlineForm(InlineFormConfig{}) },
			"SetProjectMode":       func() { stub.SetProjectMode(nil) },
			"SetActiveProject":     func() { stub.SetActiveProject(nil) },
			"ActiveProject":        func() { _ = stub.ActiveProject() },
			"SetMode":              func() { stub.SetMode("") },
			"Mode":                 func() { _ = stub.Mode() },
			"SetActiveTeam":        func() { stub.SetActiveTeam(nil) },
			"ActiveTeam":           func() { _ = stub.ActiveTeam() },
		}
		for name, call := range calls {
			assert.Panics(t, call, name)
		}
	})
}

// simText is the text drawn on a simulation screen.
func simText(sc tcell.SimulationScreen) string {
	cells, w, _ := sc.GetContents()
	var b strings.Builder
	for i, c := range cells {
		if len(c.Runes) > 0 {
			b.WriteRune(c.Runes[0])
		} else {
			b.WriteByte(' ')
		}
		if (i+1)%w == 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// standaloneRun runs wizard alone on a simulation screen and calls drive with
// the screen from the test goroutine; it returns the result of the wizard.
func standaloneRun(t *testing.T, wizard *InlineWizardView, drive func(waitFor func(string), key func(tcell.Key), typ func(string))) StandaloneResult {
	t.Helper()
	sc := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, sc.Init())
	sc.SetSize(100, 30)
	// The application starts the screen again when it takes it (tview
	// SetScreen): the test touches the screen once the first form is built.
	ready := make(chan struct{})
	var once sync.Once
	var tvApp *tview.Application
	for i := range wizard.cfg.Steps {
		if form := wizard.cfg.Steps[i].Form; form != nil {
			wizard.cfg.Steps[i].Form = func(app *tview.Application, done func()) *tview.Form {
				once.Do(func() { tvApp = app; close(ready) })
				return form(app, done)
			}
			break
		}
	}
	done := make(chan StandaloneResult, 1)
	go func() { done <- RunInlineWizardStandaloneOn(wizard, sc) }()
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("the wizard did not start")
	}
	waitFor := func(text string) {
		t.Helper()
		// The screen is read on the event loop, between two draws.
		require.Eventually(t, func() bool {
			read := make(chan string, 1)
			tvApp.QueueUpdate(func() { read <- simText(sc) })
			select {
			case txt := <-read:
				return strings.Contains(txt, text)
			case <-time.After(100 * time.Millisecond):
				return false
			}
		}, 3*time.Second, 5*time.Millisecond, "the screen shows %q", text)
	}
	key := func(k tcell.Key) { sc.InjectKey(k, 0, tcell.ModNone) }
	typ := func(s string) {
		for _, r := range s {
			sc.InjectKey(tcell.KeyRune, r, tcell.ModNone)
		}
	}
	drive(waitFor, key, typ)
	select {
	case res := <-done:
		return res
	case <-time.After(3 * time.Second):
		t.Fatal("the wizard did not end")
		return StandaloneResult{}
	}
}

// nameStep is a step with an input and a button, whose OnDone is onDone.
func nameStep(label string, got *string, onDone func() error) WizardStep {
	return WizardStep{
		Label: label, Required: true,
		Form: func(_ *tview.Application, done func()) *tview.Form {
			f := tview.NewForm()
			f.AddInputField("Name", "", 0, nil, func(s string) { *got = s })
			f.AddButton("Next", done)
			return f
		},
		OnDone: onDone,
	}
}

// The engine of the TUI, alone on the screen (the wizards of the CLI): the
// answers are typed, Ctrl+S submits, and the wizard ends with its result.
func TestRunInlineWizardStandaloneKeys(t *testing.T) {
	quit, cont := i18n.T("tui.wizard.quit.quit"), i18n.T("tui.wizard.quit.continue")
	var completions []bool

	t.Run("answers typed, Ctrl+S: completed", func(t *testing.T) {
		var name string
		var stored []string
		completions = nil
		wizard := NewInlineWizardView(InlineWizardConfig{ID: "w", Title: "w",
			OnComplete: func(ok bool, err error) { completions = append(completions, ok && err == nil) },
			Steps: []WizardStep{
				nameStep("Identity", &name, func() error { stored = append(stored, name); return nil }),
				{Label: "Save", OnDone: func() error { stored = append(stored, "saved"); return nil }},
			}})
		res := standaloneRun(t, wizard, func(waitFor func(string), key func(tcell.Key), typ func(string)) {
			waitFor("Name")
			typ("bob")
			waitFor("bob")
			key(tcell.KeyCtrlS)
		})
		assert.Equal(t, StandaloneResult{Completed: true}, res)
		assert.Equal(t, []string{"bob", "saved"}, stored, "the steps ran in order")
		assert.Equal(t, []bool{true}, completions, "OnComplete once")
	})

	t.Run("Esc asks, Quit leaves: aborted, nothing run", func(t *testing.T) {
		var name string
		ran := false
		wizard := NewInlineWizardView(InlineWizardConfig{ID: "w", Title: "w",
			Steps: []WizardStep{nameStep("Identity", &name, func() error { ran = true; return nil })}})
		res := standaloneRun(t, wizard, func(waitFor func(string), key func(tcell.Key), typ func(string)) {
			waitFor("Name")
			key(tcell.KeyEscape)
			waitFor(i18n.T("tui.wizard.quit.title"))
			waitFor(quit)
			key(tcell.KeyLeft)
			key(tcell.KeyEnter)
		})
		assert.Equal(t, StandaloneResult{Aborted: true}, res)
		assert.False(t, ran, "OnDone never ran")
	})

	t.Run("Esc then Continue: the wizard goes on", func(t *testing.T) {
		var name string
		wizard := NewInlineWizardView(InlineWizardConfig{ID: "w", Title: "w",
			Steps: []WizardStep{nameStep("Identity", &name, nil)}})
		res := standaloneRun(t, wizard, func(waitFor func(string), key func(tcell.Key), typ func(string)) {
			waitFor("Name")
			key(tcell.KeyEscape)
			waitFor(cont)
			key(tcell.KeyEnter) // the focus starts on Continue
			typ("al")
			waitFor("al")
			key(tcell.KeyCtrlS)
		})
		assert.Equal(t, StandaloneResult{Completed: true}, res)
		assert.Equal(t, "al", name)
	})

	t.Run("a step fails: the error shows, Enter retries", func(t *testing.T) {
		var name string
		calls := 0
		wizard := NewInlineWizardView(InlineWizardConfig{ID: "w", Title: "w",
			Steps: []WizardStep{nameStep("Identity", &name, func() error {
				calls++
				if calls == 1 {
					return errors.New("boom")
				}
				return nil
			})}})
		res := standaloneRun(t, wizard, func(waitFor func(string), key func(tcell.Key), typ func(string)) {
			waitFor("Name")
			key(tcell.KeyCtrlS)
			waitFor("boom")
			key(tcell.KeyEnter)
			waitFor("Name")
			key(tcell.KeyCtrlS)
		})
		assert.Equal(t, StandaloneResult{Completed: true}, res, "the retry went through, the error is gone")
		assert.Equal(t, 2, calls)
	})

	t.Run("a step fails and the user quits: the error is the result", func(t *testing.T) {
		var name string
		wizard := NewInlineWizardView(InlineWizardConfig{ID: "w", Title: "w",
			Steps: []WizardStep{nameStep("Identity", &name, func() error { return errors.New("boom") })}})
		res := standaloneRun(t, wizard, func(waitFor func(string), key func(tcell.Key), typ func(string)) {
			waitFor("Name")
			key(tcell.KeyCtrlS)
			waitFor("boom")
			key(tcell.KeyEscape)
			waitFor(quit)
			key(tcell.KeyLeft)
			key(tcell.KeyEnter)
		})
		assert.False(t, res.Completed)
		assert.False(t, res.Aborted, "a failure is not a quiet exit")
		assert.EqualError(t, res.Err, "boom")
	})

	t.Run("a step fails, the user goes back and completes: no stale error", func(t *testing.T) {
		var first, second string
		calls := 0
		wizard := NewInlineWizardView(InlineWizardConfig{ID: "w", Title: "w",
			Steps: []WizardStep{
				nameStep("First", &first, nil),
				nameStep("Second", &second, func() error {
					calls++
					if calls == 1 {
						return errors.New("boom")
					}
					return nil
				}),
			}})
		res := standaloneRun(t, wizard, func(waitFor func(string), key func(tcell.Key), typ func(string)) {
			waitFor("1/2 — First")
			key(tcell.KeyCtrlS)
			waitFor("2/2 — Second")
			key(tcell.KeyCtrlS)
			waitFor("boom")
			key(tcell.KeyCtrlB)
			waitFor("1/2 — First")
			key(tcell.KeyCtrlS)
			waitFor("2/2 — Second")
			key(tcell.KeyCtrlS)
		})
		assert.Equal(t, StandaloneResult{Completed: true}, res)
	})

	t.Run("Ctrl+C: aborted", func(t *testing.T) {
		var name string
		wizard := NewInlineWizardView(InlineWizardConfig{ID: "w", Title: "w",
			Steps: []WizardStep{nameStep("Identity", &name, nil)}})
		res := standaloneRun(t, wizard, func(waitFor func(string), key func(tcell.Key), typ func(string)) {
			waitFor("Name")
			key(tcell.KeyCtrlC)
		})
		assert.True(t, res.Aborted)
		assert.False(t, res.Completed)
	})
}
