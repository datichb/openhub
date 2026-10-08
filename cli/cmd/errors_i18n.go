package cmd

import (
	"errors"
	"regexp"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// Errors of cobra and of shared layers, translated before they are printed
// (A12). Cobra's messages are fixed strings: they are matched here.

var cobraErrors = []struct {
	re  *regexp.Regexp
	key string
}{
	{regexp.MustCompile(`^unknown command "(.*)" for "(.*)"`), "cmd.errors.unknown_command"},
	{regexp.MustCompile(`^unknown shorthand flag: '(.)' in (\S+)`), "cmd.errors.unknown_shorthand"},
	{regexp.MustCompile(`^unknown flag: (\S+)`), "cmd.errors.unknown_flag"},
	{regexp.MustCompile(`^flag needs an argument: '(.)' in (\S+)`), "cmd.errors.flag_needs_arg_short"},
	{regexp.MustCompile(`^flag needs an argument: (\S+)`), "cmd.errors.flag_needs_arg"},
	{regexp.MustCompile(`^invalid argument "(.*)" for "(.*)" flag: (.*)`), "cmd.errors.invalid_flag_value"},
	{regexp.MustCompile(`^required flag\(s\) (.*) not set`), "cmd.errors.required_flags"},
	{regexp.MustCompile(`^accepts (\d+) arg\(s\), received (\d+)`), "cmd.errors.args_exact"},
	{regexp.MustCompile(`^accepts at most (\d+) arg\(s\), received (\d+)`), "cmd.errors.args_max"},
	{regexp.MustCompile(`^requires at least (\d+) arg\(s\), only received (\d+)`), "cmd.errors.args_min"},
	{regexp.MustCompile(`^accepts between (\d+) and (\d+) arg\(s\), received (\d+)`), "cmd.errors.args_range"},
}

// ensureLocale sets the hub language when the app is not initialized yet
// (cobra rejects unknown commands and flags before PersistentPreRunE).
func ensureLocale() {
	if application != nil {
		return
	}
	if c, err := config.Load(); err == nil && c != nil && c.CLI.Language != "" {
		i18n.SetLocale(c.CLI.Language)
	}
}

// localizeError returns the message printed for an error of a command.
func localizeError(err error) string {
	var api *daemon.APIError
	switch {
	case errors.As(err, &api) && api.Path == "/shutdown" && api.Status == 409:
		return i18n.T("cmd.daemon.stop_busy")
	case isNoTerminalError(err):
		return i18n.T("cmd.errors.no_terminal")
	}
	msg := err.Error()
	for _, ce := range cobraErrors {
		m := ce.re.FindStringSubmatch(msg)
		if m == nil {
			continue
		}
		args := make([]any, 0, len(m)-1)
		for _, a := range m[1:] {
			args = append(args, a)
		}
		out := i18n.Tf(ce.key, args...)
		if i := strings.Index(msg, "\n\nDid you mean this?\n"); i >= 0 {
			var sugg []string
			for _, l := range strings.Split(msg[i+len("\n\nDid you mean this?\n"):], "\n") {
				if l = strings.TrimSpace(l); l != "" {
					sugg = append(sugg, l)
				}
			}
			if len(sugg) > 0 {
				out += "\n\n" + i18n.Tf("cmd.errors.did_you_mean", strings.Join(sugg, ", "))
			}
		}
		return out
	}
	return msg
}

// isNoTerminalError reports an interactive prompt started without a
// terminal (the form library opens /dev/tty).
func isNoTerminalError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "/dev/tty") || strings.Contains(msg, "inappropriate ioctl for device")
}

// sessionRefError translates the errors of a session reference (A12).
func sessionRefError(ref string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrNotFound):
		return errors.New(i18n.Tf("cmd.session.not_found", ref))
	case errors.Is(err, sessionsvc.ErrAmbiguous):
		return errors.New(i18n.Tf("cmd.session.ambiguous", ref))
	}
	return err
}
