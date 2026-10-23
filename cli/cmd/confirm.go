package cmd

import (
	"errors"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// confirmAction is the single confirmation of the commands that act on the
// user's data (D4, v5.0). With --yes the answer is yes without a question.
// Without a terminal nobody can answer: the action is refused with an error
// that ends oh with ExitUsage, unless --yes is given. With a terminal, a
// form asks the question (buttons in the language of the hub); defaultYes is
// the button that is highlighted. Ctrl+C is a no.
//
// A command that already has a --force which skips the question keeps it
// (v5.0) and tests it before calling confirmAction.
func confirmAction(cmd *cobra.Command, question string, defaultYes bool) (bool, error) {
	return confirmActionWith(cmd, question, "", defaultYes)
}

// confirmFunc asks a yes/no question (default no) for the code that does not
// know whether it runs under a command (the repairs of oh doctor, also run
// from the TUI, where the view has confirmed them).
type confirmFunc func(question string) (bool, error)

// cmdConfirm asks through confirmAction: --yes, or a terminal.
func cmdConfirm(cmd *cobra.Command) confirmFunc {
	return func(question string) (bool, error) { return confirmAction(cmd, question, false) }
}

// alwaysConfirm is the answer of a repair the view has already confirmed.
func alwaysConfirm(string) (bool, error) { return true, nil }

// confirmActionWith is confirmAction with a description under the question.
func confirmActionWith(cmd *cobra.Command, question, description string, defaultYes bool) (bool, error) {
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return true, nil
	}
	if !stdinIsTerminal() {
		return false, withExitCode(ExitUsage, errors.New(i18n.T("cmd.confirm.needs_yes")))
	}
	return confirmPrompt(question, description, defaultYes)
}

// confirmForm shows the form of a confirmation (a variable for the tests).
var confirmForm = func(question, description string, ok *bool) error {
	return theme.NewForm(huh.NewGroup(huh.NewConfirm().
		Title(question).
		Description(description).
		Affirmative(i18n.T("cmd.confirm.yes")).
		Negative(i18n.T("cmd.confirm.no")).
		Value(ok))).Run()
}

// confirmPrompt asks the question in a form, whatever the flags: for the
// questions that follow a choice made on a terminal, and that --yes must not
// answer (the deletion of a session chosen in a menu). The caller has checked
// that there is a terminal.
func confirmPrompt(question, description string, defaultYes bool) (bool, error) {
	ok := defaultYes
	if err := confirmForm(question, description, &ok); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return false, nil
		}
		return false, err
	}
	return ok, nil
}

// addYesFlag registers --yes (-y) on a command that confirms an action.
func addYesFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP("yes", "y", false, i18n.T("cmd.confirm.flags.yes"))
}

// confirmed reads the answer to a yes/no question typed on the keyboard. It
// is the only reader of such answers in the CLI (U-11): o, oui, y and yes, in
// any case, are yes; an empty answer takes the default of the question; any
// other answer is no. (The TUI confirms with keys, not typed answers.)
func confirmed(answer string, defaultYes bool) bool {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "":
		return defaultYes
	case "o", "oui", "y", "yes":
		return true
	}
	return false
}

// yesNoHint is the hint of a yes/no question in the language of the hub:
// "[O/n]" (yes by default) or "[o/N]", "[Y/n]" or "[y/N]" in English.
func yesNoHint(defaultYes bool) string {
	if defaultYes {
		return i18n.T("cmd.confirm.hint_yes")
	}
	return i18n.T("cmd.confirm.hint_no")
}
