package cmd

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
)

// A scripted wizard answers the steps of a CLI wizard the way a user does,
// without a terminal: for each step that is not skipped it builds the form,
// lets the test fill it, presses its first button, then runs the validation,
// OnDone and InfoFields of the step, in the order of the engines. It pins what
// a wizard does for given answers (files, keychain, stores), whatever the
// engine that runs the steps.
//
// The answers are keyed by the position of the step in the wizard, so that
// they do not depend on the language; a step that is skipped has no answer.
type scriptedWizard struct {
	t       *testing.T
	answers map[int]func(f *tview.Form)
	abortAt int // position of the step where the user quits; -1 = never

	command string              // command of the wizard
	ran     []int               // positions of the steps that ran
	infos   map[int][]string    // "label: value" of the InfoFields, by step
	refused map[int]string      // validation message that blocked a step
	forms   map[int]*tview.Form // form of each step, as the user left it
	owner   *app.App            // app the wizard was started with
}

// scriptWizard makes runCLIWizard answer with answers until the test ends.
func scriptWizard(t *testing.T, answers map[int]func(f *tview.Form)) *scriptedWizard {
	t.Helper()
	s := &scriptedWizard{
		t: t, answers: answers, abortAt: -1,
		infos: map[int][]string{}, refused: map[int]string{}, forms: map[int]*tview.Form{},
	}
	prev := runCLIWizard
	runCLIWizard = s.run
	t.Cleanup(func() { runCLIWizard = prev })
	return s
}

func (s *scriptedWizard) run(a *app.App, w cliWizard) cliWizardResult {
	s.t.Helper()
	s.command, s.owner = w.Command, a
	tvApp := tview.NewApplication()
	for i := range w.Steps {
		st := &w.Steps[i]
		if st.Skip || (st.SkipIf != nil && st.SkipIf()) {
			continue
		}
		if i == s.abortAt {
			return cliWizardResult{Aborted: true}
		}
		s.ran = append(s.ran, i)
		st.Rerender = func() {}
		if st.Form != nil {
			form := st.Form(tvApp, func() {})
			require.NotNil(s.t, form, "step %d (%s) has no form", i, st.Label)
			if answer := s.answers[i]; answer != nil {
				answer(form)
			}
			s.forms[i] = form
			require.Positive(s.t, form.GetButtonCount(), "step %d (%s) has no button", i, st.Label)
			pressButton(form.GetButton(0))
		} else {
			require.Nil(s.t, st.CustomView, "step %d (%s): custom views are not scripted", i, st.Label)
		}
		if st.Validate != nil {
			if msg := st.Validate(); msg != "" {
				s.refused[i] = msg
				return cliWizardResult{Aborted: true}
			}
		}
		if st.OnDone != nil {
			if err := st.OnDone(); err != nil {
				return cliWizardResult{Err: err}
			}
		}
		if st.InfoFields != nil {
			for _, f := range st.InfoFields() {
				s.infos[i] = append(s.infos[i], f.Label+": "+f.Value)
			}
		}
	}
	return cliWizardResult{Completed: true}
}

// pressButton presses a button like the Enter key does.
func pressButton(b *tview.Button) {
	b.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
}

// typeInto sets the text of the input field at index i of the form.
func typeInto(f *tview.Form, i int, text string) {
	f.GetFormItem(i).(*tview.InputField).SetText(text)
}

// choose selects the option at position opt of the drop-down at index i.
func choose(f *tview.Form, i, opt int) {
	f.GetFormItem(i).(*tview.DropDown).SetCurrentOption(opt)
}

// tick sets the check box at index i.
func tick(f *tview.Form, i int, on bool) {
	f.GetFormItem(i).(*tview.Checkbox).SetChecked(on)
}
