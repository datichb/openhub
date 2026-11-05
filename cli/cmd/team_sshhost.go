package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// The first contact with an SSH server (see teamstate/sshhost.go): before the
// first connection to a server that is not in known_hosts, the commands that
// set a team repository up show its fingerprints and ask for a confirmation;
// without a terminal they refuse, with the commands to run by hand.

// sshHostPromptable tells whether in can answer a question: a terminal (a
// variable for the tests).
var sshHostPromptable = func(in io.Reader) bool {
	f, ok := in.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// ensureSSHHost is the text version of the host steps of the assistants
// (teamAccess.HostSteps): nothing for a remote that is not SSH or whose
// server is known; else the fingerprints and a question, or a refusal when
// the input is not a terminal.
func (p *cliPrompter) ensureSSHHost(ctx context.Context, remote string) error {
	host := teamstate.UnknownSSHHost(ctx, remote)
	if host == nil {
		return nil
	}
	if !sshHostPromptable(p.in) {
		return &teamstate.UnknownHostError{Host: host}
	}
	keys, err := host.Scan(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(p.out, "\n  %s %s\n", theme.WarningStyle.Render(theme.IconWarning), i18n.Tf("v54a2.host.cli_unknown", host.Display()))
	fmt.Fprintf(p.out, "  %s\n", i18n.T("v54a2.host.cli_keys"))
	for _, k := range keys {
		fmt.Fprintf(p.out, "      %s  %s\n", k.Fingerprint, k.Type)
	}
	fmt.Fprintf(p.out, "  %s\n", i18n.T("v54a2.host.cli_check"))
	if !p.yes("  "+i18n.Tf("v54a2.host.cli_ask", host.WriteTo), false) {
		sshCmd, _ := host.ManualCommands()
		return errors.New(i18n.Tf("v54a2.host.refused", host.Display(), sshCmd))
	}
	if err := host.Trust(keys); err != nil {
		return fmt.Errorf(i18n.T("v54a2.host.write_failed"), host.WriteTo, err)
	}
	fmt.Fprintf(p.out, "  %s %s\n", theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("v54a2.host.added", host.Display(), host.WriteTo))
	return nil
}

// hostFingerprintLines are the keys of a server, one per line, for a screen.
func hostFingerprintLines(keys []teamstate.HostKey) string {
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s  %s", k.Fingerprint, k.Type))
	}
	return strings.Join(lines, "\n")
}
