package cmd

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// wizardKeys is what a test sends to a CLI wizard that runs for real, on a
// simulation screen: it waits for a text to be drawn, then sends keys.
type wizardKeys struct {
	t       *testing.T
	sc      tcell.SimulationScreen
	appc    chan *tview.Application
	failed  bool
	tvApp   *tview.Application
	started sync.Once
	// sent: keys went in since the last waitForText, whose screen was prev.
	sent bool
	prev string
}

// onKeys makes runCLIWizard run the wizards on a simulation screen and call
// drive from another goroutine, until the end of the test. drive must not use
// require: it fails the test with Errorf and returns.
func onKeys(t *testing.T, drive func(k *wizardKeys)) {
	t.Helper()
	prev := runCLIWizard
	t.Cleanup(func() { runCLIWizard = prev })
	runCLIWizard = func(a *app.App, w cliWizard) cliWizardResult {
		k := &wizardKeys{t: t, sc: tcell.NewSimulationScreen("UTF-8"), appc: make(chan *tview.Application, 1)}
		require.NoError(t, k.sc.Init())
		k.sc.SetSize(120, 30)
		// The application starts the screen again when it takes it: the
		// screen is touched once the first form is built.
		for i := range w.Steps {
			if form := w.Steps[i].Form; form != nil {
				w.Steps[i].Form = func(app *tview.Application, done func()) *tview.Form {
					k.started.Do(func() { k.appc <- app })
					return form(app, done)
				}
				break
			}
		}
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			select {
			case k.tvApp = <-k.appc:
			case <-time.After(3 * time.Second):
				t.Errorf("the wizard did not start")
				return
			}
			defer func() {
				if k.failed {
					k.tvApp.Stop() // the wizard would wait for ever
				}
			}()
			drive(k)
		}()
		res := runCLIWizardOn(a, w, k.sc)
		<-finished
		return res
	}
}

// waitFor waits for text to be drawn; the screen is read on the event loop,
// between two draws.
func (k *wizardKeys) waitFor(text string) {
	k.t.Helper()
	if k.failed {
		return
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		read := make(chan string, 1)
		k.tvApp.QueueUpdate(func() { read <- screenContents(k.sc) })
		select {
		case txt := <-read:
			if strings.Contains(txt, text) {
				return
			}
		case <-time.After(100 * time.Millisecond):
		}
		time.Sleep(5 * time.Millisecond)
	}
	k.failed = true
	k.t.Errorf("the screen does not show %q", text)
}

func (k *wizardKeys) key(keys ...tcell.Key) {
	k.sent = true
	for _, key := range keys {
		k.sc.InjectKey(key, 0, tcell.ModNone)
	}
}

func (k *wizardKeys) typ(s string) {
	k.sent = true
	for _, r := range s {
		k.sc.InjectKey(tcell.KeyRune, r, tcell.ModNone)
	}
}

func screenContents(sc tcell.SimulationScreen) string {
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

// oh policies add, on the engine of the TUI with real keys: the answers are
// typed, Ctrl+S submits, and the policy is written.
func TestPoliciesAddOnTheEngine(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, _ := setupClaimTestApp(t)
	onKeys(t, func(k *wizardKeys) {
		k.waitFor("Nom de la policy")
		k.typ("branch")
		k.waitFor("branch")
		k.key(tcell.KeyCtrlS)
		k.waitFor("Pattern regex")
		k.typ("^feat/")
		k.waitFor("^feat/")
		k.key(tcell.KeyCtrlS)
	})

	policiesAddCmd.SetContext(t.Context())
	require.NoError(t, runPoliciesAdd(policiesAddCmd, nil))

	list, err := repo.LoadPolicies(t.Context(), "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, teamstate.Policy{
		Name: "branch", Type: teamstate.PolicyTypeRegex, Rule: "^feat/", Enforcement: teamstate.EnforcementWarn,
	}, list[0])
}

// Ctrl+S submits a step like its button does: the MCP step of oh project add
// takes the services from its button, so they are kept (before, Ctrl+S went
// past the button and the services were lost).
func TestProjectAddMCPOnTheEngine(t *testing.T) {
	i18ntest.Set(t, "en")
	configtest.Home(t)
	fakeBd(t, false)
	a := newMockApp(nil, nil)
	io, _, _ := testIO()
	a.IO = io
	store := &mockProjectStore{}
	a.Projects = store
	onKeys(t, func(k *wizardKeys) {
		k.waitFor(i18n.T("cmd.init.project_name"))
		k.typ("mcpapp")
		k.waitFor("mcpapp")
		k.key(tcell.KeyCtrlS) // no path: the working directory
		k.waitFor(i18n.Tf("form.project.provider_custom", "bedrock"))
		k.key(tcell.KeyCtrlS)
		k.waitFor("Figma")
		k.typ(" ")
		k.key(tcell.KeyDown, tcell.KeyDown)
		k.typ(" ")
		k.key(tcell.KeyCtrlS)
		k.waitFor(i18n.T("form.project.team_no_team")) // the drop-down: the title says « Team » already
		k.key(tcell.KeyCtrlS)
	})

	require.NoError(t, runProjectAddInteractive(t.Context(), a))

	require.Len(t, store.projects, 1)
	assert.Equal(t, []string{"figma", "gslides"}, store.projects[0].MCP)
}

// Esc asks whether to quit (no more silent double Esc); « Quit » ends the
// command with no error and nothing written.
func TestPoliciesAddQuitOnTheEngine(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, _ := setupClaimTestApp(t)
	onKeys(t, func(k *wizardKeys) {
		k.waitFor("Nom de la policy")
		k.typ("never")
		k.waitFor("never")
		k.key(tcell.KeyEscape)
		k.waitFor(i18n.T("tui.wizard.quit.title"))
		k.waitFor(i18n.T("tui.wizard.quit.quit"))
		k.key(tcell.KeyLeft, tcell.KeyLeft, tcell.KeyEnter) // Continue, Skip, Quit
	})

	policiesAddCmd.SetContext(t.Context())
	require.NoError(t, runPoliciesAdd(policiesAddCmd, nil))

	list, err := repo.LoadPolicies(t.Context(), "")
	require.NoError(t, err)
	assert.Empty(t, list)
}

// A step that fails shows its error and waits: quitting then gives the error
// back to the command (before, the wizard ended on the error at once).
func TestPatternsAddErrorOnTheEngine(t *testing.T) {
	i18ntest.Set(t, "en")
	_, repo, _ := setupClaimTestApp(t)
	onKeys(t, func(k *wizardKeys) {
		k.waitFor("Nom du pattern")
		k.typ("two words")
		k.waitFor("two words")
		k.key(tcell.KeyCtrlS)
		k.waitFor(i18n.T("cmd.patterns_cmd.use_hyphens_not_spaces"))
		k.key(tcell.KeyEscape)
		k.waitFor(i18n.T("tui.wizard.quit.quit"))
		k.key(tcell.KeyLeft, tcell.KeyLeft, tcell.KeyEnter) // Continue, Skip, Quit
	})

	patternsAddCmd.SetContext(t.Context())
	err := runPatternsAdd(patternsAddCmd, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), i18n.T("cmd.patterns_cmd.use_hyphens_not_spaces"))
	list, lerr := repo.ListPatterns(t.Context(), nil, 0)
	require.NoError(t, lerr)
	assert.Empty(t, list)
}

// What a step prints while the wizard owns the screen is held, then written
// to the streams of the app, standard output first.
func TestHoldOutput(t *testing.T) {
	var out, errOut bytes.Buffer
	streams := &app.IOStreams{Out: &out, ErrOut: &errOut}

	held := holdOutput(streams)
	_, _ = streams.Out.Write([]byte("one\n"))
	_, _ = streams.ErrOut.Write([]byte("warning\n"))
	_, _ = streams.Out.Write([]byte("two\n"))
	assert.Empty(t, out.String(), "nothing reaches the output while the wizard runs")
	assert.Empty(t, errOut.String())

	held.release()
	assert.Equal(t, "one\ntwo\n", out.String())
	assert.Equal(t, "warning\n", errOut.String())
	assert.Same(t, &out, streams.Out, "the streams are given back")

	require.NotPanics(t, func() { holdOutput(nil).release() }, "no streams: nothing to hold")
}

// The view of a CLI wizard is named after the command and keeps the steps
// (not a copy: a step finds its own Rerender in the slice it built).
func TestCLIWizardConfig(t *testing.T) {
	steps := []views.WizardStep{{Label: "a"}}
	cfg := cliWizardConfig(cliWizard{Command: "project add", Steps: steps})
	assert.Equal(t, "wizard.cli.project.add", cfg.ID)
	assert.Equal(t, "project add", cfg.Title)
	cfg.Steps[0].Label = "b"
	assert.Equal(t, "b", steps[0].Label, "the steps are shared")
}
