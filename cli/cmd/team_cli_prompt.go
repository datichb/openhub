package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// The questions of the commands that ask on the terminal line by line (oh team
// rejoin): they read the input of the application (IOStreams.In), never
// os.Stdin behind its back, and hide a secret only when that input is a
// terminal.

// cliPrompter asks questions on the streams of the application.
type cliPrompter struct {
	in  io.Reader
	out io.Writer
}

func newCLIPrompter(a *app.App) *cliPrompter {
	return &cliPrompter{in: a.IO.In, out: a.IO.Out}
}

// line prints prompt and reads a line (trimmed). The input is read a byte at a
// time: another reader of the same stream loses nothing.
func (p *cliPrompter) line(prompt string) string {
	fmt.Fprint(p.out, prompt)
	return readInputLine(p.in)
}

// secret is line, without echo when the input is a terminal.
func (p *cliPrompter) secret(prompt string) string {
	fmt.Fprint(p.out, prompt)
	if f, ok := p.in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(p.out)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	return readInputLine(p.in)
}

// yes asks a yes/no question; def is the answer of Enter.
func (p *cliPrompter) yes(prompt string, def bool) bool {
	hint := " [y/N] "
	if def {
		hint = " [Y/n] "
	}
	switch strings.ToLower(p.line(prompt + hint)) {
	case "y", "yes", "o", "oui":
		return true
	case "n", "no", "non":
		return false
	}
	return def
}

// readInputLine reads up to the end of the line, one byte at a time.
func readInputLine(in io.Reader) string {
	var sb strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				break
			}
			sb.WriteByte(buf[0])
		}
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(sb.String())
}

// maxTokenAttempts is how many tokens `oh team rejoin` accepts for one
// repository before it gives up.
const maxTokenAttempts = 3

// ensureAccess makes sure the repository answers: it probes it with what git
// and oh have, and, when the access is refused, asks for the way in: the token
// of an HTTPS repository (checked, then kept in the keychain of oh), nothing
// for SSH (the key is the user's business: the error says what to check). It
// is the text version of the probe and access steps of the assistants
// (teamAccess), run before anything is cloned.
func (p *cliPrompter) ensureAccess(ctx context.Context, a *app.App, repo string) error {
	err := teamstate.ProbeRemote(ctx, repo)
	if err == nil {
		return nil
	}
	authErr := teamstate.AccessError(err)
	if authErr == nil {
		return fmt.Errorf(i18n.T("v532w.probe.failed"), repo, gitCause(err))
	}
	if !teamstate.IsHTTPS(repo) {
		return errors.New(i18n.Tf("v532w.access.refused_ssh", repo, gitCause(err)))
	}
	fmt.Fprintf(p.out, "\n  %s %s\n", theme.WarningStyle.Render(theme.IconDot), i18n.Tf("v532w.access.https_intro", repo, gitCause(err)))
	fmt.Fprintf(p.out, "  %s\n", i18n.T("v532w.access.https_hint"))
	for range maxTokenAttempts {
		user := firstNonEmpty(p.line(fmt.Sprintf("  %s [%s]: ", i18n.T("cmd.init.wizard_team_cred_username"), defaultGitUser)), defaultGitUser)
		token := p.secret(fmt.Sprintf("  %s: ", i18n.T("cmd.init.wizard_team_cred_token")))
		if token == "" {
			return errors.New(i18n.T("v532w.access.token_required"))
		}
		err = teamstate.ProbeRemoteAs(ctx, repo, teamstate.GitCredentials{Username: user, Token: token})
		if err == nil {
			if saveErr := saveTeamGitToken(ctx, a, repo, user, token); saveErr != nil {
				return saveErr
			}
			fmt.Fprintf(p.out, "  %s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.T("v532w.access.token_kept"))
			return nil
		}
		if teamstate.AccessError(err) == nil {
			return fmt.Errorf(i18n.T("v532w.probe.failed"), repo, gitCause(err))
		}
		fmt.Fprintf(p.out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("v532w.access.refused_https", repo, gitCause(err)))
	}
	return errors.New(i18n.Tf("v532w.access.refused_https", repo, gitCause(err)))
}
