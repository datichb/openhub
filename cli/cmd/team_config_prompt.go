package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// promptAttempts is how many times a question is asked again after an answer
// that is not one, before the assistant gives up.
const promptAttempts = 3

// prompter asks the questions of `oh team config` on the streams of the app
// (a.IO.In and a.IO.Out; the standard input only when that is what the app
// has). An answer is never guessed: Enter takes the value shown in brackets
// (the current one), an answer that is not one is said and asked again, and an
// input that ends (Ctrl+D, a closed pipe) stops the assistant: err is set,
// the answers that follow are the current values and nothing must be written.
type prompter struct {
	in  *bufio.Reader
	out io.Writer
	err error
}

// newPrompter is the prompter of the streams of a. Without a terminal to
// answer on (the standard input is a pipe, /dev/null, a CI job) the questions
// cannot be answered: it is refused rather than answered with the first choice.
func newPrompter(a *app.App) (*prompter, error) {
	in := a.IO.In
	if in == nil {
		return nil, errors.New(i18n.T("v533b.prompt.no_terminal"))
	}
	if f, ok := in.(*os.File); ok && f == os.Stdin && !stdinIsTerminal() {
		return nil, errors.New(i18n.T("v533b.prompt.no_terminal"))
	}
	return &prompter{in: bufio.NewReader(in), out: a.IO.Out}, nil
}

// failed is why the assistant has to stop (nil: all the answers were given).
func (p *prompter) failed() error { return p.err }

// fail stops the assistant with err (the first failure is kept).
func (p *prompter) fail(err error) {
	if p.err == nil {
		p.err = err
	}
}

// line reads an answer, trimmed. ok is false once the input has ended.
func (p *prompter) line() (string, bool) {
	if p.err != nil {
		return "", false
	}
	s, err := p.in.ReadString('\n')
	if err != nil && s == "" {
		fmt.Fprintln(p.out)
		p.fail(errors.New(i18n.T("v533b.prompt.input_closed")))
		return "", false
	}
	return strings.TrimSpace(s), true
}

// clearable reads the answer "-" as « remove this value ».
func clearable(v string) string {
	if v == "-" {
		return ""
	}
	return v
}

// askInput asks for a text; Enter keeps def. "-" is returned as it is: the
// caller decides whether it clears the value (clearable).
func askInput(p *prompter, prompt, def string) string {
	if def != "" {
		fmt.Fprintf(p.out, "  %s [%s]: ", prompt, def)
	} else {
		fmt.Fprintf(p.out, "  %s: ", prompt)
	}
	s, ok := p.line()
	if !ok || s == "" {
		return def
	}
	return s
}

// parseYesNo reads an answer to a yes/no question.
func parseYesNo(s string) (yes, ok bool) {
	switch strings.ToLower(s) {
	case "o", "oui", "y", "yes":
		return true, true
	case "n", "non", "no":
		return false, true
	}
	return false, false
}

// askYN asks a yes/no question; Enter takes the default.
func askYN(p *prompter, prompt string, defaultYes bool) bool {
	for range promptAttempts {
		fmt.Fprintf(p.out, "  %s %s ", prompt, yesNoHint(defaultYes))
		s, ok := p.line()
		if !ok || s == "" {
			return defaultYes
		}
		if yes, ok := parseYesNo(s); ok {
			return yes
		}
		fmt.Fprint(p.out, i18n.Tf("v533b.prompt.not_yes_no", s))
	}
	p.fail(errors.New(i18n.T("v533b.prompt.gave_up")))
	return defaultYes
}

// askSelect asks for one of options; Enter takes the option def (0-based).
// def < 0: there is no current value, an answer is required.
func askSelect(p *prompter, prompt string, options []string, def int) int {
	fmt.Fprintf(p.out, "\n  %s\n", prompt)
	for i, opt := range options {
		fmt.Fprintf(p.out, "    [%d] %s\n", i+1, opt)
	}
	for range promptAttempts {
		if def >= 0 {
			fmt.Fprint(p.out, i18n.Tf("v533b.prompt.choice_default", def+1))
		} else {
			fmt.Fprint(p.out, i18n.T("v533b.prompt.choice_required"))
		}
		s, ok := p.line()
		if !ok {
			return max(def, 0)
		}
		if s == "" && def >= 0 {
			return def
		}
		if n, ok := parseChoice(s, len(options)); ok {
			return n - 1
		}
		fmt.Fprint(p.out, i18n.Tf("v533b.prompt.out_of_range", s, len(options)))
	}
	p.fail(errors.New(i18n.T("v533b.prompt.gave_up")))
	return max(def, 0)
}

// parseChoice reads a number from 1 to n.
func parseChoice(s string, n int) (int, bool) {
	v := 0
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		v = v*10 + int(c-'0')
		if v > n {
			return 0, false
		}
	}
	return v, v >= 1
}

// askTriState asks for a setting that may be left unset: inherit (nil), yes or
// no. Enter keeps the current state; inherit is worded by the caller (what is
// inherited). The result is the new state.
func askTriState(p *prompter, prompt string, current *bool, inherit string) *bool {
	def := 0
	if current != nil {
		def = 2
		if *current {
			def = 1
		}
	}
	switch askSelect(p, prompt, []string{inherit, i18n.T("v533b.prompt.yes"), i18n.T("v533b.prompt.no")}, def) {
	case 1:
		b := true
		return &b
	case 2:
		b := false
		return &b
	}
	return nil
}

// boolWord is "yes" or "no" in the language of the hub.
func boolWord(b bool) string {
	if b {
		return i18n.T("v533b.prompt.yes")
	}
	return i18n.T("v533b.prompt.no")
}
